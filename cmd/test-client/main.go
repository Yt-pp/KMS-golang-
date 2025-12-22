package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	kmsproto "kms/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  go run ./cmd/test-client login              # Login and get token")
		fmt.Println("  go run ./cmd/test-client encrypt <text>     # Encrypt text (requires token)")
		fmt.Println("  go run ./cmd/test-client decrypt <cipher> <nonce> # Decrypt (requires token)")
		fmt.Println("\nSet KMS_GRPC_ADDR to change server address (default: 127.0.0.1:50051)")
		fmt.Println("Set KMS_BEARER_TOKEN for encrypt/decrypt operations")
		os.Exit(1)
	}

	addr := getenvDefault("KMS_GRPC_ADDR", "127.0.0.1:50051")
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	cmd := os.Args[1]
	switch cmd {
	case "login":
		testLogin(conn)
	case "encrypt":
		if len(os.Args) < 3 {
			log.Fatal("encrypt requires text argument")
		}
		testEncrypt(conn, os.Args[2])
	case "decrypt":
		if len(os.Args) < 4 {
			log.Fatal("decrypt requires cipher and nonce arguments (as hex)")
		}
		testDecrypt(conn, os.Args[2], os.Args[3])
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}

func testLogin(conn *grpc.ClientConn) {
	authClient := kmsproto.NewAuthClient(conn)
	resp, err := authClient.Login(context.Background(), &kmsproto.LoginRequest{
		Username: "demo",
		Password: "demo123",
	})
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	fmt.Printf("Token: %s\n", resp.Token)
	fmt.Printf("\nYou can use this token by setting:\n")
	fmt.Printf("  $env:KMS_BEARER_TOKEN=\"%s\"\n", resp.Token)
}

func testEncrypt(conn *grpc.ClientConn, plaintext string) {
	token := os.Getenv("KMS_BEARER_TOKEN")
	if token == "" {
		log.Fatal("KMS_BEARER_TOKEN not set. Please login first and set the token.")
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	kmsClient := kmsproto.NewKMSClient(conn)

	resp, err := kmsClient.Encrypt(ctx, &kmsproto.EncryptRequest{
		Plaintext: []byte(plaintext),
	})
	if err != nil {
		log.Fatalf("encrypt failed: %v", err)
	}

	fmt.Printf("Plaintext: %s\n", plaintext)
	fmt.Printf("Ciphertext (hex): %x\n", resp.Ciphertext)
	fmt.Printf("Nonce (hex): %x\n", resp.Nonce)
}

func testDecrypt(conn *grpc.ClientConn, cipherHex, nonceHex string) {
	token := os.Getenv("KMS_BEARER_TOKEN")
	if token == "" {
		log.Fatal("KMS_BEARER_TOKEN not set. Please login first and set the token.")
	}

	// Decode hex strings
	ciphertext := hexDecode(cipherHex)
	nonce := hexDecode(nonceHex)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	kmsClient := kmsproto.NewKMSClient(conn)

	resp, err := kmsClient.Decrypt(ctx, &kmsproto.DecryptRequest{
		Ciphertext: ciphertext,
		Nonce:      nonce,
	})
	if err != nil {
		log.Fatalf("decrypt failed: %v", err)
	}

	fmt.Printf("Decrypted: %s\n", string(resp.Plaintext))
}

func hexDecode(s string) []byte {
	result, err := hex.DecodeString(s)
	if err != nil {
		log.Fatalf("invalid hex string: %v", err)
	}
	return result
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

