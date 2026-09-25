# Runtime build commands. Contributors to this module need Go; users of the
# toolkit do not, and get a prebuilt binary from GitHub Releases instead.
#
# CGO is off everywhere: SQLite is pure Go (modernc.org/sqlite), so no C
# toolchain is needed to build and no SQLite install is needed to run.

BINARY  := vibe-agent
# GOEXE is ".exe" on Windows and empty elsewhere. Without it, install wrote an
# extensionless file that Git Bash can execute and Windows cannot resolve by
# name, so hooks worked from a POSIX shell and nothing else could find the
# binary. The release target already appended the extension per platform; these
# two did not.
EXE     := $(shell go env GOEXE)

# A local build stamps the commit it came from, so `doctor` can tell an install
# apart from the source beside it. It used to be "dev" on both sides, which made
# a version comparison prove nothing: a ten-day-old binary and a fresh one
# reported the same string. A dirty tree is marked, because the commit alone
# would describe a build that no longer matches its own source.
GIT_SHA   := $(shell git rev-parse --short HEAD 2>/dev/null)
GIT_DIRTY := $(shell git diff --quiet 2>/dev/null || echo "-dirty")
VERSION   ?= $(if $(GIT_SHA),dev-$(GIT_SHA)$(GIT_DIRTY),dev)

# One install location, shared with scripts/install-runtime.{sh,ps1}. These used
# to write to three different directories: GOPATH/bin here, ~/.local/bin from the
# shell installer, and %LOCALAPPDATA%\Programsibe-agent from the PowerShell
# one. A machine could then hold several vibe-agent binaries at different
# versions, and PATH order decided which one the hooks called — so `make install`
# could appear to do nothing because a copy installed elsewhere still won.
INSTALL_DIR ?= $(if $(VIBE_INSTALL_DIR),$(VIBE_INSTALL_DIR),$(HOME)/.local/bin)
DIST    := dist
LDFLAGS := -s -w -X main.version=$(VERSION)

# One entry per published artifact. Adding a row here is the only change needed
# to ship another target.
PLATFORMS := \
	windows/amd64 \
	windows/arm64 \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64

export CGO_ENABLED := 0

# golangci-lint is installed on demand into this module's own bin/, never into
# the user's GOPATH, so a contributor's global toolchain is left alone and CI
# and a laptop run the same binary.
TOOLS_DIR    := $(CURDIR)/bin
GOLANGCI     := $(TOOLS_DIR)/golangci-lint$(EXE)
GOLANGCI_REF ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.1
MIGRATE      := $(TOOLS_DIR)/migrate$(EXE)
MIGRATE_REF  ?= github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3
MIGRATIONS_DIR := migrations
# File-backed SQLite URL for the CLI. Override for a different workspace.
# The agent binary applies the same SQL via embed on database.Open; these
# targets are for developers inspecting or forcing a local memory.db.
#
# Scheme must be sqlite:// (not sqlite3://): `go install -tags sqlite` pulls
# modernc and registers that name. A path relative to runtime/ works on
# Windows and Unix; absolute file: URLs are easy to get wrong here.
DATABASE_URL ?= sqlite://../.agent-state/memory.db

.PHONY: all build test bench vet fmt fmt-check lint golangci tools check release clean install help \
	new-migrate migrate-up migrate-down

all: check build

## build: compile for the current platform into dist/
build:
	go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)$(EXE) ./cmd

## install: build and install to INSTALL_DIR, then check nothing else shadows it
install:
	@mkdir -p "$(INSTALL_DIR)"
	go build -ldflags="$(LDFLAGS)" -o "$(INSTALL_DIR)/$(BINARY)$(EXE)" ./cmd
	@echo "installed $(INSTALL_DIR)/$(BINARY)$(EXE) ($(VERSION))"
	@# Hooks resolve the binary by name, so what matters is which copy PATH finds,
	@# not which one was just written. Reporting the difference is the whole point:
	@# a shadowed install is invisible, and the symptom is a hook answering with
	@# behaviour from a build you replaced.
	@resolved=$$(command -v $(BINARY) 2>/dev/null || true); \
	if [ -z "$$resolved" ]; then \
	  echo "warning: $(BINARY) is not on PATH. Hooks invoke it by name, so none run."; \
	  echo "         add it with: export PATH=\"$(INSTALL_DIR):\$$PATH\""; \
	  exit 0; \
	fi; \
	mine=$$("$(INSTALL_DIR)/$(BINARY)$(EXE)" version 2>/dev/null || echo unknown); \
	theirs=$$("$$resolved" version 2>/dev/null || echo unknown); \
	if [ "$$mine" != "$$theirs" ]; then \
	  echo "warning: PATH resolves $(BINARY) to a different build:"; \
	  echo "           $$resolved ($$theirs)"; \
	  echo "         so this install does not change what the hooks run."; \
	  echo "         remove that copy, or put $(INSTALL_DIR) earlier on PATH."; \
	fi

## test: run every package test, including the consumer-repo end-to-end suite
test:
	go test -count=1 ./...

## bench: run Go benchmarks with allocation counts (not part of check)
bench:
	go test -run=^$$ -bench=. -benchmem ./...

## vet: run the Go static checks
vet:
	go vet ./...

## fmt: rewrite files with gofmt
fmt:
	gofmt -w .

## fmt-check: fail when anything needs gofmt, for CI
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

## tools: install the pinned linters and migrate CLI into ./bin
tools: $(GOLANGCI) $(MIGRATE)

$(GOLANGCI):
	GOBIN=$(TOOLS_DIR) go install $(GOLANGCI_REF)

# sqlite tag pulls modernc (no CGO), matching database.Open's driver.
$(MIGRATE):
	GOBIN=$(TOOLS_DIR) go install -tags 'sqlite' $(MIGRATE_REF)

## new-migrate: create a migration pair. usage: make new-migrate name=add_foo
new-migrate: $(MIGRATE)
ifndef name
	$(error usage: make new-migrate name=<migration_name>)
endif
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

## migrate-up: apply all pending SQL under migrations/ to DATABASE_URL
migrate-up: $(MIGRATE)
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

## migrate-down: roll back one migration on DATABASE_URL
migrate-down: $(MIGRATE)
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

## golangci: the configured linter set, see .golangci.yml
golangci: $(GOLANGCI)
	$(GOLANGCI) run ./...

## lint: every static check, no tests
lint: vet fmt-check golangci

## check: everything CI runs for this module
check: lint test

## release: cross-compile every published target and write checksums
release: clean
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		goos=$${platform%/*}; goarch=$${platform#*/}; \
		ext=""; [ "$$goos" = "windows" ] && ext=".exe"; \
		out="$(DIST)/$(BINARY)_$(VERSION)_$${goos}_$${goarch}$$ext"; \
		echo "building $$out"; \
		GOOS=$$goos GOARCH=$$goarch go build -ldflags="$(LDFLAGS)" -o "$$out" ./cmd || exit 1; \
	done
	@cd $(DIST) && sha256sum * > SHA256SUMS && echo "wrote $(DIST)/SHA256SUMS"

## clean: remove build output
clean:
	rm -rf $(DIST) $(TOOLS_DIR)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
