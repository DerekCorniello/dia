---
name: dia-workspace-config
description: Write and validate dia workspace YAML configs (the files dia start/GUI use to launch a dev environment). Use when asked to create, edit, scaffold, or debug a dia workspace config, a .dia.yaml, or a file under ~/.config/dia/workspaces/.
---

# Writing dia workspace configs

This skill is for authoring the YAML files dia loads to define a workspace: an
editor, terminal(s), services, and browser tabs that come up together with
`dia start <name>`. It is grounded directly in the validator
(`internal/config/load.go`, `internal/config/types.go`) and the app-type
registry (`internal/registry/registry.go`, `handlers.go`), not just the
README, so treat this as the more precise source when the two disagree —
and re-check those files yourself if dia's version has moved on since this
was written.

## Where a config lives

Two places, and both are picked up automatically (`internal/config/discover.go`):

- **Global**: any `*.yaml`/`*.yml` file directly under
  `$XDG_CONFIG_HOME/dia/workspaces/` (defaults to `~/.config/dia/workspaces/`
  when `$XDG_CONFIG_HOME` is unset). The filename does not need to match the
  workspace's `name` field. One file per workspace is the convention, but
  nothing enforces it.
- **Project-local**: either a single `.dia.yaml` at the repo root (found by
  walking up from the current directory — it does not have to be cwd exactly,
  any ancestor works) **or** any `*.yaml`/`*.yml` files directly inside a
  `.dia/` directory in cwd (this one is *not* walked up — only checked in cwd
  itself).

On a **name collision**, the project-local workspace wins and silently
shadows the global one of the same name. There is no warning when this
happens.

Default choice: **global**, unless the workspace is genuinely tied to one
repo checkout, in which case use project-local so it travels with the repo
(and stays out of other clones' way — global is a single shared namespace
across every project on the machine).

## Minimal shape

```yaml
version: 1
name: my-project
apps:
  - type: editor
    cmd: code .
    cwd: ~/projects/my-project
```

`version` and `name` are the only two required top-level fields; `apps` may
technically be empty in the schema but a workspace with zero apps and zero
plugins is rejected at *runtime* start (`internal/runtime/runtime.go`:
`"runtime: workspace has no apps or plugins"` — this is not a load-time
validation error, so `dia start --dry-run` will not catch it either).

### Top-level fields

| Field | Required | Notes |
|---|---|---|
| `version` | recommended | Integer. Missing/`0` is silently treated as `1`. A value greater than dia's current schema version (`1` as of this writing) is a hard validation error — never write a version number higher than what the target dia build actually supports. |
| `name` | yes | See naming rule below. Used as the workspace's identity for `dia start <name>`, recency tracking, and name-collision shadowing. |
| `description` | no | Free text, shown in the GUI. |
| `apps` | effectively yes | List of app entries, see below. Empty is schema-valid but runtime-rejected if `plugins` is also empty. |
| `plugins` | no | List of `{ id, config? }`. See Plugins section — **not validated at load time at all**. |

**Unknown top-level fields are a hard parse error**, not a warning: the YAML
decoder runs with `KnownFields(true)`. A typo like `dsecription:` fails to
load rather than being silently ignored — which is good news for catching
typos, but means you cannot add ad-hoc metadata fields to a workspace file.

### The name rule — read the actual function, not the error text

The validator's own error message is wrong. It says a bad name `"must match
^[a-z0-9][a-z0-9-]*$"`, implying lowercase-only, digit/letter start, no
underscore. The function that actually runs (`validName` in
`internal/config/load.go`) accepts:

- ASCII letters, **upper or lower case**
- digits
- hyphen and underscore, **anywhere including the first character**

So `My_Workspace-2` is accepted even though the error text (if you trigger a
different failure) would suggest otherwise. When generating a name, the safe
convention to follow is still lowercase-with-hyphens (`my-project`) — it
reads best in the CLI and GUI — but don't assume uppercase or underscores are
rejected if you encounter one in an existing file.

## App types

Every entry in `apps:` needs enough information for the registry
(`internal/registry/registry.go`) to resolve it to either a **launched
process** or an **opened URL**. `type` is optional; if omitted, dia falls
back to treating the entry like `local` (needs `cmd`).

| `type` | Required fields | Behavior |
|---|---|---|
| *(omitted)* | `cmd` | Same as `local`. |
| `local` | `cmd` | Runs `cmd` as a process. `cmd` is split on whitespace into a program name plus leading args, honoring single/double quotes for paths with spaces (`internal/registry/cmd.go`'s `splitProgram`) — this is a small hand-rolled tokenizer, **not a real shell**: no `&&`, `\|`, redirects, globbing, or env-var expansion inside `cmd`. `args` is appended after whatever `cmd` parses to. To run something that genuinely needs shell semantics, invoke a shell explicitly: `cmd: bash`, `args: ["-lc", "npm run dev && echo done"]`. |
| `editor` | `cmd` | Alias for `local`. Renders with an editor icon in the GUI; no behavioral difference. |
| `terminal` | `cmd` | Alias for `local`, terminal icon. |
| `service` | `cmd` | Alias for `local`, service icon. |
| `custom` | `cmd` | Alias for `local`, generic icon. |
| `ai` | `cmd` | Alias for `local`. **Not documented in the README's app-type table** — exists in the registry (`registry.go`) as an icon-only label like the others. |
| `open` | `url` | Opens `url` in the OS's default handler for that scheme. Any scheme is accepted (`mailto:`, `file://`, `ssh://`, custom schemes) — validation only checks that `url` is non-empty, not that it's http(s). |
| `browser` | `url` or `urls` | Opens one or more URLs in a browser. Two modes, see below: without `browser` set, opens a single `url` (must be `http://`/`https://`) in the OS default handler; with `browser` set to a binary name, launches that binary directly with each of `url`/`urls` as its own tab, and the http(s) restriction is lifted. `cmd` is never read by this type — use `local`/`custom` instead if you need lower-level control over the launched process. |
| `gh` | `cmd` | Wraps the `gh` CLI: `cmd` is the subcommand (e.g. `pr`), `args` are appended. Runs `gh <cmd> <args...>`. |
| `gh:pr` | — | Sugar for `gh pr <args...>`. No required fields. |
| `gh:issue` | — | Sugar for `gh issue <args...>`. |
| `gh:checkout` | — | Sugar for `gh checkout <args...>`. |
| `gh:repo-clone` | `url` | Runs `gh repo clone <url> [cwd]`. `cwd` does double duty here: it's the launched process's own working directory (like every other type) *and*, if set, it's also appended as the positional destination argument telling `gh` where to put the clone. |

**Unrecognized `type` values are accepted by the schema validator** (the
`default:` branch in `validateApp` is deliberately lenient — the comment
explains it's so a workspace with several problems reports all of them at
once, not just the first). This means a typo like `type: eidtor` passes
`dia start <name> --dry-run` cleanly and only fails at a real `dia start`
with `unknown app type "eidtor"`. **Always double-check `type` spelling by
eye** — dry-run will not catch it.

If `type`, `cmd`, and `url` are *all* empty on an entry, that's a real
validation error (`"must have type, cmd, or url"`) and dry-run does catch it.

`type: browser` requires at least one of `url`/`urls` unconditionally —
`validateApp`'s `case "browser"` and the runtime resolver (`resolveBrowser`
in `internal/registry/handlers.go`) agree on this now. (They didn't always:
the validator used to also accept a `cmd`-only entry, which passed
`Validate()`/`--dry-run` but then always failed at a real `dia start`
with `type "browser": url is required`, since `resolveBrowser` never
read `cmd`. Fixed by tightening the validator to match the handler it's
supposed to be gatekeeping — mentioned here in case you're looking at
an older dia checkout that still has the mismatch.)

### `type: browser` in "specific binary" mode — multiple tabs, non-http shortcuts

Setting `browser` on a `type: browser` entry switches it from "hand a URL
to the OS's default-handler dispatcher" (`xdg-open` and equivalents,
which only takes one URL and only really wants http(s)) to "launch this
exact binary myself." This is a `type: local` launch under the hood
(`ActionLaunch`, not `ActionOpenURL`), so:

- `browser` is a binary name looked up on `$PATH`, same convention as
  `cmd` elsewhere (e.g. `zen-browser`, `firefox`, `google-chrome`). It is
  **not** validated against an allowlist — a typo here is a `dia start`-time
  "executable not found" error, not a load-time one.
- `url` (singular) and `urls` (list) are merged, in that order, into one
  list of destinations. Each one opens as its own tab in a single launch
  of `browser` — not a new window per URL, and not N separate process
  launches.
- Without `browser` set, `urls` (plural, more than one destination) is a
  validation error: the OS-default-handler path has no way to open more
  than one URL per entry.
- With `browser` set, the http(s)-prefix requirement is lifted. Each
  entry is passed straight to the browser binary's own address-bar
  resolution — the same thing that would happen if you typed it into
  that browser's URL bar by hand. This is what makes internal go-link-style
  shortcuts (e.g. a hostname-only string like `dc/gh/mux/prs` that
  resolves via a hosts-file/DNS-search-domain entry or a keyword search)
  work as a workspace entry, where they'd be rejected as an invalid URL
  under the default OS-handler path.
- The actual CLI flags used to open multiple tabs in one launch differ by
  browser engine, so there's a small compat layer,
  `browserTabArgs` in `internal/registry/browsercompat.go`: Firefox-family
  binaries (`firefox`, `zen`/`zen-browser`, `librewolf`, `waterfox`,
  `floorp`, `icecat`, `firefox-esr`) get each URL wrapped in its own
  `-new-tab <url>` flag, because bare positional URLs after the first are
  otherwise dropped rather than opened as tabs. Everything else
  (Chromium-family browsers, and any binary this layer doesn't recognize)
  gets plain positional URL arguments, which is the documented Chromium
  convention and a reasonable default for the unknown case. If you hit a
  browser that doesn't fit either bucket, add it to `firefoxFamily` in
  that file (or extend the function) rather than fighting it via YAML.
- Because this is a real process launch, dia tracks its PID like any other
  `local` app. For single-instance browsers this is harmless in practice:
  a launch against an already-running instance just forwards the request
  to that instance and exits almost immediately, which dia treats as a
  normal stop rather than a crash — the same pattern `gh:pr`/`gh:issue`
  already rely on.

```yaml
- type: browser
  browser: zen-browser
  new_window: true
  urls:
    - "dc/gh/mux/prs"
    - "dc/gh/mux/issues"
```

`new_window: true` additionally forces those tabs into a brand-new
browser window rather than appending to whatever window the user already
has open — useful when a workspace's tabs are meant to be a self-contained
group the user can close together, not mixed into their general browsing.
Per-family mechanics, in `browserLaunch` (`internal/registry/browsercompat.go`):

- Chromium family / fallback: `--new-window` is prepended once, ahead of
  the plain positional URL list — Chromium documents the same "force a
  new window" semantics, applied to the whole list that follows rather
  than needing to be repeated per URL.
- Firefox family, single URL: a plain `-new-window <url>` direct exec —
  no complications with only one destination.
- Firefox family, multiple URLs: **not** a single `-new-window <a>
  -new-tab <b> -new-tab <c>` call, despite that looking like the obvious
  translation of the single-URL case. Confirmed against a real
  zen-browser instance: bundling those into one remote command is racy
  — the already-running instance can dispatch the `-new-tab` calls to
  whatever window currently holds focus rather than the one
  `-new-window` just created moments earlier in the same command, so
  they land in an unrelated pre-existing window instead of the fresh
  one. The fix is to split it into two remote calls through a shell,
  with a short `sleep` between them so the new window has time to take
  focus before the rest arrive: `sh -c "<bin> -new-window <a>; sleep
  0.6; <bin> -new-tab <b> -new-tab <c>"` (each argument shell-quoted).
  If you're debugging a `browser` app that isn't behaving, check
  `a.Launch.Cmd` — `"sh"` means this path was taken.

### Common fields on every app entry

| Field | Applies to | Notes |
|---|---|---|
| `cmd` | most types | Program name, or a quoted-aware whitespace-tokenized string (not real shell syntax — see the `local` row above). |
| `args` | all | List of extra arguments, appended after whatever `cmd` parses to (and, for `gh:*` sugar types, after the baked-in subcommand). |
| `cwd` | process types | Supports a **leading** `~` (expanded to `$HOME` — only works as a prefix, not mid-path) and `$VAR` / `${VAR}` tokens anywhere in the string (expanded via the *dia process's own environment*, not the app's `env:` map — see below). |
| `env` | process types | `map[string]string`, merged into the *child process's* environment when it launches. Does **not** affect `$VAR` expansion in `cwd`/`args` for this same entry — that expansion already happened using dia's own environment before the child is spawned. |
| `label` | all | Free-text display label; purely cosmetic. |
| `url` | `open`/`browser`/`gh:repo-clone` | See per-type table above for exact requirements. |
| `urls` | `browser` | List of additional destinations, merged after `url`. Requires `browser` to be set — see the "specific binary" mode section below. |
| `browser` | `browser` | Names the browser binary to launch directly instead of going through the OS default handler. See the "specific binary" mode section below. |
| `new_window` | `browser` | Boolean. Forces the tabs into a fresh browser window instead of landing in whatever window the user already has open. No-op unless `browser` is also set — the OS default handler gives no control over window placement. See below. |
| `open` | — | Boolean field exists in the schema (`App.Open`) but is not read by any current handler — leave it unset. |
| `wait` | — | Boolean field exists in the schema (`App.Wait`) but is not read by any current handler — leave it unset. |

## Plugins

```yaml
plugins:
  - id: whiteboard
    config:
      theme: dark
```

`id` must match an *installed* plugin (`dia plugin list`), but **this is
never checked at config-validation or dry-run time** — `config.Validate`
has no logic for the `plugins` field at all. A workspace referencing a
nonexistent or disabled plugin ID loads and validates fine; the failure only
surfaces later, when the workspace actually starts and dia tries to enable
that plugin. If you're generating a workspace that references a plugin,
verify the ID yourself first with `dia plugin list` (or `dia plugin list
--local` for a project-local plugin) rather than trusting config validation
to catch a typo.

`config` is an arbitrary JSON-shaped map passed to the plugin via
`dia.getConfig()`; its shape is defined by that specific plugin's
`config_schema` in its own `plugin.json`, not by anything in the workspace
schema.

## Recipe: writing and validating a config

1. Decide global vs project-local (see above).
2. Write the YAML by hand, or scaffold first with `dia new <name>` (writes to
   the global dir) or `dia new <name> --local` (writes `.dia.yaml` in cwd)
   and edit the result — this guarantees the boilerplate (`version: 1`, a
   valid `name`) is already correct.
3. Validate what you wrote:
   - `dia start <name> --dry-run` resolves and validates the workspace
     without launching anything, and prints the resolved apps. This catches:
     missing required fields per type, a bad `name`, an unsupported
     `version`, YAML syntax errors, and unknown top-level keys.
   - It does **not** catch: an unrecognized `type` value (typo), a
     nonexistent plugin `id`, or a `cwd`/`env` value that will fail only once
     a real shell tries to use it (e.g. a `$VAR` that isn't actually set in
     dia's environment at launch time).
   - `dia list --json` confirms the workspace was discovered at all (useful
     for catching "wrong directory" / name-collision-shadowing surprises).
4. For anything dry-run can't check, read the relevant `type` row above
   carefully instead of guessing, and cross-reference `internal/registry/handlers.go`
   if in doubt — it is short and is the actual source of truth for what each
   type does with `cmd`/`args`/`cwd`/`url`.

## Worked examples

A workspace exercising every built-in type in one file (adapted from
`examples/sample-workspace.yaml` in this repo, which is a working, tested
reference — start there for a known-good baseline):

```yaml
version: 1
name: sample-workspace

apps:
  - type: editor
    cmd: code .
    cwd: ~/projects/sample-workspace

  - type: terminal
    cmd: bash
    args: ["-lc", "npm run dev"]
    cwd: ~/projects/sample-workspace

  - type: service
    cmd: docker
    args: ["compose", "up"]
    cwd: ~/projects/sample-workspace
    env:
      COMPOSE_PROJECT_NAME: sample

  - type: browser
    url: http://localhost:8080

  - type: gh:pr
    args: ["list", "--state", "open"]

plugins:
  - id: whiteboard
```

A minimal single-app config, the kind `dia new` scaffolds:

```yaml
version: 1
name: quick-look
apps:
  - type: browser
    url: https://example.com
```

## If something still won't load

Read the actual error text — `Validate` aggregates every problem into one
`ValidationErrors` value with dotted/bracketed paths like
`workspace.apps[2].cmd`, so a single failed load usually reports everything
wrong at once, not just the first issue. If the error doesn't make sense
against this document, the schema may have moved: re-read
`internal/config/load.go`'s `Validate`/`validateApp` and
`internal/registry/handlers.go` directly rather than trusting this file
blindly forever.
