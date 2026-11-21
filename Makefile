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
.PHONY: help deps dev build test test-backend test-frontend vet typecheck check e2e version clean \
	release release-dry-run release-preflight
REPO_URL := https://github.com/hadi-susanto/mergediff

help: ## List targets
	@echo "Usage: make <target>"
	@echo
	@awk 'BEGIN { FS = ":.*## " } /^[a-zA-Z0-9_-]+:.*## / { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

deps: ## Download Go modules, install frontend packages, check the Wails setup
	go mod download
	cd frontend && npm ci
	$(WAILS3) doctor

dev: ## Run with hot reload on sample files (override with ARGS="...")
	MERGEDIFF_DEV_ARGS="$(ARGS)" $(WAILS3) dev

build: ## Build the production binary into bin/ (UNIVERSAL=1: macOS arm64+amd64)
ifeq ($(UNIVERSAL),1)
	$(WAILS3) task darwin:build:universal EXTRA_LDFLAGS="$(LDFLAGS)"
else
	$(WAILS3) build EXTRA_LDFLAGS="$(LDFLAGS)"
endif
	@# The binary embeds frontend/dist; without index.html the window is blank.
	@test -f frontend/dist/index.html || { \
		echo "build: frontend/dist/index.html is missing, so the binary has no UI."; \
		echo "       Run 'make clean' (it also resets Wails' task cache in .task/) and build again."; \
		exit 1; }

test: ## Full test from scratch: clean, then check (uncached Go tests), then e2e
	$(MAKE) clean
	$(MAKE) check GOTESTFLAGS=-count=1
	$(MAKE) e2e

test-backend: ## Run Go tests (GOTESTFLAGS=-count=1 to skip the cache)
	go test $(GOTESTFLAGS) ./...

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

check: vet typecheck test-backend test-frontend ## Run vet, gofmt check, type-check, Go and frontend tests

e2e: build ## Drive the real window end to end (Linux/X11 only; see scripts/e2e/README.md)
	scripts/e2e/run.sh

version: ## Print the version string a build would embed
	@echo "$(VERSION_DATE) ($(COMMIT)$(DIRTY))"

clean: ## Remove build output (bin/, built frontend files, Wails' task cache)
	rm -rf bin .task
	find frontend/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +

# Steps 1-5 of a release; stops at the first failure with a clear message.
release-preflight:
	@set -e; \
	fail() { echo "release: $$*" >&2; exit 1; }; \
	branch="$$(git symbolic-ref --quiet --short HEAD 2>/dev/null || echo '(detached HEAD)')"; \
	[ "$$branch" = main ] || fail "releases are made from main, but the current branch is $$branch"; \
	echo "ok: on branch main"; \
	status="$$(git status --porcelain)"; \
	if [ -n "$$status" ]; then \
		echo "release: the working tree is not clean (commit, stash or remove these files):" >&2; \
		echo "$$status" | sed 's/^/  /' >&2; exit 1; \
	fi; \
	echo "ok: working tree clean"; \
	git fetch origin --tags || fail "git fetch origin --tags failed"; \
	local_main="$$(git rev-parse main)"; \
	remote_main="$$(git rev-parse --verify --quiet origin/main)" || fail "origin/main does not exist; push main first"; \
	[ "$$local_main" = "$$remote_main" ] || fail "local main ($$(git rev-parse --short=7 main)) differs from origin/main ($$(git rev-parse --short=7 origin/main)); push or pull first"; \
	echo "ok: main equals origin/main"; \
	if git rev-parse --verify --quiet "refs/tags/$(TAG)" >/dev/null; then \
		fail "tag $(TAG) already exists locally (one release per day; existing tags are never moved)"; \
	fi; \
	rc=0; git ls-remote --exit-code --tags origin "refs/tags/$(TAG)" >/dev/null || rc=$$?; \
	case $$rc in \
		0) fail "tag $(TAG) already exists on origin (one release per day; existing tags are never moved)";; \
		2) ;; \
		*) fail "could not list tags on origin (git ls-remote exit $$rc)";; \
	esac; \
	echo "ok: tag $(TAG) is free"
	$(MAKE) check
	@echo "ok: make check passed"

release-dry-run: release-preflight ## Run the release checks and show what would be tagged, without tagging
	@echo "Would tag $(TAG) at $(COMMIT) ($$(git log -1 --format=%s HEAD)) and push it to origin."

release: release-preflight ## Tag today's release (vYYYY-MM-DD) and push it; YES=1 skips the prompt
	@echo "Tag:    $(TAG)"
	@echo "Commit: $(COMMIT) $$(git log -1 --format=%s HEAD)"
	@if [ "$(YES)" != 1 ]; then \
		printf 'Create and push $(TAG)? [y/N] '; read answer; \
		case "$$answer" in y|Y|yes|YES) ;; *) echo "release: cancelled" >&2; exit 1;; esac; \
	fi
	git -c tag.gpgsign=false tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	@echo "Release workflow started: $(REPO_URL)/actions"
