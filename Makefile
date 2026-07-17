BINARY := dia
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG := github.com/DerekCorniello/dia
WAILS := $(shell go env GOPATH)/bin/wails
GOLANGCI := $(shell go env GOPATH)/bin/golangci-lint
GOLANGCI_VERSION := v1.64.8
LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildTime=$(BUILD_TIME)

.PHONY: dev build test vet lint fmt tidy release clean install-tools install-hooks check

dev:
	$(WAILS) dev

build:
	$(WAILS) build -clean -trimpath -ldflags "$(LDFLAGS)" -tags webkit2_41

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
	go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)

# install-hooks symlinks the version-controlled pre-commit hook into
# the local .git/hooks dir. Run once after cloning.
install-hooks:
	ln -sf ../../scripts/hooks/pre-commit .git/hooks/pre-commit
	@echo "pre-commit hook installed"

release:
	@echo "release: push a tag matching v*, e.g. git tag v0.3.0 && git push origin v0.3.0"
	@echo "see .github/workflows/release.yml for the build/archive/publish steps"
