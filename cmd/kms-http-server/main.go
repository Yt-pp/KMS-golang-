package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	kmslib "kms/internal/kms"
	kmsproto "kms/proto"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// REST API request/response structures
type EncryptRequest struct {
	Plaintext string `json:"plaintext"`
	KeyID     string `json:"key_id,omitempty"`
}

type EncryptResponse struct {
	Ciphertext string `json:"ciphertext"` // base64 encoded
	Nonce      string `json:"nonce"`      // base64 encoded
}

type BatchEncryptRequest struct {
	Items []EncryptRequest `json:"items"`
}

type BatchEncryptResponse struct {
	Results []EncryptResponse `json:"results"`
	Errors  []string          `json:"errors,omitempty"`
}

type DecryptRequest struct {
	// Legacy mode: separate fields
	Ciphertext string `json:"ciphertext,omitempty"` // base64 encoded
	Nonce      string `json:"nonce,omitempty"`      // base64 encoded

	// New mode: single combined string (base64 of nonce + ciphertext), e.g. encrypted_pan/encrypted_cvv
	Encrypted string `json:"encrypted,omitempty"`

	KeyID string `json:"key_id,omitempty"`
}

type DecryptResponse struct {
	Plaintext string `json:"plaintext"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// HTTP server that wraps gRPC KMS service
type HTTPServer struct {
	grpcAddr   string
	grpcClient kmsproto.KMSClient
	grpcConn   *grpc.ClientConn
	token      string
}

func main() {
	grpcAddr := getenvDefault("KMS_GRPC_ADDR", "127.0.0.1:50051")
	httpAddr := getenvDefault("KMS_HTTP_ADDR", ":8080")
	token := os.Getenv("KMS_BEARER_TOKEN")

	// Connect to gRPC server
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to gRPC server at %s: %v", grpcAddr, err)
	}
	defer conn.Close()

	client := kmsproto.NewKMSClient(conn)

	server := &HTTPServer{
		grpcAddr:   grpcAddr,
		grpcClient: client,
		grpcConn:   conn,
		token:      token,
	}

	// Setup routes
	r := mux.NewRouter()
	r.HandleFunc("/health", server.healthHandler).Methods("GET")
	r.HandleFunc("/api/v1/encrypt", server.encryptHandler).Methods("POST")
	r.HandleFunc("/api/v1/encrypt/batch", server.batchEncryptHandler).Methods("POST")
	r.HandleFunc("/api/v1/decrypt", server.decryptHandler).Methods("POST")

	// CORS middleware for SSIS
	r.Use(corsMiddleware)

	log.Printf("KMS HTTP server listening on %s (gRPC backend: %s)", httpAddr, grpcAddr)
	if err := http.ListenAndServe(httpAddr, r); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *HTTPServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *HTTPServer) encryptHandler(w http.ResponseWriter, r *http.Request) {
	var req EncryptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Plaintext == "" {
		respondError(w, http.StatusBadRequest, "plaintext is required")
		return
	}

	ctx := s.createContext(r)
	resp, err := s.grpcClient.Encrypt(ctx, &kmsproto.EncryptRequest{
		Plaintext: []byte(req.Plaintext),
		KeyId:     req.KeyID,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EncryptResponse{
		Ciphertext: base64.StdEncoding.EncodeToString(resp.Ciphertext),
		Nonce:      base64.StdEncoding.EncodeToString(resp.Nonce),
	})
}

func (s *HTTPServer) batchEncryptHandler(w http.ResponseWriter, r *http.Request) {
	var req BatchEncryptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Items) == 0 {
		respondError(w, http.StatusBadRequest, "items array is required")
		return
	}

	// Limit batch size for performance
	if len(req.Items) > 1000 {
		respondError(w, http.StatusBadRequest, "batch size cannot exceed 1000 items")
		return
	}

	ctx := s.createContext(r)
	
	// Pre-allocate results slice for better performance
	results := make([]EncryptResponse, len(req.Items))
	errors := make([]string, 0)
	
	// Use worker pool pattern for better performance
	// Limit concurrent goroutines to avoid overwhelming gRPC connection
	maxWorkers := 50
	if len(req.Items) < maxWorkers {
		maxWorkers = len(req.Items)
	}
	
	type workItem struct {
		index int
		item  EncryptRequest
	}
	
	type result struct {
		index int
		resp  *kmsproto.EncryptResponse
		err   error
	}
	
	workChan := make(chan workItem, len(req.Items))
	resultChan := make(chan result, len(req.Items))
	
	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				resp, err := s.grpcClient.Encrypt(ctx, &kmsproto.EncryptRequest{
					Plaintext: []byte(work.item.Plaintext),
					KeyId:     work.item.KeyID,
				})
				resultChan <- result{index: work.index, resp: resp, err: err}
			}
		}()
	}
	
	// Send work items
	for i, item := range req.Items {
		workChan <- workItem{index: i, item: item}
	}
	close(workChan)
	
	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results (maintain order)
	tempResults := make([]*kmsproto.EncryptResponse, len(req.Items))
	for res := range resultChan {
		if res.err != nil {
			errors = append(errors, res.err.Error())
		} else {
			tempResults[res.index] = res.resp
		}
	}
	
	// Convert to response format (base64 encoding done here, not in goroutines)
	for i, resp := range tempResults {
		if resp != nil {
			results[i] = EncryptResponse{
				Ciphertext: base64.StdEncoding.EncodeToString(resp.Ciphertext),
				Nonce:      base64.StdEncoding.EncodeToString(resp.Nonce),
			}
		}
	}
	
	// Filter out empty results if needed
	finalResults := make([]EncryptResponse, 0, len(results))
	for _, r := range results {
		if r.Ciphertext != "" {
			finalResults = append(finalResults, r)
		}
	}
	results = finalResults

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BatchEncryptResponse{
		Results: results,
		Errors:  errors,
	})
}

func (s *HTTPServer) decryptHandler(w http.ResponseWriter, r *http.Request) {
	var req DecryptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Support both legacy (ciphertext + nonce) and new combined format
	var (
		ciphertext []byte
		nonce      []byte
		err        error
	)

	if req.Encrypted != "" {
		// New combined format: base64(nonce+ciphertext)
		nonce, ciphertext, err = kmslib.SplitNonceAndCiphertext(req.Encrypted, kmslib.AESGCMNonceSize)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid encrypted format: "+err.Error())
			return
		}
	} else {
		// Legacy format: separate ciphertext and nonce fields
		ciphertext, err = base64.StdEncoding.DecodeString(req.Ciphertext)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid ciphertext encoding")
			return
		}

		nonce, err = base64.StdEncoding.DecodeString(req.Nonce)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid nonce encoding")
			return
		}
	}

	ctx := s.createContext(r)
	resp, err := s.grpcClient.Decrypt(ctx, &kmsproto.DecryptRequest{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		KeyId:      req.KeyID,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DecryptResponse{
		Plaintext: string(resp.Plaintext),
	})
}

func (s *HTTPServer) createContext(r *http.Request) context.Context {
	ctx := r.Context()
	
	// Use token from Authorization header or fallback to env var
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	} else if s.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+s.token)
	}
	
	// Add timeout
	ctx, _ = context.WithTimeout(ctx, 30*time.Second)
	return ctx
}

func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

