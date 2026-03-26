package interceptor_test

import (
	"context"
	"errors"
	"testing"

	"tiny-file-watcher/server/interceptor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// stubVerifier lets tests control whether a token is accepted.
type stubVerifier struct{ err error }

func (s stubVerifier) Verify(_ context.Context, _ string) error { return s.err }

func okVerifier() interceptor.TokenVerifier  { return stubVerifier{nil} }
func badVerifier() interceptor.TokenVerifier { return stubVerifier{errors.New("bad token")} }

// ctxWithBearer injects an Authorization header into a new incoming-context.
func ctxWithBearer(token string) context.Context {
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewIncomingContext(context.Background(), md)
}

// passHandler is a trivial grpc.UnaryHandler that returns a fixed string.
func passHandler(_ context.Context, req any) (any, error) { return "ok", nil }

// ── noopVerifier ──────────────────────────────────────────────────────────────

func TestNoopVerifier_AlwaysOK(t *testing.T) {
	v := interceptor.NewNoopVerifier()
	assert.NoError(t, v.Verify(context.Background(), "anything"))
	assert.NoError(t, v.Verify(context.Background(), ""))
}

// ── unary interceptor ─────────────────────────────────────────────────────────

func TestUnaryAuth_MissingMetadata(t *testing.T) {
	unary, _ := interceptor.NewAuthInterceptors(okVerifier())

	_, err := unary(context.Background(), nil, &grpc.UnaryServerInfo{}, passHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryAuth_MissingAuthorizationHeader(t *testing.T) {
	unary, _ := interceptor.NewAuthInterceptors(okVerifier())

	// Metadata present but no authorization key.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-other", "value"))
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{}, passHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.ErrorContains(t, err, "missing authorization header")
}

func TestUnaryAuth_NonBearerScheme(t *testing.T) {
	unary, _ := interceptor.NewAuthInterceptors(okVerifier())

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic dXNlcjpwYXNz"))
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{}, passHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.ErrorContains(t, err, "must be Bearer token")
}

func TestUnaryAuth_InvalidToken(t *testing.T) {
	unary, _ := interceptor.NewAuthInterceptors(badVerifier())

	_, err := unary(ctxWithBearer("bad"), nil, &grpc.UnaryServerInfo{}, passHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.ErrorContains(t, err, "invalid token")
}

func TestUnaryAuth_ValidToken(t *testing.T) {
	unary, _ := interceptor.NewAuthInterceptors(okVerifier())

	resp, err := unary(ctxWithBearer("good-token"), nil, &grpc.UnaryServerInfo{}, passHandler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

// ── stream interceptor ────────────────────────────────────────────────────────

// mockServerStream implements grpc.ServerStream with a configurable context.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m mockServerStream) Context() context.Context { return m.ctx }

func streamWith(ctx context.Context) grpc.ServerStream {
	return mockServerStream{ctx: ctx}
}

func passStreamHandler(_ any, _ grpc.ServerStream) error { return nil }

func TestStreamAuth_MissingMetadata(t *testing.T) {
	_, stream := interceptor.NewAuthInterceptors(okVerifier())

	err := stream(nil, streamWith(context.Background()), &grpc.StreamServerInfo{}, passStreamHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestStreamAuth_InvalidToken(t *testing.T) {
	_, stream := interceptor.NewAuthInterceptors(badVerifier())

	err := stream(nil, streamWith(ctxWithBearer("bad")), &grpc.StreamServerInfo{}, passStreamHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.ErrorContains(t, err, "invalid token")
}

func TestStreamAuth_ValidToken(t *testing.T) {
	_, stream := interceptor.NewAuthInterceptors(okVerifier())

	err := stream(nil, streamWith(ctxWithBearer("good-token")), &grpc.StreamServerInfo{}, passStreamHandler)

	assert.NoError(t, err)
}
