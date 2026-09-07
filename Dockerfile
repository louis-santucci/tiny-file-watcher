# Builder stage
FROM golang:tip-alpine3.22 AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN apk update && rm -rf /var/cache/apk/*

RUN go install "github.com/bufbuild/buf/cmd@latest"

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY buf.yaml buf.yaml
COPY buf.gen.yaml buf.gen.yaml

COPY grpc/ ./grpc/

RUN mkdir -p gen/grpc && buf dep update && buf generate

COPY internal/ ./internal/
COPY server/ ./server/

RUN go build -o tfws ./server

# Runtime stage
FROM alpine:3.23.3

# add a non-root user to run the application
RUN adduser -D -u 1000 tfw

USER tfw

WORKDIR /app

RUN mkdir -p /app/.tfw

COPY --from=builder /src/tfws ./tfws

ENV TFWS_CONFIG_PATH=/app/.tfw

ENTRYPOINT ["./tfws"]
