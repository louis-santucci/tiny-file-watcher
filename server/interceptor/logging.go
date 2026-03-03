package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryLoggingInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	code := codes.OK
	if err != nil {
		code = status.Code(err)
	}

	slog.Info("gRPC request completed",
		slog.String("method", info.FullMethod),
		slog.String("code", code.String()),
		slog.Duration("duration", time.Since(start)))

	if err != nil {
		slog.Error("gRPC request failed", slog.String("method", info.FullMethod), slog.String("error", err.Error()))
	}

	return resp, err

}
