BINARY := dia
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG := github.com/DerekCorniello/dia
LDFLAGS := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.BuildTime=$(BUILD_TIME)

.PHONY: dev build test vet fmt tidy release clean install-tools

dev:
	wails dev

build:
	wails build -clean -trimpath -ldflags "$(LDFLAGS)"

test:
	go test -count=1 -timeout 60s ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

clean:
	rm -rf build/bin frontend/dist

install-tools:
	go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1

release:
	@echo "release: build the binary per platform first, then run goreleaser with --skip-build"
	@echo "see .goreleaser.yaml for the matrix"
