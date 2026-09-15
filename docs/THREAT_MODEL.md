# Dia threat model

## Scope

Dia is a single-user desktop launcher. One OS user owns the state directory, the daemon socket/pipe, managed browser seeds, and workspace files. Dia does not defend against a local attacker who already runs as that user; it defends against cross-user, cross-process confusion, and accidental credential disclosure.

## Assets

- Workspace YAML, hooks, env values, and plugin config.
- `state.json` (instances, recent, favorites, themes, roots, plugin grants).
- Managed browser seeds and ephemeral clones under `~/.local/state/dia/browser/` (contain cookies, logins, and profile data).
- Long-lived app processes and browser windows owned by the daemon.
- Daemon socket/pipe (`serve.sock` / named pipe) and the ability to start/stop workspaces.

## Trust boundaries

- **Daemon vs GUI/CLI.** The daemon is the sole supervisor. Clients are `verifyPeer`-checked and must `HELLO` with `ProtocolVersion`. A stale socket file is probed before removal; `serve.lock` is held for the daemon lifetime.
- **Browser profile vs real profile.** The user's real profile (`~/.zen`, `~/.mozilla/firefox`) is read once to seed `browser/seeds/<browser>`. Ephemeral clones in `browser/profiles/` are copy-on-write; writeback to the seed happens only after the owned browser tree is confirmed stopped and under a per-seed lock with PID + 1h stale reclaim.
- **Plugin vs host.** See `PLUGIN_THREAT_MODEL.md`. Resolver runtimes see only `getConfig`/`pluginDir`; window plugins start with read-only grants and go through the capability bridge.

## Guarantees

- State directory `0700`, state file `0600`, socket `0600`, daemon log `0600` with log rotation. Windows pipe uses an owner-only security descriptor. Linux socket verifies peer UID via `SO_PEERCRED`.
- Cross-process `state.lock` with `reloadLocked()` before every `Mutate`, atomic `state.json.tmp` + `fsync` + `rename` + `syncDir`, and `state.json.bak` for recovery. Symlinked state paths are rejected.
- Process identity (`pid + start-time` on Linux via `/proc`, `pid + creation time` on Windows) is persisted at launch and verified before every `Kill`/`IsRunning`/`Reconcile`. Unix teardown signals the process group (`Setsid`, `kill -pid`); Windows uses a `KILL_ON_JOB_CLOSE` Job Object for live launches and identity-checked `taskkill /T` for recovered ones.
- Daemon protocol: `HELLO` handshake, `maxMessageSize=4MiB`, per-request `30s` deadline, server-side `bufio.Reader` buffering preserved, client `sync.Mutex` serializes `Do`, daemon `mu` serializes dispatch with `Reload` before lifecycle verbs.
- Wails payloads: list endpoints have lean summaries + `GetWorkspace`/`GetPlugin` details and `List*Paginated` caps so the frontend can bound IPC. Fuzz targets exist for `daemon/message` and `state` envelopes.

## Non-goals

- No sandbox for `exec` or `fetch` once a capability is granted; a granted plugin is trusted with that effect.
- No network isolation, no multi-user daemon sharing, no protection if the owning UID is already compromised.
- No automatic migration of profile formats across browser major versions; `Refresh` is explicit.

## User responsibilities

- Review `browser` apps: naming a binary copies your real profile into `browser/seeds/<browser>`; treat that directory as secret material.
- Review plugin capabilities before granting mutating ones; treat updates as new reviews.
- Do not share `~/.local/state/dia` or expose the socket/pipe over the network.
- On `pre_start`/`pre_stop` hooks, keep commands idempotent; forced stop skips hooks.

## Reporting

Report private credential or socket bypass issues with `dia doctor` output and `dia --version`, but do not attach `state.json`, `browser/seeds/*`, or env dumps.
