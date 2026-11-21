# mergediff — development, build and test automation.
# GNU Make with POSIX shell commands (Linux, macOS, Windows via Git Bash).

COMMIT       := $(shell git rev-parse --short=7 HEAD 2>/dev/null || echo unknown)
DIRTY        := $(shell test -z "$$(git status --porcelain 2>/dev/null)" || echo -dirty)
DATE         := $(shell date -u +%Y-%m-%d)
VERSION_DATE ?= $(DATE)
LDFLAGS      := -X main.versionDate=$(VERSION_DATE) -X main.commit=$(COMMIT)$(DIRTY)
TAG          := v$(DATE)

WAILS3 ?= wails3
# Files `make dev` opens (wails3 dev starts the app without arguments).
# Override: make dev ARGS="a.txt b.txt"  or  ARGS="base local remote -o out"
ARGS ?= testdata/dev/local.go testdata/dev/remote.go

.DEFAULT_GOAL := help
.PHONY: help deps dev build test test-frontend vet typecheck check e2e version clean

help: ## List targets
	@echo "Usage: make <target>"
	@echo
	@awk 'BEGIN { FS = ":.*## " } /^[a-zA-Z0-9_-]+:.*## / { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

deps: ## Download Go modules, install frontend packages, check the Wails setup
	go mod download
	cd frontend && npm ci
	$(WAILS3) doctor

dev: ## Run with hot reload on sample files (override with ARGS="...")
	MERGEDIFF_DEV_ARGS="$(ARGS)" $(WAILS3) dev

build: ## Build the production binary into bin/ with the version embedded
	$(WAILS3) build EXTRA_LDFLAGS="$(LDFLAGS)"

test: ## Run Go tests
	go test ./...

vet: ## Run go vet and fail if any Go file is not gofmt-formatted
	go vet ./...
	@unformatted="$$(find . -name '*.go' -not -path './frontend/node_modules/*' -not -path './.git/*' -exec gofmt -l {} +)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

typecheck: ## Type-check the frontend
	cd frontend && npx tsc --noEmit

test-frontend: ## Run frontend logic tests (node --test)
	cd frontend && node --test src/*.test.ts

check: vet typecheck test test-frontend ## Run vet, gofmt check, type-check, Go and frontend tests

e2e: build ## Drive the real window end to end (Linux/X11 only; see scripts/e2e/README.md)
	scripts/e2e/run.sh

version: ## Print the version string a build would embed
	@echo "$(VERSION_DATE) ($(COMMIT)$(DIRTY))"

clean: ## Remove build output (bin/, built frontend files)
	rm -rf bin
	find frontend/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +
