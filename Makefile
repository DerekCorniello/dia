BINARY := dia
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG := github.com/DerekCorniello/dia
WAILS := $(shell go env GOPATH)/bin/wails
GOLANGCI := $(shell go env GOPATH)/bin/golangci-lint
GOLANGCI_VERSION := v1.64.8
# Derived from go.mod rather than pinned separately: a CLI older than
# the runtime in go.mod builds against a different Wails than the one
# compiled in, and the only warning is a line the build prints and
# nobody reads. These drifted to v2.10.1 vs v2.12.0 once already.
WAILS_VERSION := $(shell go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2 2>/dev/null)
LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildTime=$(BUILD_TIME)

.PHONY: dev build run test vet lint fmt tidy release clean install-tools install-hooks check

# The webkit2_41 tag has to match `build`: without it wails links
# against webkit2gtk-4.0, which on a 4.1-only system fails at the
# linker with missing libjxl symbols.
dev:
	$(WAILS) dev -tags webkit2_41

build:
	$(WAILS) build -clean -trimpath -ldflags "$(LDFLAGS)" -tags webkit2_41
	@echo "built build/bin/$(BINARY) -- run it with 'make run' or './build/bin/$(BINARY)'"

# run launches the binary make build produced. wails writes to
# build/bin/, not the repo root, which is easy to miss when a stale
# binary is sitting in the root from an older build.
run: build
	./build/bin/$(BINARY)

test:
	go test -count=1 -timeout 60s ./...

vet:
	go vet ./...

lint:
	$(GOLANGCI) run ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

# check runs the same gates as the pre-commit hook and CI.
check: fmt vet lint test

clean:
	rm -rf build/bin frontend/dist

install-tools:
	go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)

# install-hooks symlinks the version-controlled pre-commit hook into
# the local .git/hooks dir. Run once after cloning.
install-hooks:
	ln -sf ../../scripts/hooks/pre-commit .git/hooks/pre-commit
	@echo "pre-commit hook installed"

release:
	@echo "release: push a tag matching v*, e.g. git tag v0.3.0 && git push origin v0.3.0"
	@echo "see .github/workflows/release.yml for the build/archive/publish steps"
