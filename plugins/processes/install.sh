#!/bin/sh
# Installs the process plugin into your keyshell plugin directory, rewriting the
# manifest so it points at this checkout.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
dest="${XDG_CONFIG_HOME:-$HOME/.config}/keyshell/plugins/processes"

mkdir -p "$dest"
sed "s#@PLUGIN@#$here/process-plugin.py#" "$here/plugin.json" > "$dest/plugin.json"

printf 'Installed %s\n' "$dest/plugin.json"
printf 'Restart keyshell, then type ,ps to open it.\n'