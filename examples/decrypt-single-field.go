package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	kmslib "kms/internal/kms"
	kmsproto "kms/proto"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Example: How to decrypt data stored in single-field format (encrypted_pan, encrypted_cvv)
// Database stores: base64(nonce + ciphertext) as a single string field

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run decrypt-single-field.go <driver> <dsn> <kms_addr>")
		fmt.Println("Example: go run decrypt-single-field.go sqlserver 'sqlserver://user:pass@localhost:1433?database=db' 127.0.0.1:50051")
		os.Exit(1)
	}

	driver := os.Args[1]
	dsn := os.Args[2]
	kmsAddr := os.Args[3]

	// 1. Connect to database
	db, err := sql.Open(driver, dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping DB: %v", err)
	}

	// 2. Connect to KMS
	conn, err := grpc.Dial(kmsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial KMS: %v", err)
	}
	defer conn.Close()

	client := kmsproto.NewKMSClient(conn)
	token := os.Getenv("KMS_BEARER_TOKEN")

	// 3. Query encrypted data
	query := "SELECT TOP 5 id, encrypted_pan, encrypted_cvv FROM encrypted_cards"
	if driver == "mysql" {
		query = "SELECT id, encrypted_pan, encrypted_cvv FROM encrypted_cards LIMIT 5"
	}

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	fmt.Println("\n=== Decrypting Single-Field Encrypted Data ===")
	fmt.Println("ID | Masked PAN | CVV Status")
	fmt.Println("---|------------|------------")

	ctx := context.Background()
	for rows.Next() {
		var id int
		var encryptedPAN, encryptedCVV string

		if err := rows.Scan(&id, &encryptedPAN, &encryptedCVV); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		// 4. Split the combined field (base64(nonce + ciphertext))
		panNonce, panCiphertext, err := kmslib.SplitNonceAndCiphertext(encryptedPAN, kmslib.AESGCMNonceSize)
		if err != nil {
			log.Printf("Failed to split PAN for ID %d: %v", id, err)
			continue
		}

		cvvNonce, cvvCiphertext, err := kmslib.SplitNonceAndCiphertext(encryptedCVV, kmslib.AESGCMNonceSize)
		if err != nil {
			log.Printf("Failed to split CVV for ID %d: %v", id, err)
			continue
		}

		// 5. Decrypt PAN
		reqCtx := ctx
		if token != "" {
			reqCtx = metadata.AppendToOutgoingContext(reqCtx, "authorization", "Bearer "+token)
		}

		panResp, err := client.Decrypt(reqCtx, &kmsproto.DecryptRequest{
			Ciphertext: panCiphertext,
			Nonce:      panNonce,
		})

		// 6. Decrypt CVV (just verify it works, don't display it)
		cvvResp, cvvErr := client.Decrypt(reqCtx, &kmsproto.DecryptRequest{
			Ciphertext: cvvCiphertext,
			Nonce:      cvvNonce,
		})

		// 7. Display masked PAN (safe to log)
		maskedPAN := "ERROR"
		cvvStatus := "ERROR"
		if err == nil && panResp != nil {
			plain := string(panResp.Plaintext)
			if len(plain) > 4 {
				maskedPAN = fmt.Sprintf("****%s", plain[len(plain)-4:])
			} else {
				maskedPAN = "****"
			}
		}

		if cvvErr == nil && cvvResp != nil {
			cvvStatus = "OK"
		}

		fmt.Printf("%d  | %s | %s\n", id, maskedPAN, cvvStatus)
	}

	fmt.Println("\n=== Decryption Complete ===")
}

// Alternative: If you want to manually decode without using the helper
func manualDecodeExample(encryptedBase64 string) {
	// Decode base64
	combined, _ := base64.StdEncoding.DecodeString(encryptedBase64)

	// Split: first 12 bytes = nonce, rest = ciphertext
	nonce := combined[:12]
	ciphertext := combined[12:]

	fmt.Printf("Nonce (hex): %x\n", nonce)
	fmt.Printf("Ciphertext length: %d bytes\n", len(ciphertext))
}

