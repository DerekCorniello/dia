# Changelog

All notable changes to dia are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Phase 0 skeleton: Wails v2 + Svelte + TypeScript + Vite desktop app
- `internal/version` package with build-time Version, Commit, BuildTime vars
- `internal/cli` cobra-based CLI with `--version`/`-V` and `--help`
- `internal/wailsapp` package; main-package `App` facade keeps wails binding
  working while implementation lives in `internal/*`
- `Makefile` with `dev`, `build`, `test`, `vet`, `fmt`, `tidy`, `clean`,
  `install-tools` targets
- `.goreleaser.yaml` for cross-platform release packaging
- GitHub Actions CI: `go test` + `go vet` + `gofmt` on linux, macos, windows
- GitHub Actions release: matrix build via `wails build`, then GoReleaser
  packaging and `softprops/action-gh-release` publish
- `README.md` and updated `PLAN.md`

### Known limitations

- No real subcommands yet (Phase 4); the bare CLI currently shows help for
  any unknown argument and exits 0
- No actual workspaces (Phase 1+)
- Frontend is a placeholder; real UI lands in Phase 5
