SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.ONESHELL:

PKG     := github.com/alephao/snapdiff
BIN     := snapdiff
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
TEMPL   ?= $(HOME)/go/bin/templ

PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	linux/amd64 \
	linux/arm64 \
	windows/amd64

.PHONY: all build generate test vet fmt lint release clean acceptance \
	screenshots screenshots-update screenshots-install

all: lint test build

generate:
	@if [ ! -x "$(TEMPL)" ]; then \
		echo "templ not found at $(TEMPL); run: GOBIN=\$$HOME/go/bin go install github.com/a-h/templ/cmd/templ@latest" >&2; \
		exit 1; \
	fi
	$(TEMPL) generate -path ./internal/web/views

build: generate
	go build -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/snapdiff

test:
	go test ./...

acceptance:
	go test -tags acceptance -count=1 ./test/acceptance/...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	@diff_files=$$(gofmt -l . | grep -v '_templ\.go$$' || true); \
	if [ -n "$$diff_files" ]; then \
		echo "gofmt needed:"; echo "$$diff_files"; exit 1; \
	fi
	go vet ./...

clean:
	rm -rf $(BIN) dist/

# Visual-regression screenshot suite (Playwright). Opt-in: requires Node
# + chromium installed in tests-screenshots/. See docs/adr/0017-*.md.
screenshots-install:
	@cd tests-screenshots && pnpm install && pnpm exec playwright install chromium

screenshots:
	@cd tests-screenshots && pnpm exec playwright test

screenshots-update:
	@cd tests-screenshots && pnpm exec playwright test --update-snapshots

# Cross-compile the matrix from a single host (CGO disabled).
release: generate
	mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="dist/$(BIN)-$$os-$$arch$$ext"; \
		echo "==> $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -ldflags '$(LDFLAGS)' -o "$$out" ./cmd/snapdiff || exit 1; \
	done
	@echo "==> dist/"
	@ls -lh dist/
