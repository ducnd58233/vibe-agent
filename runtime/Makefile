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

.PHONY: all build test vet fmt fmt-check lint check release clean install help

all: check build

## build: compile for the current platform into dist/
build:
	go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)$(EXE) ./cmd

## install: put the binary on the Go bin path for local use
install:
	go build -ldflags="$(LDFLAGS)" -o "$(shell go env GOPATH)/bin/$(BINARY)$(EXE)" ./cmd

## test: run every package test, including the consumer-repo end-to-end suite
test:
	go test -count=1 ./...

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

## lint: alias for the checks CI enforces
lint: vet fmt-check

## check: everything CI runs for this module
check: fmt-check vet test

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
	rm -rf $(DIST)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
