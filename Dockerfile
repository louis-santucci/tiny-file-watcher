# Builder stage
FROM dhi.io/golang/1.26.1-debian13-dev AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY grpc/ ./grpc/
COPY server/ ./server/

RUN go build -o tfws ./server

# Runtime stage
FROM dhi.io/debian-base/trixie-debian13

WORKDIR /app

COPY --from=builder /tfws ./tfws

ENV TFWS_CONFIG_PATH=./.tfw

VOLUME ./.tfw

ENTRYPOINT ["./tfws"]
