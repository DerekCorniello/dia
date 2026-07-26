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
make run            # build, then launch ./build/bin/dia
```

Frontend gates (run from `frontend/`):

```sh
npm run check        # svelte-check (types)
npm run lint         # eslint (config in eslint.config.js)
npm run format:check # prettier, verify only
npm run format       # prettier, rewrite in place
npm test             # vitest
npm run build        # vite build
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

Plugins can also claim workspace app types (`app_types` in the
manifest, `resolveApp` in the entry, gated by the mutating
`apps:resolve` capability). The resolver runs in a *separate,
restricted* goja runtime built by `NewResolverRuntime`: the only
`dia.*` calls it sees are `getConfig` and `pluginDir`. Keep it that
way. Purity is what lets the CLI resolve app types without a
`HostAPI`, keeps `--dry-run` side-effect free, and stops a resolver
from doing I/O in the middle of a workspace start. Resolvers are
cached in `Manager.resolvers`, separate from panel runtimes, and must
be dropped whenever a plugin's code or grants change.

`internal/registry` must not import `internal/plugins`. The adapter in
`registry/plugin.go` states what it needs as a local `AppResolver`
interface over plain maps; the dependency points one way.

`pluginwindow.go` has a second path-containment check, `underDir`,
that predates and duplicates `ContainedRelPath`. Both are correct;
keep them in sync if either is modified.

`registry.SyncPluginTypes` *reconciles* rather than only adding: it
unregisters plugin-claimed types that are no longer claimed. Grants can
change at runtime (Settings > Plugins), and revoking `apps:resolve` has
to make the type stop resolving immediately, not at the next restart.
Call it after anything that changes grants.

Capabilities are never granted by being requested. `MergeCapabilities`
unions and is the wrong tool for deriving grants; use
`GrantCapabilities(manifest.Capabilities, approved)`, which intersects.
Discovery, `Enable`, and install all default to the read-only set, and
only an explicit user action (`dia plugin enable --caps`, or
`App.SetPluginCapabilities`) hands out a mutating one.

Any plugin-supplied path must go through `plugins.ContainedRelPath`.
Validate the *cleaned* path, never a prefix of the raw string:
`panel/../../../etc/passwd` has no leading `..` and will escape. This
has bitten both `require()` and `ui.entry`; the rule lives in one place
so it only has to be right once. Code that reads a plugin path and
hands the bytes somewhere the plugin can observe them (the plugin
window asset server, for one) should re-check containment itself.

## Window plugins and capabilities

A window plugin (`ui.type=window`) runs in a separate dia process
that starts with the read-only capability subset. Mutating calls
(`exec`, `fetch`, `startWorkspace`) fall through from the
`pluginWindowHost.dispatch` to the goja runtime bridge, which
enforces capability gating. The host dispatch explicitly rejects
`exec` and `fetch` so they go through the bridge; do not handle them
directly in the host dispatch or the capability model is bypassed.

## $EDITOR / $VISUAL splitting

Both `cmd_edit.go` and `detect.go` read `$EDITOR`/`$VISUAL` from the
environment. These can contain arguments (e.g. `code --wait`), so
split the raw value with `strings.Fields` before passing to
`exec.Command`. `filepath.Base` is only correct for `LookPath`.

## UI zoom

`App.svelte` uses CSS `transform: scale()` instead of the
non-standard `zoom` property for cross-engine compatibility.
