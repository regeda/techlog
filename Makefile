# Techlog developer tasks. Run `make` to list them.

BIN := bin/techlog
DB ?= share/techlog.db
PORT ?= 8080
CLAUDE_HOME ?= $(HOME)/.claude
SKILL := techlog-record
SKILL_SRC := $(CURDIR)/skills/$(SKILL)
SKILL_DST = $(CLAUDE_HOME)/skills/$(SKILL)
GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint)

.DEFAULT_GOAL := help

.PHONY: help deploy-website build run seed flush reset skill skill-remove test lint fmt check tools clean

help: ## list the available tasks
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-13s %s\n", $$1, $$2}'

build: ## build bin/techlog
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

skill: ## install the Claude skill into ~/.claude/skills (symlink; COPY=1 copies; FORCE=1 replaces; CLAUDE_HOME=dir)
	@mkdir -p "$(CLAUDE_HOME)/skills"
	@if [ -L "$(SKILL_DST)" ] && [ "$$(readlink "$(SKILL_DST)")" = "$(SKILL_SRC)" ] && [ -z "$(COPY)" ]; then \
		echo "already installed: $(SKILL_DST) -> $(SKILL_SRC)"; exit 0; fi; \
	if [ -e "$(SKILL_DST)" ] || [ -L "$(SKILL_DST)" ]; then \
		if [ -z "$(FORCE)" ]; then \
			echo "$(SKILL_DST) already exists:"; ls -ld "$(SKILL_DST)"; echo "use FORCE=1 to replace it"; exit 1; fi; \
		rm -rf "$(SKILL_DST)"; fi; \
	if [ -n "$(COPY)" ]; then cp -R "$(SKILL_SRC)" "$(SKILL_DST)" && echo "copied $(SKILL_SRC) to $(SKILL_DST)"; \
	else ln -s "$(SKILL_SRC)" "$(SKILL_DST)" && echo "linked $(SKILL_DST) -> $(SKILL_SRC)"; fi

skill-remove: ## remove the installed Claude skill (only this skill; FORCE=1 also removes a foreign link)
	@if [ -L "$(SKILL_DST)" ]; then \
		if [ "$$(readlink "$(SKILL_DST)")" = "$(SKILL_SRC)" ] || [ -n "$(FORCE)" ]; then rm "$(SKILL_DST)" && echo "removed link $(SKILL_DST)"; \
		else echo "$(SKILL_DST) links elsewhere ($$(readlink "$(SKILL_DST)")); use FORCE=1 to remove it"; exit 1; fi; \
	elif [ -f "$(SKILL_DST)/SKILL.md" ] && grep -q '^name: $(SKILL)$$' "$(SKILL_DST)/SKILL.md"; then \
		rm -rf "$(SKILL_DST)" && echo "removed copy $(SKILL_DST)"; \
	elif [ -e "$(SKILL_DST)" ]; then echo "$(SKILL_DST) is not the $(SKILL) skill; not removing it"; exit 1; \
	else echo "nothing to remove at $(SKILL_DST)"; fi

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

deploy-website: ## build the website and deploy it to Cloudflare Pages (ARGS="--branch x")
	scripts/deploy-website.sh $(ARGS)

clean: ## remove build output
	rm -rf bin
