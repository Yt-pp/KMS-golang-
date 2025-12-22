package main

import (
	"log"
	"os"

	"kms/internal/auth"
	kmslib "kms/internal/kms"
	"kms/internal/server"

	"google.golang.org/grpc"
)

func main() {
	// Configuration via environment variables for simplicity.
	addr := getenvDefault("KMS_GRPC_ADDR", ":50051")
	masterKeyPath := getenvDefault("KMS_MASTER_KEY_PATH", "master.key")
	jwtSecret := os.Getenv("KMS_JWT_SECRET")
	jwtAud := os.Getenv("KMS_JWT_AUD")
	jwtIss := os.Getenv("KMS_JWT_ISS")

	mgr, err := kmslib.NewManagerFromFile(masterKeyPath)
	if err != nil {
		log.Fatalf("failed to load master key from %s: %v", masterKeyPath, err)
	}

	var interceptors []grpc.UnaryServerInterceptor
	if jwtSecret != "" {
		jwtCfg := auth.JWTConfig{
			Secret:   jwtSecret,
			Audience: jwtAud,
			Issuer:   jwtIss,
		}
		interceptors = append(interceptors, auth.UnaryServerInterceptor(jwtCfg))
		log.Printf("KMS server: JWT auth enabled (aud=%s, iss=%s)", jwtAud, jwtIss)
		if err := server.Run(addr, mgr, jwtCfg, interceptors...); err != nil {
			log.Fatalf("KMS server exited with error: %v", err)
		}
	} else {
		log.Print("KMS server: JWT auth disabled (KMS_JWT_SECRET not set)")
		if err := server.Run(addr, mgr, auth.JWTConfig{}, interceptors...); err != nil {
			log.Fatalf("KMS server exited with error: %v", err)
		}
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}


