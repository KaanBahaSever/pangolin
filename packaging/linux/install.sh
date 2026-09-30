#!/bin/sh
# Installs Pangolin for the current user, without root:
#
#   ./install.sh              install or update
#   ./install.sh --uninstall  remove the program (your vault is left alone)
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
bin="$HOME/.local/bin"
apps="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
icons="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/512x512/apps"

if [ "${1:-}" = "--uninstall" ]; then
  rm -f "$bin/pangolin" "$apps/pangolin.desktop" "$icons/pangolin.png"
  echo "Pangolin removed. Your vault in ~/.config/Pangolin was not touched."
  exit 0
fi

mkdir -p "$bin" "$apps" "$icons"
install -m 755 "$here/pangolin" "$bin/pangolin"
install -m 644 "$here/pangolin.png" "$icons/pangolin.png"
# The menu entry points at the installed binary, so it works even when
# ~/.local/bin is not on PATH.
sed "s|^Exec=.*|Exec=$bin/pangolin|" "$here/pangolin.desktop" > "$apps/pangolin.desktop"
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$apps" >/dev/null 2>&1 || true

echo "Pangolin installed. Start it from the application menu, or run: $bin/pangolin"
