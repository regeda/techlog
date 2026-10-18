# Techlog developer tasks. Run `make` to list them.

BIN := bin/techlog
DB ?= share/techlog.db
PORT ?= 8080
PLUGIN := techlog
SCOPE ?= user
BIN_DIR := bin
GOLANGCI_LINT_VERSION := v2.15.0
GOLANGCI_LINT ?= $(BIN_DIR)/golangci-lint

.DEFAULT_GOAL := help

.PHONY: help deploy-website build run seed flush reset plugin plugin-remove test lint fmt check tools clean

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(GOLANGCI_LINT): $(BIN_DIR)
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(BIN_DIR) $(GOLANGCI_LINT_VERSION)

help: ## list the available tasks
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-13s %s\n", $$1, $$2}'

build: $(BIN_DIR) ## build bin/techlog
	go build -o $(BIN) ./cmd/techlog

run: build ## build and serve the UI, API and MCP (pass flags with ARGS="--addr :9090")
	$(BIN) serve --db $(DB) $(ARGS)

seed: build ## load the Spring Spin example story
	$(BIN) seed --db $(DB) $(ARGS)

flush: ## delete the database and every event in it (asks first; FORCE=1 skips; DB=path)
	@test -n "$(DB)" || { echo "DB is empty; refusing to flush"; exit 1; }
	@if curl -sf -o /dev/null --max-time 1 http://localhost:$(PORT)/api/v1/systems; then \
		echo "a server is answering on localhost:$(PORT); stop it before flushing"; exit 1; fi
	@echo "This deletes $(DB), $(DB)-wal and $(DB)-shm."
	@if [ -z "$(FORCE)" ]; then \
		printf "Delete these and every event in them? [y/N] "; read answer; \
		case "$$answer" in y|Y) ;; *) echo "aborted"; exit 1;; esac; fi
	rm -f $(DB) $(DB)-wal $(DB)-shm

reset: ## flush the database, then load the example story (same options as flush)
	@$(MAKE) --no-print-directory flush
	@$(MAKE) --no-print-directory seed

plugin: ## install the techlog plugin from this checkout (SCOPE=user|project|local; restart Claude Code to load it)
	@claude plugin marketplace list | grep -q '^  ❯ $(PLUGIN)$$' || claude plugin marketplace add --scope $(SCOPE) "$(CURDIR)"
	claude plugin install --scope $(SCOPE) $(PLUGIN)@$(PLUGIN)

plugin-remove: ## uninstall the techlog plugin and drop its marketplace (SCOPE=user|project|local)
	-claude plugin uninstall --scope $(SCOPE) $(PLUGIN)@$(PLUGIN)
	-claude plugin marketplace remove $(PLUGIN)

test: ## run all tests
	go test ./...

lint: $(GOLANGCI_LINT) ## run golangci-lint
	@test -x "$(GOLANGCI_LINT)" || { echo "golangci-lint not found: run 'make tools'"; exit 1; }
	$(GOLANGCI_LINT) run --fix ./...

fmt: ## format the code
	gofmt -w cmd internal

check: lint test ## lint and test; run before committing

tools: $(GOLANGCI_LINT) ## install golangci-lint $(GOLANGCI_LINT_VERSION) into $(BIN_DIR)

deploy-website: ## build the website and deploy it to Cloudflare Pages (ARGS="--branch x")
	scripts/deploy-website.sh $(ARGS)

clean: ## remove build output
	rm -rf $(BIN_DIR)
