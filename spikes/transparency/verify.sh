#!/bin/sh
# Capture the desktop, run the S4 transparency spike over it, and compare.
#
# The surface is anchored at the output's top-left, so screen coordinates and
# spike coordinates are the same. Both captures are the *composited* screen,
# which is the only view that can answer whether alpha reaches the desktop.
#
# Usage: verify.sh [transparent|opaque]
set -eu

mode=${1:-transparent}
# Optional second argument: -unpremultiply converts the gg image to straight
# alpha before upload, the workaround for the upload path applying alpha twice.
extra=${2:-}
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
# A separate artifact directory per mode, so runs do not overwrite each other.
# (Avoid `$( [ ... ] && echo )`: a failing test inside a command substitution
# exits the script under `set -e`.)
suffix=""
if [ -n "$extra" ]; then
	suffix="-unpremultiplied"
fi
tmp="$root/.tmp/s4-$mode$suffix"
mkdir -p "$tmp"

case "$mode" in
	transparent) flag=true ;;
	opaque) flag=false ;;
	*) echo "usage: $0 [transparent|opaque] [-unpremultiply]" >&2; exit 2 ;;
esac

echo "== capturing the desktop with no overlay"
grim -t ppm "$tmp/baseline.ppm"

echo "== starting the spike (transparent=$flag)"
# shellcheck disable=SC2086
(cd "$root" && go run ./spikes/transparency -transparent=$flag $extra -duration=12s > "$tmp/spike.log" 2>&1 &)

# Wait for the surface to be mapped: the spike prints its layout once it is up.
for _ in $(seq 1 40); do
	if grep -q "S4-LAYOUT" "$tmp/spike.log" 2>/dev/null; then
		break
	fi
	sleep 0.5
done
sleep 3

if ! grep -q "S4-LAYOUT" "$tmp/spike.log"; then
	echo "spike never presented; log:" >&2
	cat "$tmp/spike.log" >&2
	exit 1
fi

grep -o 'S4-LAYOUT .*' "$tmp/spike.log" | cut -d' ' -f2- > "$tmp/layout.json"

echo "== capturing the screen with the overlay up"
grim -t ppm "$tmp/with.ppm"

if [ "$mode" = transparent ]; then
	python3 "$here/verify.py" "$tmp/baseline.ppm" "$tmp/with.ppm" "$tmp/layout.json" --transparent
else
	python3 "$here/verify.py" "$tmp/baseline.ppm" "$tmp/with.ppm" "$tmp/layout.json"
fi

echo
echo "artifacts in $tmp (baseline.ppm, with.ppm, layout.json, spike.log)"