.PHONY: build-api build-admin build run-api run-admin cli test test-simple test-integration lint clean

# 供 test-simple 等使用 pipefail，避免管道中仅按 grep 退出码误判成功。
SHELL := /bin/bash

BUILD_DIR := bin

build-api:
	go build -o $(BUILD_DIR)/api ./cmd/api

build-admin:
	go build -o $(BUILD_DIR)/admin ./cmd/admin

build-cli:
	go build -o $(BUILD_DIR)/cli ./cmd/cli

build: build-api build-admin build-cli

run-api:
	go run ./cmd/api

run-admin:
	go run ./cmd/admin

cli:
	go run ./cmd/cli $(ARGS)

test:
	go test ./... -v -count=1

# 使用 pipefail，避免 go test 失败时仍因 grep 退出 0 导致 make 误报成功。
test-simple:
	@set -o pipefail && go test ./... 2>&1 | grep -v "no test files"

test-integration:
	go test ./test/integration/... -v -count=1 -tags=integration

lint:
	golangci-lint run ./...

clean:
	find $(BUILD_DIR) -mindepth 1 -not -name ".gitignore" -delete 
