package server

import (
	"context"
	"log"
	"net"

	"kms/internal/auth"
	kmslib "kms/internal/kms"
	kmsproto "kms/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// KMSServer implements the gRPC KMS service.
type KMSServer struct {
	kmsproto.UnimplementedKMSServer
	manager kmslib.Manager
}

func NewKMSServer(mgr kmslib.Manager) *KMSServer {
	return &KMSServer{manager: mgr}
}

func (s *KMSServer) Encrypt(ctx context.Context, req *kmsproto.EncryptRequest) (*kmsproto.EncryptResponse, error) {
	ct, nonce, err := s.manager.Encrypt(req.GetPlaintext())
	if err != nil {
		return nil, err
	}
	return &kmsproto.EncryptResponse{
		Ciphertext: ct,
		Nonce:      nonce,
	}, nil
}

func (s *KMSServer) Decrypt(ctx context.Context, req *kmsproto.DecryptRequest) (*kmsproto.DecryptResponse, error) {
	pt, err := s.manager.Decrypt(req.GetCiphertext(), req.GetNonce())
	if err != nil {
		return nil, err
	}
	return &kmsproto.DecryptResponse{
		Plaintext: pt,
	}, nil
}

// Run starts the gRPC server on the given address, e.g. ":50051".
// You can supply optional unary interceptors (e.g., auth).
// jwtCfg is used by the Auth service to issue tokens.
func Run(addr string, mgr kmslib.Manager, jwtCfg auth.JWTConfig, interceptors ...grpc.UnaryServerInterceptor) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	opts := []grpc.ServerOption{}
	if len(interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	}

	grpcServer := grpc.NewServer(opts...)
	kmsproto.RegisterKMSServer(grpcServer, NewKMSServer(mgr))
	kmsproto.RegisterAuthServer(grpcServer, NewAuthServer(jwtCfg))
	
	// Enable gRPC reflection for tools like grpcurl
	reflection.Register(grpcServer)

	log.Printf("KMS gRPC server listening on %s", addr)
	return grpcServer.Serve(lis)
}


