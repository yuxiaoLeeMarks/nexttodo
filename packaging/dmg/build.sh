#!/bin/sh
# Builds the disk image of an app built by `mygo build -skip-dmg`:
#   packaging/dmg/build.sh build/darwin-universal/NextTodo.app "build/NextTodo 0.6.0.dmg"
# It installs dmgbuild in a virtual environment of its own, once.
set -eu
app=$1
out=$2
here=$(cd "$(dirname "$0")" && pwd)
venv=${DMGBUILD_VENV:-${TMPDIR:-/tmp}/nexttodo-dmgbuild}
if [ ! -x "$venv/bin/dmgbuild" ]; then
	python3 -m venv "$venv"
	"$venv/bin/pip" install --quiet dmgbuild==1.6.5
fi
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
# One background for both kinds of display.
tiffutil -cathidpicheck "$here/background.png" "$here/background@2x.png" -out "$work/background.tiff" >/dev/null 2>&1
rm -f "$out"
"$venv/bin/dmgbuild" -s "$here/settings.py" -D app="$app" -D background="$work/background.tiff" NextTodo "$out"
echo "$out"
