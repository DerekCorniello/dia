# Compatibility policy

This is the version matrix dia supports and the deprecation promise for
anything third parties (workspaces, plugins) depend on.

## Supported versions

| Component | Supported |
|---|---|
| OS | Linux (x86_64, arm64), macOS 13+ (universal), Windows 10+ (x86_64) |
| Go toolchain | 1.23+ (build with the Go version in `go.mod`; 1.24 verified) |
| Node | 20 (frontend build) |
| Wails | v2.12.0 (CLI and runtime pinned together via `go.mod`) |
| WebKitGTK | 4.1 (`-tags webkit2_41` on Linux) |
| Browsers (managed profiles) | Zen, Firefox/Firefox ESR, LibreWolf, Waterfox, Floorp, Icecat, Chromium, Chrome, Brave, Vivaldi, Edge — Linux paths verified; macOS/Windows discovery implemented, real-process coverage Linux-only |
| Config schema | `version: 1` (newer rejected at load) |
| State schema | `version: 1` (newer rejected at open; legacy `recent` migrated) |
| Daemon protocol | `protocol: 1` negotiated by `hello`; mismatch fails closed |

## Deprecation policy

- Config fields, daemon verbs, plugin host APIs, and manifest fields are
  additive. Removal or rename requires one minor release of dual support
  with a `doctor` warning naming the replacement.
- State migrations are explicit and version-gated; dia never writes a
  newer state schema than it can read back.
- Wails IPC shapes (`WorkspaceInfo`, `AppInfo`, `InstanceInfo`,
  `PluginInfo`, descriptors) are versioned by the `DaemonInfo.protocol`
  field; the GUI degrades to summary views when a detail call is absent.

## Backup and restore

`dia backup --out backup.tar.gz` archives workspace YAMLs and `state.json`
(+ `.bak`). Managed browser seeds are excluded by default because they
contain credentials; pass `--include-seeds` only for a same-machine move,
and treat the archive as secret material. Restore by extracting over the
config and state dirs while the daemon is shut down (`dia shutdown` first).
