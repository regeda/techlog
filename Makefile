# Techlog developer tasks. Run `make` to list them.

BIN := bin/techlog
GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint)

.DEFAULT_GOAL := help

.PHONY: help build run seed test lint fmt check tools clean

help: ## list the available tasks
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-8s %s\n", $$1, $$2}'

build: ## build bin/techlog
	go build -o $(BIN) ./cmd/techlog

run: build ## build and serve the UI, API and MCP (pass flags with ARGS="--addr :9090")
	$(BIN) serve $(ARGS)

seed: build ## load the Spring Spin example story
	$(BIN) seed $(ARGS)

test: ## run all tests
	go test ./...

lint: ## run golangci-lint
	@test -x "$(GOLANGCI_LINT)" || { echo "golangci-lint not found: run 'make tools'"; exit 1; }
	$(GOLANGCI_LINT) run ./...

fmt: ## format the code
	gofmt -w cmd internal

check: lint test ## lint and test; run before committing

tools: ## install golangci-lint $(GOLANGCI_LINT_VERSION)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

clean: ## remove build output
	rm -rf bin
