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

	// Use NewManager which supports both file and HSM backends
	var mgr kmslib.Manager
	var err error
	hsmType := os.Getenv("KMS_HSM_TYPE")
	if hsmType != "" {
		log.Printf("KMS server: Using HSM backend (type=%s)", hsmType)
		mgr, err = kmslib.NewManager()
		if err != nil {
			log.Fatalf("failed to initialize HSM manager: %v", err)
		}
	} else {
		log.Printf("KMS server: Using file-based key from %s", masterKeyPath)
		var fileMgr kmslib.Manager
		fileMgr, err = kmslib.NewManagerFromFile(masterKeyPath)
		if err != nil {
			log.Fatalf("failed to load master key from %s: %v", masterKeyPath, err)
		}
		mgr = fileMgr
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


