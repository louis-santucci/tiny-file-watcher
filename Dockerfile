# Builder stage
FROM golang:tip-alpine3.22 AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN apk update && \
    apk add --no-cache make protobuf-dev \
    && rm -rf /var/cache/apk/*

RUN go install "google.golang.org/protobuf/cmd/protoc-gen-go@latest" && \
    go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest" && \
    go install "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY grpc/ ./grpc/
COPY internal/ ./internal/
COPY server/ ./server/

RUN mkdir -p ./gen/grpc && \
	protoc \
		--proto_path=grpc/ \
		--go_out=gen/grpc --go_opt=paths=source_relative \
		--go-grpc_out=gen/grpc --go-grpc_opt=paths=source_relative \
		grpc/filewatcher.proto

RUN go build -o tfws ./server

# Runtime stage
FROM alpine:3.23.3

USER tfw

WORKDIR /app

RUN mkdir -p /app/.tfw

COPY --from=builder /src/tfws ./tfws

ENV TFWS_CONFIG_PATH=./.tfw

ENTRYPOINT ["./tfws"]
