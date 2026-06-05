#!/bin/sh
# dia-fake: a minimal example plugin.
#
# A dia plugin is any executable named `dia-<name>` on $PATH. The
# config references it by name; dia looks it up and runs it with
# the args and env from the YAML.
#
# To try it:
#   1. Copy this file somewhere on $PATH as `dia-fake`
#      (or symlink: ln -s examples/plugins/dia-fake.sh ~/bin/dia-fake)
#   2. Add to a workspace:
#         apps:
#           - type: plugin
#             plugin: fake
#             args: ["hello"]
#   3. `dia start <workspace>`

if [ "$1" = "--describe" ]; then
	cat <<'EOF'
name: fake
description: Prints its args. Use as a starting point for new plugins.
args:
  - name: message
    required: false
    description: Text to print. Defaults to "hello, dia".
env:
  - name: DIA_PLUGIN_DRY_RUN
    description: If set, the plugin prints what it would do and exits.
EOF
	exit 0
fi

msg="${1:-hello, dia}"
if [ -n "$DIA_PLUGIN_DRY_RUN" ]; then
	echo "[dry-run] dia-fake: msg=$msg cwd=$(pwd) args=$*"
	exit 0
fi
echo "$msg"
