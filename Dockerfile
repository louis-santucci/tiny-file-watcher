# Builder stage
FROM dhi.io/golang:1.27-alpine AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go install "github.com/bufbuild/buf/cmd/buf@latest"

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
FROM dhi.io/debian-base:trixie

WORKDIR /app

COPY --from=builder /src/tfws ./tfws

ENV TFWS_CONFIG_PATH=/app/.tfw
ENV TZ=Europe/Paris

ENTRYPOINT ["./tfws"]
