#!/bin/sh
# Installs the emoji plugin into your keyshell plugin directory, rewriting the
# manifest so it points at this checkout.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
dest="${XDG_CONFIG_HOME:-$HOME/.config}/keyshell/plugins/emoji"

mkdir -p "$dest"
sed "s#@PLUGIN@#$here/emoji-plugin.py#" "$here/plugin.json" > "$dest/plugin.json"

printf 'Installed %s\n' "$dest/plugin.json"
printf 'Restart keyshell, then type ,em to open it.\n'