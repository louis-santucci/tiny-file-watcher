FROM dhi.io/golang:1.26-debian13-dev AS builder

# Set the working directory
WORKDIR /build

RUN apt update -y && apt install -y wget unzip

RUN wget https://github.com/protocolbuffers/protobuf/releases/download/v21.12/protoc-21.12-linux-x86_64.zip
RUN unzip protoc-21.12-linux-x86_64.zip -d /build/.local
ENV PATH="/build/.local/bin:$PATH"

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY server/ server/
COPY internal/ internal/
COPY grpc/ grpc/
COPY client/ client/

# Build the application
RUN mkdir -p gen/grpc && \
    protoc \
		--proto_path=grpc \
    		--go_out=gen/grpc --go_opt=paths=source_relative \
    		--go-grpc_out=gen/grpc --go-grpc_opt=paths=source_relative \
    		filewatcher.proto
RUN GOOS=linux go build -o tfws /build/server

FROM dhi.io/debian-base:trixie-debian13 AS final

# Set the working directory
WORKDIR /app

# Copy the built application from the builder stage
COPY --from=builder /build/tfws ./tfws

# Command to run the application
CMD ["./tfws"]