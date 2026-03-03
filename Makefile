BINARY      := tfw
SERVER_PKG  := ./server
PROTO_DIR   := grpc
GEN_DIR     := gen/grpc
PROTO_FILE  := $(PROTO_DIR)/filewatcher.proto
GOPATH      := $(shell go env GOPATH)
INSTALL_DIR := $(GOPATH)/bin

# Ensure GOPATH/bin is on PATH so protoc plugins and golangci-lint are found.
export PATH := $(INSTALL_DIR):$(PATH)

PROTOC_GEN_GO      := $(INSTALL_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(INSTALL_DIR)/protoc-gen-go-grpc
GOLANGCI_LINT      := $(INSTALL_DIR)/golangci-lint

.PHONY: all install-tools generate build install test lint clean

all: generate build

## install-tools: install protoc plugins and golangci-lint
install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

## generate: regenerate Go code from .proto file
generate: $(PROTO_FILE) | $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	mkdir -p $(GEN_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

## build: compile the server binary
build: generate
	go build -o $(BINARY) $(SERVER_PKG)

## install: build and copy binary to GOPATH/bin
install: build
	install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)

## test: run all tests
test: generate
	go test ./...

## lint: run golangci-lint
lint: generate | $(GOLANGCI_LINT)
	golangci-lint run ./...

## clean: remove built binary and generated proto files
clean:
	rm -f $(BINARY)
	rm -f $(GEN_DIR)/*.pb.go $(GEN_DIR)/*_grpc.pb.go

$(PROTOC_GEN_GO):
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

$(PROTOC_GEN_GO_GRPC):
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
