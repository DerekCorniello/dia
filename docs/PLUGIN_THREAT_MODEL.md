# Plugin threat model

## Scope

Plugins are user-installed code. Dia treats a plugin as untrusted with respect
to the launcher, other plugins, workspace files, and browser credentials. The
plugin may be useful and intentionally powerful, but installation or enabling
it is an explicit trust decision by the user.

## Assets

- Workspace YAML files, environment values, and hook commands.
- Runtime state, plugin grants, themes, and managed browser profiles.
- The user's network, filesystem, and ability to launch processes.
- The integrity of the dia daemon and other installed plugins.

## Boundaries and guarantees

- Resolver code runs in a restricted goja runtime. It receives only
  `getConfig` and `pluginDir`, and must remain side-effect free.
- Panel code runs in a separate runtime. Host calls are capability checked;
  requesting a capability never grants it.
- Window plugins run in a separate dia process with a read-only default grant.
  Mutating calls fall through to the same capability bridge.
- Plugin file paths are cleaned and checked for containment before reads.
- Plugin window assets are served with a restrictive Content Security Policy.
- Plugin files, fetched response bodies, and daemon messages have size limits.
- State and browser profile directories are private to the owning user.

## User responsibilities and non-goals

Dia cannot make trusted plugin code harmless. A user who grants `exec`,
`fetch`, workspace mutation, or file-writing capabilities is authorizing those
effects. Dia does not provide a VM, syscall sandbox, network anonymity, or
protection from a plugin that abuses an already-granted capability.

Remote navigation, downloads, clipboard access, and persistent web storage are
not part of the supported window-plugin contract. Platform review is required
before enabling any of them.

## Installation and updates

Review a plugin's source and requested capabilities before granting mutating
access. Updates must be treated as new code review events; changing the
manifest or executable code can change the security impact even when the
plugin ID is unchanged.

## Reporting

Report suspected path escapes, capability bypasses, daemon authorization bugs,
or browser-profile disclosure privately to the project maintainers. Include
the dia version and a minimal reproduction, but do not attach state files,
browser profiles, environment dumps, or plugin secrets.
