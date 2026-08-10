GO ?= go
BIN := dia
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

.PHONY: all dev build run test vet lint fmt tidy clean install-tools hooks check

all: hooks build

# The webkit2_41 tag has to match `build`: without it wails links
# against webkit2gtk-4.0, which on a 4.1-only system fails at the
# linker with missing libjxl symbols.
dev:
	JSC_SIGNAL_FOR_GC=12 $(WAILS) dev -tags webkit2_41

build:
	$(WAILS) build -clean -trimpath -ldflags "$(LDFLAGS)" -tags webkit2_41
	@echo "built build/bin/$(BIN) -- run it with 'make run' or './build/bin/$(BIN)'"

# run launches the binary make build produced. wails writes to
# build/bin/, not the repo root, which is easy to miss when a stale
# binary is sitting in the root from an older build.
run: build
	./build/bin/$(BIN)

# install builds and copies the binary to GOPATH/bin, making it
# available as a system-wide command. On Linux it also installs the
# .desktop entry and icon so launchers (GNOME, KDE, etc.) find the
# app; macOS and Windows register themselves via the .app bundle and
# the .exe resources, so there is nothing to do there.
install: build
	cp build/bin/$(BIN) $(shell go env GOPATH)/bin/$(BIN)
	@echo "installed to $(shell go env GOPATH)/bin/$(BIN)"
	@if [ "$$(uname -s)" = "Linux" ]; then \
		DATA="$${XDG_DATA_HOME:-$$HOME/.local/share}"; \
		APPS="$$DATA/applications"; \
		ICONS="$$DATA/icons/hicolor/512x512/apps"; \
		mkdir -p "$$APPS" "$$ICONS"; \
		cp build/appicon.png "$$ICONS/$(BIN).png"; \
		sed -e "s|@EXEC@|$(shell go env GOPATH)/bin/$(BIN)|" \
			-e "s|@ICON@|$$ICONS/$(BIN).png|" \
			packaging/$(BIN).desktop > "$$APPS/$(BIN).desktop"; \
		chmod +x "$$APPS/$(BIN).desktop"; \
		update-desktop-database "$$APPS" >/dev/null 2>&1 || true; \
		echo "desktop entry installed to $$APPS/$(BIN).desktop"; \
	fi

test:
	$(GO) test -count=1 -timeout 60s ./...

vet:
	$(GO) vet ./...

lint:
	$(GOLANGCI) run ./...

fmt:
	gofmt -l -w .

tidy:
	$(GO) mod tidy

# check runs the same gates as the pre-commit hook and CI.
check: fmt vet lint test
	@echo "all checks passed"

clean:
	rm -rf build/bin frontend/dist

install-tools:
	go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)

hooks:
	@if [ "$(shell git config core.hooksPath)" != ".githooks" ]; then \
		git config core.hooksPath .githooks; \
		echo "configured git hooks (.githooks/)"; \
	fi
