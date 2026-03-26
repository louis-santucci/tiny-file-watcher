package interceptor

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// NewAuthInterceptors returns a unary and a stream server interceptor that
// validate the Bearer id_token from incoming gRPC metadata.
func NewAuthInterceptors(v TokenVerifier) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	return unaryAuth(v), streamAuth(v)
}

func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}
	auth := vals[0]
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "authorization header must be Bearer token")
	}
	return strings.TrimPrefix(auth, "Bearer "), nil
}

func unaryAuth(v TokenVerifier) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		token, err := extractBearerToken(ctx)
		if err != nil {
			return nil, err
		}
		if err := v.Verify(ctx, token); err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}
		return handler(ctx, req)
	}
}

func streamAuth(v TokenVerifier) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		token, err := extractBearerToken(ss.Context())
		if err != nil {
			return err
		}
		if err := v.Verify(ss.Context(), token); err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}
		return handler(srv, ss)
	}
}
