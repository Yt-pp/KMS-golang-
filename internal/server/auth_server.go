package server

import (
	"context"
	"os"
	"time"

	"kms/internal/auth"
	kmsproto "kms/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthServer implements the Auth gRPC service for demo purposes.
// It authenticates a simple username/password pair from environment variables
// and issues a JWT using the shared JWTConfig.
type AuthServer struct {
	kmsproto.UnimplementedAuthServer
	jwtCfg  auth.JWTConfig
	user    string
	pass    string
	tokenTTL time.Duration
}

func NewAuthServer(cfg auth.JWTConfig) *AuthServer {
	user := os.Getenv("KMS_DEMO_USER")
	if user == "" {
		user = "demo"
	}
	pass := os.Getenv("KMS_DEMO_PASS")
	if pass == "" {
		pass = "demo123"
	}
	return &AuthServer{
		jwtCfg:  cfg,
		user:    user,
		pass:    pass,
		tokenTTL: time.Hour,
	}
}

func (s *AuthServer) Login(ctx context.Context, req *kmsproto.LoginRequest) (*kmsproto.LoginResponse, error) {
	if req.GetUsername() != s.user || req.GetPassword() != s.pass {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	token, err := auth.IssueToken(s.jwtCfg, req.GetUsername(), s.tokenTTL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to issue token: %v", err)
	}

	return &kmsproto.LoginResponse{
		Token: token,
	}, nil
}


