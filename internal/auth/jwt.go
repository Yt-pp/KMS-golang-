package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// JWTConfig drives how we validate incoming bearer tokens.
type JWTConfig struct {
	Secret   string // HMAC secret (HS256)
	Audience string // optional
	Issuer   string // optional
}

// UnaryServerInterceptor validates Authorization: Bearer <token> if Secret is set.
// If Secret is empty, the interceptor is a no-op (open).
func UnaryServerInterceptor(cfg JWTConfig) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if cfg.Secret == "" {
			// Auth disabled.
			return handler(ctx, req)
		}

		// Allow unauthenticated access to the login method so clients can obtain a token.
		if strings.HasSuffix(info.FullMethod, "/Login") {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := ""
		if vals := md.Get("authorization"); len(vals) > 0 {
			authHeader = vals[0]
		}
		if authHeader == "" {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}
		if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization scheme")
		}
		raw := strings.TrimSpace(authHeader[7:])
		if raw == "" {
			return nil, status.Error(codes.Unauthenticated, "empty bearer token")
		}

		claims := jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(cfg.Secret), nil
		})
		if err != nil || !token.Valid {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		now := time.Now()
		if claims.ExpiresAt != nil && now.After(claims.ExpiresAt.Time) {
			return nil, status.Error(codes.Unauthenticated, "token expired")
		}
		if claims.NotBefore != nil && now.Before(claims.NotBefore.Time) {
			return nil, status.Error(codes.Unauthenticated, "token not yet valid")
		}
		if cfg.Audience != "" {
			if len(claims.Audience) == 0 || claims.Audience[0] != cfg.Audience {
				return nil, status.Error(codes.Unauthenticated, "invalid audience")
			}
		}
		if cfg.Issuer != "" {
			if claims.Issuer != cfg.Issuer {
				return nil, status.Error(codes.Unauthenticated, "invalid issuer")
			}
		}

		return handler(ctx, req)
	}
}

// IssueToken creates a signed JWT string for the given subject (e.g. username).
// This uses HS256 and the same cfg that the interceptor validates with.
func IssueToken(cfg JWTConfig, subject string, ttl time.Duration) (string, error) {
	if cfg.Secret == "" {
		return "", errors.New("JWT secret not configured")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	if cfg.Audience != "" {
		claims.Audience = []string{cfg.Audience}
	}
	if cfg.Issuer != "" {
		claims.Issuer = cfg.Issuer
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}


