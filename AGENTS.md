# AGENTS.md

Guidance for AI agents and contributors working in this repository.

## What this is

dia is a cross-platform desktop launcher for dev workspaces, built with
Wails (Go backend + Svelte/TypeScript frontend) and a parallel scriptable
CLI. See `README.md` for the feature surface and `CHANGELOG.md` for history.

## Layout

```
main.go            Wails entrypoint; routes GUI vs CLI vs plugin-window
internal/config    YAML parsing, validation, discovery
internal/runtime   instance lifecycle, PID tracking, reconcile
internal/platform  OS-specific process launching (build-tagged files)
internal/registry  app-type registry and built-ins
internal/plugins   JS plugin host (goja + capability bridge)
internal/state     XDG paths, JSON state store
internal/diag      shared smoke checks (doctor)
internal/cli       cobra commands
internal/wailsapp  bindings exposed to the Svelte UI
frontend/          Svelte + TypeScript + Vite + Tailwind
examples/          sample workspaces and plugins
tools/             developer tools run via `go run` (not shipped)
```

## Build, test, lint

Run all local gates at once:

```sh
make check          # fmt + vet + lint + test
```

Individually:

```sh
make fmt            # gofmt -l -w .
make vet            # go vet ./...
make lint           # golangci-lint run ./...  (config in .golangci.yml)
make test           # go test -count=1 -timeout 60s ./...
make build          # wails build (needs the Wails CLI + GTK/WebKit on Linux)
```

Frontend gates (run from `frontend/`):

```sh
npm run check       # svelte-check
npm test            # vitest
npm run build       # vite build
```

First-time setup:

```sh
make install-tools  # Wails CLI + golangci-lint
make install-hooks  # symlink the version-controlled pre-commit hook
```

CI mirrors these gates on Linux, macOS, and Windows; keep them green.

## Conventions

- No comments unless they explain *why*, not *what*.
- ASCII only in code, docs, and commit messages. No emojis or em-dashes.
- Hand-rolled validation with precise error messages; prefer the stdlib
  over a new dependency.
- Tests for non-trivial logic. Leave code better than you found it.
- Prefer explicit error handling over silent fallbacks.
- No backwards-compatibility shims unless explicitly required.
- Do not commit compiled binaries; `dia` and `gen-completions` are
  gitignored build artifacts.

## Plugin system notes

The plugin host runs untrusted JS in goja under a capability model
(`internal/plugins`). Read-only capabilities are granted by default;
mutating ones are opt-in. When touching the bridge or runtime, preserve
the sandbox: `require()` is plugin-scoped (relative paths only, no `..`),
files are size-capped, and calls are time-bounded. See `README.md` for
the host API and capability table.
