# Builder stage
FROM golang:tip-alpine3.22 AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go install "google.golang.org/protobuf/cmd/protoc-gen-go@latest" && \\
    go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest" && \\
    go install "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY grpc/ ./grpc/
COPY internal/ ./internal/
COPY server/ ./server/

RUN mkdir grpc/gen && \\
    protoc --go_out=grpc/gen --go-grpc_out=grpc/gen grpc/*.proto

RUN go build -o tfws ./server

# Runtime stage
FROM dhi.io/debian-base:trixie-debian13

WORKDIR /app

COPY --from=builder /tfws ./tfws

ENV TFWS_CONFIG_PATH=./.tfw

VOLUME ./.tfw

ENTRYPOINT ["./tfws"]
