package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kmslib "kms/internal/kms"
	kmsproto "kms/proto"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/xuri/excelize/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v3"
)

// --- Configuration ---
const (
	WorkerCount = 20
	BatchSize   = 500
)

// --- Structs ---
type CardRecord struct {
	ID        int64
	CardNo    string
	CVV       string
	OtherData string
}

type EncryptedRecord struct {
	SourceID     int64
	EncryptedPAN string // Base64 encoded: nonce + ciphertext
	EncryptedCVV string // Base64 encoded: nonce + ciphertext
	OtherData    string
}

type VerificationRecord struct {
	SourceID     int64
	OriginalPAN  string
	OriginalCVV  string
	EncryptedPAN string // The encrypted value from database (base64 string)
	EncryptedCVV string // The encrypted value from database (base64 string)
	DecryptedPAN string
	DecryptedCVV string
	PANMatch     bool
	CVVMatch     bool
	PANError     string
	CVVError     string
	OtherData    string
}

type AppConfig struct {
	KMS struct {
		Addr string `yaml:"addr"`
	} `yaml:"kms"`
	Auth struct {
		BearerToken string `yaml:"bearerToken"`
		Username    string `yaml:"username"`
		Password    string `yaml:"password"`
	} `yaml:"auth"`
	SourceDB struct {
		Driver string `yaml:"driver"`
		DSN    string `yaml:"dsn"`
	} `yaml:"sourceDB"`
	DestDB struct {
		Driver string `yaml:"driver"`
		DSN    string `yaml:"dsn"`
	} `yaml:"destDB"`
}

var (
	processedCount atomic.Uint64
	errorCount     atomic.Uint64
)

// Using helper functions from kms package for combined encryption format

func main() {
	// Flags
	cfgPath := flag.String("config", "config.yaml", "Path to config file")
	verifyMode := flag.Bool("verify", false, "Run in SAFE verification mode (decrypt & mask)")
	verifyExcelMode := flag.Bool("verify-excel", false, "Run ETL + verify all data + export to Excel")
	maskData := flag.Bool("mask-data", false, "Mask sensitive data in Excel output (default: show actual decrypted values)")
	
	// Default Excel output path: C:\Users\user\Desktop\work\KMS-golang-\verification_results_YYYYMMDD_HHMMSS.xlsx
	timestamp := time.Now().Format("20060102_150405")
	defaultExcelPath := filepath.Join("C:\\Users\\user\\Desktop\\work\\KMS-golang-", fmt.Sprintf("verification_results_%s.xlsx", timestamp))
	excelOutput := flag.String("excel-output", defaultExcelPath, "Excel output file path")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 1. Connect DBs
	srcDB, err := sql.Open(cfg.SourceDB.Driver, cfg.SourceDB.DSN)
	if err != nil {
		log.Fatal(err)
	}
	defer srcDB.Close()

	dstDB, err := sql.Open(cfg.DestDB.Driver, cfg.DestDB.DSN)
	if err != nil {
		log.Fatal(err)
	}
	defer dstDB.Close()

	// 2. Connect KMS
	conn, err := grpc.Dial(cfg.KMS.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	kmsClient := kmsproto.NewKMSClient(conn)

	// Auto Login
	kmsToken := os.Getenv("KMS_BEARER_TOKEN")
	if kmsToken != "" {
		log.Printf("Using KMS_BEARER_TOKEN from environment")
	} else if cfg.Auth.BearerToken != "" {
		log.Printf("Using bearer token from config file")
		kmsToken = cfg.Auth.BearerToken
	} else if cfg.Auth.Username != "" {
		log.Printf("Auto-login: Logging in as %s...", cfg.Auth.Username)
		token, err := loginToKMS(conn, cfg.Auth.Username, cfg.Auth.Password)
		if err != nil {
			log.Fatalf("Auto-login failed: %v", err)
		}
		kmsToken = token
		if kmsToken != "" {
			log.Printf("Auto-login successful! Token obtained (length: %d)", len(kmsToken))
		} else {
			log.Fatalf("Auto-login returned empty token!")
		}
	} else {
		log.Printf("WARNING: No authentication configured. JWT auth may be disabled on server.")
		log.Printf("  Set KMS_BEARER_TOKEN env var, or configure auth in config.yaml")
	}

	// --- 判斷模式 ---
	if *verifyExcelMode {
		// ETL + 完整驗證 + Excel 匯出模式
		runETLWithVerification(srcDB, dstDB, kmsClient, kmsToken, cfg.DestDB.Driver, *excelOutput, *maskData)
	} else if *verifyMode {
		// 安全驗證模式：只顯示遮罩後的資料
		runSafeVerification(dstDB, kmsClient, kmsToken, cfg.DestDB.Driver)
	} else {
		// 正常 ETL 模式：高效加密
		runETL(srcDB, dstDB, kmsClient, kmsToken, cfg.DestDB.Driver)
	}
}

// === 安全驗證模式 (Safe Verification Mode) ===
func runSafeVerification(db *sql.DB, client kmsproto.KMSClient, token string, driver string) {
	fmt.Println("\n=== Running PCI-Compliant Verification ===")

	// 隨機取 5 筆 - read from single encrypted_pan field
	query := "SELECT TOP 5 source_id, encrypted_pan FROM encrypted_cards"
	if driver == "mysql" || driver == "postgres" {
		query = "SELECT source_id, encrypted_pan FROM encrypted_cards LIMIT 5"
	}

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to query encrypted data: %v", err)
	}
	defer rows.Close()

	// 輸出標題 (不會輸出完整明文)
	fmt.Println("SourceID | Status | MaskedPAN (Safe to Log)")
	fmt.Println("-------------------------------------------")

	ctx := context.Background()
	for rows.Next() {
		var id int64
		var encryptedPAN string
		if err := rows.Scan(&id, &encryptedPAN); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		// Split the combined field back into nonce and ciphertext
		nonce, ciphertext, err := kmslib.SplitNonceAndCiphertext(encryptedPAN, kmslib.AESGCMNonceSize)
		if err != nil {
			log.Printf("Failed to split encrypted data for ID %d: %v", id, err)
			fmt.Printf("%d      | FAIL: %v | ERROR\n", id, err)
			continue
		}

		// 解密
		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		reqCtx = metadata.AppendToOutgoingContext(reqCtx, "authorization", "Bearer "+token)

		resp, err := client.Decrypt(reqCtx, &kmsproto.DecryptRequest{
			Ciphertext: ciphertext,
			Nonce:      nonce,
		})
		cancel()

		status := "OK"
		maskedPan := "ERROR"

		if err != nil {
			status = fmt.Sprintf("FAIL: %v", err)
		} else {
			// **關鍵步驟：遮罩處理 (Masking)**
			plain := string(resp.Plaintext)
			if len(plain) > 4 {
				// 保留後 4 碼，其餘用 * 取代
				// 例如：************1234
				maskedPan = strings.Repeat("*", len(plain)-4) + plain[len(plain)-4:]
			} else {
				maskedPan = "****"
			}
		}

		fmt.Printf("%d      | %s     | %s\n", id, status, maskedPan)
	}
	fmt.Println("=== Verification Done ===")
}

// === 標準 ETL 模式 (無 Log 檔案) ===
func runETL(srcDB, dstDB *sql.DB, client kmsproto.KMSClient, token, driver string) {
	startTotal := time.Now()
	log.Printf("Starting Production Batch ETL (Batch Size: %d)...", BatchSize)

	// Test KMS connection first
	log.Printf("Testing KMS connection...")
	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if token != "" {
		testCtx = metadata.AppendToOutgoingContext(testCtx, "authorization", "Bearer "+token)
		log.Printf("Using authentication token (length: %d)", len(token))
	} else {
		log.Printf("WARNING: No authentication token - encryption may fail if JWT auth is enabled")
	}

	// Try a test encryption
	testResp, err := client.Encrypt(testCtx, &kmsproto.EncryptRequest{Plaintext: []byte("test")})
	cancel()
	if err != nil {
		log.Fatalf("KMS connection test FAILED: %v", err)
		log.Fatalf("  This means the KMS server is not accessible or authentication failed.")
		log.Fatalf("  Check:")
		log.Fatalf("  1. KMS server is running (check port 50051)")
		log.Fatalf("  2. KMS_JWT_SECRET matches between client and server")
		log.Fatalf("  3. Token is valid (not expired)")
		return
	}
	log.Printf("KMS connection test PASSED (encrypted %d bytes)", len(testResp.Ciphertext))

	jobs := make(chan CardRecord, 100)
	results := make(chan EncryptedRecord, 100)
	var wgWorkers sync.WaitGroup
	var wgWriter sync.WaitGroup

	for w := 1; w <= WorkerCount; w++ {
		wgWorkers.Add(1)
		go worker(w, jobs, results, &wgWorkers, client, token)
	}

	wgWriter.Add(1)
	go batchWriter(dstDB, results, &wgWriter, driver)

	go func() {
		feedRecords(srcDB, jobs)
		close(jobs)
	}()

	wgWorkers.Wait()
	close(results)
	wgWriter.Wait()

	duration := time.Since(startTotal)
	total := processedCount.Load()
	errors := errorCount.Load()

	fmt.Println("\n=== ETL Completed ===")
	fmt.Printf("Total Time:    %v\n", duration)
	fmt.Printf("Total Records: %d\n", total)
	fmt.Printf("Errors:        %d\n", errors)

	if errors > 0 && total == 0 {
		fmt.Println("\n⚠️  WARNING: All encryption attempts failed!")
		fmt.Println("   Check the error messages above to see why.")
		fmt.Println("   Common causes:")
		fmt.Println("   1. KMS server not running or unreachable")
		fmt.Println("   2. Authentication token invalid or expired")
		fmt.Println("   3. KMS_JWT_SECRET mismatch between client and server")
		fmt.Println("   4. Network/firewall issues")
	}
}

// === ETL + Verification + Excel Export Mode ===
func runETLWithVerification(srcDB, dstDB *sql.DB, client kmsproto.KMSClient, token, driver, excelPath string, maskData bool) {
	startTotal := time.Now()

	// Step 1: Run ETL (encrypt all data)
	fmt.Println("\n=== Step 1: Running ETL (Encryption) ===")
	runETL(srcDB, dstDB, client, token, driver)

	// Step 2: Verify all encrypted data
	fmt.Println("\n=== Step 2: Verifying All Encrypted Data ===")

	// Check if any data was encrypted
	var encryptedCount int
	countQuery := "SELECT COUNT(*) FROM encrypted_cards"
	if driver == "mysql" || driver == "postgres" {
		countQuery = "SELECT COUNT(*) FROM encrypted_cards"
	}
	err := dstDB.QueryRow(countQuery).Scan(&encryptedCount)
	if err != nil {
		log.Printf("WARNING: Could not count encrypted records: %v", err)
	} else {
		log.Printf("Found %d encrypted records in destination database", encryptedCount)
		if encryptedCount == 0 {
			log.Printf("ERROR: No encrypted records found! Cannot verify.")
			log.Printf("  This means Step 1 (ETL) failed to encrypt any records.")
			log.Printf("  Check the error messages above to see why encryption failed.")
			return
		}
	}

	// Add overall timeout for verification (30 minutes max)
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer verifyCancel()
	
	verificationResults := verifyAllDataWithContext(verifyCtx, srcDB, dstDB, client, token, driver)

	// Step 3: Export to Excel
	fmt.Println("\n=== Step 3: Exporting Results to Excel ===")
	fmt.Printf("Excel output path: %s\n", excelPath)
	if !maskData {
		log.Printf("WARNING: Sensitive data (PAN/CVV) will be shown in plaintext in Excel file!")
		log.Printf("  Use -mask-data flag to mask sensitive data for production use.")
	}
	if len(verificationResults) == 0 {
		log.Printf("WARNING: No verification results to export!")
	} else {
		if err := exportToExcel(verificationResults, excelPath, maskData); err != nil {
			log.Fatalf("Failed to export to Excel: %v", err)
		}
	}

	duration := time.Since(startTotal)
	fmt.Printf("\n=== Complete Verification Finished ===\n")
	fmt.Printf("Total Time:        %v\n", duration)
	fmt.Printf("Total Records:     %d\n", len(verificationResults))
	fmt.Printf("PAN Matches:       %d\n", countMatches(verificationResults, true, false))
	fmt.Printf("CVV Matches:       %d\n", countMatches(verificationResults, false, true))
	fmt.Printf("Perfect Matches:    %d\n", countMatches(verificationResults, true, true))
	fmt.Printf("Excel File:        %s\n", excelPath)
}

func verifyAllDataWithContext(ctx context.Context, srcDB, dstDB *sql.DB, client kmsproto.KMSClient, token, driver string) []VerificationRecord {
	log.Printf("Starting verification process...")
	
	// Read all encrypted records from destination
	encryptedQuery := "SELECT source_id, encrypted_pan, encrypted_cvv, other_data FROM encrypted_cards ORDER BY source_id"
	if driver == "mysql" || driver == "postgres" {
		encryptedQuery = "SELECT source_id, encrypted_pan, encrypted_cvv, other_data FROM encrypted_cards ORDER BY source_id"
	}

	log.Printf("Reading encrypted records from destination database...")
	encryptedRows, err := dstDB.Query(encryptedQuery)
	if err != nil {
		log.Fatalf("Failed to query encrypted data: %v", err)
	}
	defer encryptedRows.Close()

	// Build a map of encrypted records by source_id
	encryptedMap := make(map[int64]EncryptedRecord)
	encryptedCount := 0
	for encryptedRows.Next() {
		select {
		case <-ctx.Done():
			log.Printf("Verification cancelled: %v", ctx.Err())
			return nil
		default:
		}
		
		var rec EncryptedRecord
		if err := encryptedRows.Scan(&rec.SourceID, &rec.EncryptedPAN, &rec.EncryptedCVV, &rec.OtherData); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		encryptedMap[rec.SourceID] = rec
		encryptedCount++
	}
	log.Printf("Loaded %d encrypted records into memory", encryptedCount)

	// Read all original records from source
	log.Printf("Reading original records from source database...")
	originalRows, err := srcDB.Query("SELECT id, card_no, cvv, other_data FROM cards_to_encrypt ORDER BY id")
	if err != nil {
		log.Fatalf("Failed to query original data: %v", err)
	}
	defer originalRows.Close()

	// Decrypt and compare
	var results []VerificationRecord
	var wg sync.WaitGroup
	// Use large buffer to avoid blocking (will be set after we know record count)
	var resultsChan chan VerificationRecord
	var mu sync.Mutex // Protect results slice

	// Process in parallel
	// Use fewer workers for verification to avoid overwhelming HSM
	verifyWorkerCount := WorkerCount
	if verifyWorkerCount > 10 {
		verifyWorkerCount = 10 // Cap at 10 workers for HSM operations
		log.Printf("Reduced worker count to %d for HSM verification (to avoid overwhelming HSM)", verifyWorkerCount)
	}
	semaphore := make(chan struct{}, verifyWorkerCount)
	totalProcessed := atomic.Uint64{}
	timeoutCount := atomic.Uint64{}

	log.Printf("Starting parallel decryption and verification (using %d workers)...", verifyWorkerCount)
	startTime := time.Now()

	// Read all original records first
	var originalRecords []CardRecord
	for originalRows.Next() {
		select {
		case <-ctx.Done():
			log.Printf("Verification cancelled while reading records: %v", ctx.Err())
			return nil
		default:
		}
		
		var original CardRecord
		if err := originalRows.Scan(&original.ID, &original.CardNo, &original.CVV, &original.OtherData); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		originalRecords = append(originalRecords, original)
	}
	log.Printf("Loaded %d original records, starting verification...", len(originalRecords))

	// Create results channel with large buffer (2x records to avoid blocking)
	resultsChan = make(chan VerificationRecord, len(originalRecords)*2)

	// Start collecting results immediately in a separate goroutine (before we start sending)
	collectionDone := make(chan bool, 1)
	expectedCount := len(originalRecords) // We expect one result per original record
	go func() {
		lastProgressTime := time.Now()
		lastProgressCount := 0
		stuckThreshold := 30 * time.Second // If no progress for 30 seconds, warn
		maxWaitTime := 10 * time.Minute     // Maximum time to wait for all results
		
		for {
			select {
			case result, ok := <-resultsChan:
				if !ok {
					// Channel closed, all results collected
					collectionDone <- true
					return
				}
				
				mu.Lock()
				results = append(results, result)
				currentCount := len(results)
				mu.Unlock()

				// Check for stuck progress
				now := time.Now()
				if currentCount > lastProgressCount {
					lastProgressTime = now
					lastProgressCount = currentCount
				} else if now.Sub(lastProgressTime) > stuckThreshold {
					log.Printf("WARNING: No progress for %v seconds. Current: %d/%d records", 
						stuckThreshold, currentCount, expectedCount)
					log.Printf("  This may indicate HSM/decrypt issues. Continuing to wait...")
					lastProgressTime = now // Reset timer
				}

				// Progress update
				if currentCount%50 == 0 || currentCount == expectedCount {
					elapsed := time.Since(startTime)
					if elapsed.Seconds() > 0 {
						rate := float64(currentCount) / elapsed.Seconds()
						fmt.Printf("Collected: %d/%d results (%.1f records/sec)...\r", currentCount, expectedCount, rate)
					}
				}
				
				// Check if we've collected all expected results
				if currentCount >= expectedCount {
					log.Printf("\nAll expected results collected (%d/%d)", currentCount, expectedCount)
					collectionDone <- true
					return
				}
				
			case <-time.After(maxWaitTime):
				// Force timeout after max wait time
				log.Printf("WARNING: Maximum wait time (%v) reached. Collected %d/%d results. Forcing completion...", 
					maxWaitTime, len(results), expectedCount)
				collectionDone <- true
				return
			}
		}
	}()

	// Process records in parallel
	for _, original := range originalRecords {
		select {
		case <-ctx.Done():
			log.Printf("Verification cancelled: %v", ctx.Err())
			// Wait for current goroutines to finish
			wg.Wait()
			close(resultsChan)
			// Collect what we have so far
			for result := range resultsChan {
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}
			return results
		default:
		}
		
		encrypted, exists := encryptedMap[original.ID]
		if !exists {
			resultsChan <- VerificationRecord{
				SourceID:     original.ID,
				OriginalPAN:  original.CardNo,
				OriginalCVV:  original.CVV,
				EncryptedPAN: "", // No encrypted value found
				EncryptedCVV: "", // No encrypted value found
				PANError:     "Encrypted record not found",
				CVVError:     "Encrypted record not found",
				OtherData:    original.OtherData,
			}
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore
		go func(orig CardRecord, enc EncryptedRecord) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			// Create a timeout context for this single record verification
			recordCtx, recordCancel := context.WithTimeout(ctx, 15*time.Second)
			defer recordCancel()

			// Use a channel to detect if verifyRecord completes or times out
			done := make(chan VerificationRecord, 1)
			go func() {
				verification := verifyRecord(recordCtx, client, token, orig, enc)
				done <- verification
			}()

			var verification VerificationRecord
			select {
			case verification = <-done:
				// Normal completion
			case <-recordCtx.Done():
				// Timeout - create error record
				timeoutCount.Add(1)
				verification = VerificationRecord{
					SourceID:    orig.ID,
					OriginalPAN: orig.CardNo,
					OriginalCVV: orig.CVV,
					PANError:    "Timeout after 15s (record may be stuck)",
					CVVError:    "Timeout after 15s (record may be stuck)",
					OtherData:   orig.OtherData,
				}
				count := timeoutCount.Load()
				if count <= 5 || count%10 == 0 {
					log.Printf("WARNING: Record %d verification timed out (total timeouts: %d)", orig.ID, count)
				}
			}

			resultsChan <- verification
			
			// Progress logging
			processed := totalProcessed.Add(1)
			if processed%50 == 0 || processed == uint64(len(originalRecords)) {
				elapsed := time.Since(startTime)
				if elapsed.Seconds() > 0 {
					rate := float64(processed) / elapsed.Seconds()
					fmt.Printf("Progress: %d/%d records verified (%.1f records/sec)...\r", processed, len(originalRecords), rate)
				}
			}
		}(original, encrypted)
	}

	// Close results channel when all goroutines complete (or timeout)
	go func() {
		// Wait for all goroutines with a timeout
		done := make(chan bool, 1)
		go func() {
			wg.Wait()
			done <- true
		}()
		
		select {
		case <-done:
			close(resultsChan)
			log.Printf("\nAll verification goroutines completed")
		case <-time.After(15 * time.Minute):
			log.Printf("WARNING: Some goroutines may still be running. Closing channel anyway...")
			close(resultsChan)
		}
	}()

	// Wait for collection to complete
	log.Printf("Collecting verification results...")
	<-collectionDone

	fmt.Printf("\n")
	timeouts := timeoutCount.Load()
	if timeouts > 0 {
		log.Printf("WARNING: %d records timed out during verification (may indicate HSM issues)", timeouts)
	}
	log.Printf("Verification complete: %d records processed", len(results))
	return results
}

var (
	decryptErrorCount atomic.Uint64
	firstDecryptError sync.Once
)

func verifyRecord(ctx context.Context, client kmsproto.KMSClient, token string, original CardRecord, encrypted EncryptedRecord) VerificationRecord {
	verification := VerificationRecord{
		SourceID:     original.ID,
		OriginalPAN:  original.CardNo,
		OriginalCVV:  original.CVV,
		EncryptedPAN: encrypted.EncryptedPAN, // Store encrypted value for display
		EncryptedCVV: encrypted.EncryptedCVV, // Store encrypted value for display
		OtherData:    original.OtherData,
	}

	// Decrypt PAN
	panNonce, panCiphertext, err := kmslib.SplitNonceAndCiphertext(encrypted.EncryptedPAN, kmslib.AESGCMNonceSize)
	if err != nil {
		verification.PANError = fmt.Sprintf("Split error: %v", err)
	} else {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second) // Increased timeout
		if token != "" {
			reqCtx = metadata.AppendToOutgoingContext(reqCtx, "authorization", "Bearer "+token)
		}
		panResp, err := client.Decrypt(reqCtx, &kmsproto.DecryptRequest{
			Ciphertext: panCiphertext,
			Nonce:      panNonce,
		})
		cancel()
		if err != nil {
			verification.PANError = fmt.Sprintf("Decrypt error: %v", err)
			// Log first few decrypt errors for debugging
			count := decryptErrorCount.Add(1)
			if count <= 3 {
				log.Printf("Decrypt error (record %d, PAN): %v", original.ID, err)
			}
		} else {
			verification.DecryptedPAN = string(panResp.Plaintext)
			verification.PANMatch = verification.DecryptedPAN == verification.OriginalPAN
		}
	}

	// Decrypt CVV
	cvvNonce, cvvCiphertext, err := kmslib.SplitNonceAndCiphertext(encrypted.EncryptedCVV, kmslib.AESGCMNonceSize)
	if err != nil {
		verification.CVVError = fmt.Sprintf("Split error: %v", err)
	} else {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second) // Increased timeout
		if token != "" {
			reqCtx = metadata.AppendToOutgoingContext(reqCtx, "authorization", "Bearer "+token)
		}
		cvvResp, err := client.Decrypt(reqCtx, &kmsproto.DecryptRequest{
			Ciphertext: cvvCiphertext,
			Nonce:      cvvNonce,
		})
		cancel()
		if err != nil {
			verification.CVVError = fmt.Sprintf("Decrypt error: %v", err)
			// Log first few decrypt errors for debugging
			count := decryptErrorCount.Add(1)
			if count <= 3 {
				log.Printf("Decrypt error (record %d, CVV): %v", original.ID, err)
			}
		} else {
			verification.DecryptedCVV = string(cvvResp.Plaintext)
			verification.CVVMatch = verification.DecryptedCVV == verification.OriginalCVV
		}
	}

	return verification
}

func countMatches(results []VerificationRecord, checkPAN, checkCVV bool) int {
	count := 0
	for _, r := range results {
		if checkPAN && checkCVV {
			if r.PANMatch && r.CVVMatch && r.PANError == "" && r.CVVError == "" {
				count++
			}
		} else if checkPAN {
			if r.PANMatch && r.PANError == "" {
				count++
			}
		} else if checkCVV {
			if r.CVVMatch && r.CVVError == "" {
				count++
			}
		}
	}
	return count
}

func exportToExcel(results []VerificationRecord, filePath string, maskData bool) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing Excel file: %v", err)
		}
	}()

	sheetName := "Verification Results"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(index)

	// Set headers - Show the full flow: Original → Encrypted → Decrypted → Match
	headers := []string{"Source ID", "Original PAN", "Encrypted PAN", "Decrypted PAN", "PAN Match", "PAN Error",
		"Original CVV", "Encrypted CVV", "Decrypted CVV", "CVV Match", "CVV Error", "Other Data"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	// Style headers
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	if err == nil {
		f.SetCellStyle(sheetName, "A1", fmt.Sprintf("%c1", 'A'+len(headers)-1), headerStyle)
	}

	// Write data - Show full flow: Original → Encrypted → Decrypted → Match
	for i, result := range results {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), result.SourceID)
		
		// PAN columns: Original → Encrypted → Decrypted → Match → Error
		// Original PAN - always mask for security
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), maskPAN(result.OriginalPAN))
		
		// Encrypted PAN - show the encrypted base64 string (truncated if too long)
		encryptedPAN := result.EncryptedPAN
		if len(encryptedPAN) > 50 {
			encryptedPAN = encryptedPAN[:50] + "..."
		}
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), encryptedPAN)
		
		// Decrypted PAN - show actual value if maskData is false
		if maskData {
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), maskPAN(result.DecryptedPAN))
		} else {
			f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), result.DecryptedPAN)
		}
		
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), boolToYesNo(result.PANMatch))
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), result.PANError)
		
		// CVV columns: Original → Encrypted → Decrypted → Match → Error
		// Original CVV - always mask for security
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), maskCVV(result.OriginalCVV))
		
		// Encrypted CVV - show the encrypted base64 string (truncated if too long)
		encryptedCVV := result.EncryptedCVV
		if len(encryptedCVV) > 50 {
			encryptedCVV = encryptedCVV[:50] + "..."
		}
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), encryptedCVV)
		
		// Decrypted CVV - show actual value if maskData is false
		if maskData {
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), maskCVV(result.DecryptedCVV))
		} else {
			f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), result.DecryptedCVV)
		}
		
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), boolToYesNo(result.CVVMatch))
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), result.CVVError)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), result.OtherData)

		// Color code rows: green for perfect match, red for errors
		if result.PANMatch && result.CVVMatch && result.PANError == "" && result.CVVError == "" {
			style, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"#C6EFCE"}, Pattern: 1}})
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("L%d", row), style)
		} else if result.PANError != "" || result.CVVError != "" {
			style, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFC7CE"}, Pattern: 1}})
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("L%d", row), style)
		} else if !result.PANMatch || !result.CVVMatch {
			style, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFEB9C"}, Pattern: 1}})
			f.SetCellStyle(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("L%d", row), style)
		}
	}

	// Auto-size columns - wider for encrypted columns (base64 strings)
	for i := 0; i < len(headers); i++ {
		col := string(rune('A' + i))
		width := 15.0
		// Encrypted columns need more width (C for PAN, H for CVV)
		if i == 2 || i == 7 { // Column C (Encrypted PAN) and H (Encrypted CVV)
			width = 50.0
		}
		f.SetColWidth(sheetName, col, col, width)
	}

	// Create summary sheet
	summarySheet := "Summary"
	_, err = f.NewSheet(summarySheet)
	if err != nil {
		return fmt.Errorf("failed to create summary sheet: %w", err)
	}

	total := len(results)
	panMatches := countMatches(results, true, false)
	cvvMatches := countMatches(results, false, true)
	perfectMatches := countMatches(results, true, true)
	errors := total - perfectMatches

	f.SetCellValue(summarySheet, "A1", "Verification Summary")
	f.SetCellValue(summarySheet, "A2", "Total Records")
	f.SetCellValue(summarySheet, "B2", total)
	f.SetCellValue(summarySheet, "A3", "Perfect Matches")
	f.SetCellValue(summarySheet, "B3", perfectMatches)
	f.SetCellValue(summarySheet, "A4", "PAN Matches")
	f.SetCellValue(summarySheet, "B4", panMatches)
	f.SetCellValue(summarySheet, "A5", "CVV Matches")
	f.SetCellValue(summarySheet, "B5", cvvMatches)
	f.SetCellValue(summarySheet, "A6", "Errors/Mismatches")
	f.SetCellValue(summarySheet, "B6", errors)
	f.SetCellValue(summarySheet, "A7", "Success Rate")
	if total > 0 {
		f.SetCellValue(summarySheet, "B7", fmt.Sprintf("%.2f%%", float64(perfectMatches)*100/float64(total)))
	}

	// Delete default Sheet1
	f.DeleteSheet("Sheet1")

	// Ensure directory exists before saving
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s' for Excel file: %w", dir, err)
		}
		log.Printf("Ensured directory exists: %s", dir)
	}

	// Convert to absolute path for better error messages
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath // Fallback to original if Abs fails
	}

	// Save file
	log.Printf("Saving Excel file to: %s", absPath)
	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("failed to save Excel file to '%s': %w", absPath, err)
	}

	log.Printf("✓ Excel file saved successfully: %s", absPath)
	return nil
}

func maskPAN(pan string) string {
	if len(pan) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(pan)-4) + pan[len(pan)-4:]
}

func maskCVV(cvv string) string {
	if cvv == "" {
		return ""
	}
	return "***"
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// --- Helper Functions (保持不變) ---
func loginToKMS(conn *grpc.ClientConn, username, password string) (string, error) {
	authClient := kmsproto.NewAuthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := authClient.Login(ctx, &kmsproto.LoginRequest{Username: username, Password: password})
	if err != nil {
		return "", err
	}
	return resp.Token, nil
}
func worker(id int, jobs <-chan CardRecord, results chan<- EncryptedRecord, wg *sync.WaitGroup, client kmsproto.KMSClient, token string) {
	defer wg.Done()
	ctx := context.Background()
	errorCountLocal := 0
	successCountLocal := 0
	firstError := true

	for r := range jobs {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second) // Increased timeout
		if token != "" {
			reqCtx = metadata.AppendToOutgoingContext(reqCtx, "authorization", "Bearer "+token)
		} else {
			if firstError {
				log.Printf("WARNING (worker %d): No token provided, encryption may fail if JWT auth is enabled", id)
				firstError = false
			}
		}

		encPAN, err := client.Encrypt(reqCtx, &kmsproto.EncryptRequest{Plaintext: []byte(r.CardNo)})
		if err != nil {
			cancel()
			errorCount.Add(1)
			errorCountLocal++
			if errorCountLocal <= 3 { // Log first 3 errors per worker
				log.Printf("ERROR (worker %d, record %d, PAN): %v", id, r.ID, err)
				// Check if it's an authentication error
				if strings.Contains(err.Error(), "Unauthenticated") || strings.Contains(err.Error(), "unauthorized") {
					log.Printf("  -> Authentication error! Token may be invalid or expired")
				}
			}
			continue
		}

		encCVV, err := client.Encrypt(reqCtx, &kmsproto.EncryptRequest{Plaintext: []byte(r.CVV)})
		cancel() // Always cancel after both operations complete
		if err != nil {
			errorCount.Add(1)
			errorCountLocal++
			// Always log first error, then log every 100th error
			if errorCountLocal == 1 || errorCountLocal%100 == 0 {
				errMsg := err.Error()
				log.Printf("ERROR (worker %d, record %d, CVV): %v", id, r.ID, errMsg)
				if strings.Contains(strings.ToLower(errMsg), "unauthenticated") ||
					strings.Contains(strings.ToLower(errMsg), "unauthorized") ||
					strings.Contains(strings.ToLower(errMsg), "permission denied") {
					log.Printf("  -> AUTHENTICATION ERROR! Token may be invalid, expired, or KMS_JWT_SECRET mismatch")
				}
			}
			continue
		}

		successCountLocal++
		// Combine nonce + ciphertext into single base64 strings
		// Combine nonce + ciphertext into single base64 strings
		encryptedPAN := kmslib.CombineNonceAndCiphertext(encPAN.Nonce, encPAN.Ciphertext)
		encryptedCVV := kmslib.CombineNonceAndCiphertext(encCVV.Nonce, encCVV.Ciphertext)
		results <- EncryptedRecord{
			SourceID:     r.ID,
			EncryptedPAN: encryptedPAN,
			EncryptedCVV: encryptedCVV,
			OtherData:    r.OtherData,
		}
	}
}
func batchWriter(db *sql.DB, results <-chan EncryptedRecord, wg *sync.WaitGroup, driver string) {
	defer wg.Done()
	batch := make([]EncryptedRecord, 0, BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := insertBatch(db, batch, driver); err != nil {
			log.Printf("Failed to insert batch of %d records: %v", len(batch), err)
			errorCount.Add(uint64(len(batch)))
		} else {
			processedCount.Add(uint64(len(batch)))
		}
		batch = batch[:0]
	}
	for r := range results {
		batch = append(batch, r)
		if len(batch) >= BatchSize {
			flush()
		}
	}
	flush()
}
func insertBatch(db *sql.DB, batch []EncryptedRecord, driver string) error {
	if len(batch) == 0 {
		return nil
	}
	var queryBuilder strings.Builder
	var params []interface{}
	if driver == "postgres" {
		queryBuilder.WriteString("INSERT INTO encrypted_users (id, credit_card_enc, email_enc, full_name_enc) VALUES ")
	} else {
		// Store as single string fields: encrypted_pan and encrypted_cvv
		queryBuilder.WriteString("INSERT INTO encrypted_cards (source_id, encrypted_pan, encrypted_cvv, other_data) VALUES ")
	}
	for i, r := range batch {
		if i > 0 {
			queryBuilder.WriteString(",")
		}
		if driver == "postgres" {
			queryBuilder.WriteString("(?, ?, ?, ?)")
		} else {
			queryBuilder.WriteString("(?, ?, ?, ?)")
		}
		if driver == "postgres" {
			params = append(params, r.SourceID, r.EncryptedPAN, r.EncryptedCVV, r.OtherData)
		} else {
			params = append(params, r.SourceID, r.EncryptedPAN, r.EncryptedCVV, r.OtherData)
		}
	}

	query := queryBuilder.String()
	_, err := db.Exec(query, params...)
	if err != nil {
		// Log the actual SQL error with first record details for debugging
		log.Printf("DATABASE INSERT ERROR (batch size: %d): %v", len(batch), err)
		log.Printf("  SQL Query: %s", query)
		if len(batch) > 0 {
			first := batch[0]
			log.Printf("  First record - SourceID: %d, PAN length: %d, CVV length: %d",
				first.SourceID, len(first.EncryptedPAN), len(first.EncryptedCVV))
			log.Printf("  First record PAN preview: %s...", first.EncryptedPAN[:min(50, len(first.EncryptedPAN))])
		}
		// Check for common SQL Server errors
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "invalid column name") {
			log.Printf("  -> COLUMN NAME ERROR! Table schema may not match expected columns")
			log.Printf("  -> Expected columns: source_id, encrypted_pan, encrypted_cvv, other_data")
		} else if strings.Contains(errStr, "cannot insert null") {
			log.Printf("  -> NULL VALUE ERROR! Some required fields are NULL")
		} else if strings.Contains(errStr, "string or binary data would be truncated") {
			log.Printf("  -> DATA TRUNCATION ERROR! Encrypted data too long for column")
		}
	}
	return err
}
func feedRecords(db *sql.DB, jobs chan<- CardRecord) {
	rows, err := db.Query("SELECT id, card_no, cvv, other_data FROM cards_to_encrypt")
	if err != nil {
		log.Printf("ERROR: Failed to query source database: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var r CardRecord
		if err := rows.Scan(&r.ID, &r.CardNo, &r.CVV, &r.OtherData); err != nil {
			log.Printf("ERROR: Failed to scan record: %v", err)
			continue
		}
		jobs <- r
		count++
	}
	if count == 0 {
		log.Printf("WARNING: No records found in source database table 'cards_to_encrypt'")
		log.Printf("  Please check:")
		log.Printf("  1. Source database connection is correct")
		log.Printf("  2. Table 'cards_to_encrypt' exists")
		log.Printf("  3. Table has data")
	} else {
		log.Printf("Found %d records in source database", count)
	}
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func loadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
