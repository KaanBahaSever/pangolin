#!/usr/bin/env bash
# Builds the release packages for the platform it runs on and puts them in
# dist/. Used by .github/workflows/release.yml; also works locally.
#
#   packaging/build.sh 1.2.3
#
# Windows (Git Bash)  dist/pangolin-windows-amd64.zip
# macOS               dist/pangolin-macos-universal.dmg   (Intel + Apple Silicon)
# Linux               dist/pangolin-linux-<arch>.deb and .tar.gz
set -euo pipefail

version="${1:?usage: packaging/build.sh <version>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
dist="$root/dist"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

cd "$root"
rm -rf "$dist"
mkdir -p "$dist"

export CGO_ENABLED=1
# The bundled SQLite amalgamation is noisy under newer compilers.
export CGO_CFLAGS="${CGO_CFLAGS:--w}"
ldflags="-s -w -X github.com/KaanBahaSever/pangolin/internal/version.Version=$version"

build_windows() {
  local stage="$work/Pangolin"
  mkdir -p "$stage"
  go build -trimpath -ldflags "$ldflags -H=windowsgui" -o "$stage/Pangolin.exe" ./cmd/pangolin
  cp LICENSE README.md "$stage/"
  local zip
  zip="$(cygpath -w "$dist/pangolin-windows-amd64.zip")"
  if command -v 7z >/dev/null 2>&1; then
    (cd "$work" && 7z a -tzip "$zip" Pangolin >/dev/null)
  else
    powershell.exe -NoProfile -Command \
      "Compress-Archive -Path '$(cygpath -w "$stage")' -DestinationPath '$zip'"
  fi
}

build_macos() {
  local app="$work/dmg/Pangolin.app"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"

  # One binary per architecture, then merged into a universal one.
  local min="-mmacosx-version-min=11.0"
  GOOS=darwin GOARCH=arm64 CC="clang -arch arm64" CGO_CFLAGS="$CGO_CFLAGS $min" CGO_LDFLAGS="$min" \
    go build -trimpath -ldflags "$ldflags" -o "$work/pangolin-arm64" ./cmd/pangolin
  GOOS=darwin GOARCH=amd64 CC="clang -arch x86_64" CGO_CFLAGS="$CGO_CFLAGS $min" CGO_LDFLAGS="$min" \
    go build -trimpath -ldflags "$ldflags" -o "$work/pangolin-amd64" ./cmd/pangolin
  lipo -create -output "$app/Contents/MacOS/pangolin" "$work/pangolin-arm64" "$work/pangolin-amd64"

  cp assets/icon.icns "$app/Contents/Resources/icon.icns"
  sed "s/__VERSION__/$version/g" packaging/macos/Info.plist > "$app/Contents/Info.plist"

  # Ad-hoc signature: required for the binary to run at all on Apple Silicon.
  # It is not a Developer ID signature, so Gatekeeper still asks on first open.
  codesign --force --deep --sign - "$app"

  ln -s /Applications "$work/dmg/Applications"
  # hdiutil occasionally reports "resource busy" on CI runners; retry.
  local try
  for try in 1 2 3 4 5; do
    if hdiutil create -volname Pangolin -srcfolder "$work/dmg" -ov -format UDZO \
        "$dist/pangolin-macos-universal.dmg" >/dev/null; then
      return
    fi
    sleep 3
  done
  echo "hdiutil failed" >&2
  exit 1
}

build_linux() {
  local arch
  arch="$(dpkg --print-architecture)"
  go build -trimpath -ldflags "$ldflags" -o "$work/pangolin" ./cmd/pangolin

  # Tarball with a per-user installer.
  local stage="$work/pangolin-$version"
  mkdir -p "$stage"
  cp "$work/pangolin" LICENSE README.md packaging/linux/pangolin.desktop packaging/linux/install.sh "$stage/"
  cp assets/icon.png "$stage/pangolin.png"
  chmod +x "$stage/install.sh"
  tar -czf "$dist/pangolin-linux-$arch.tar.gz" -C "$work" "pangolin-$version"

  # Debian package.
  local deb="$work/deb"
  install -Dm755 "$work/pangolin" "$deb/usr/bin/pangolin"
  install -Dm644 packaging/linux/pangolin.desktop "$deb/usr/share/applications/pangolin.desktop"
  install -Dm644 assets/icon.png "$deb/usr/share/icons/hicolor/512x512/apps/pangolin.png"
  install -Dm644 LICENSE "$deb/usr/share/doc/pangolin/copyright"
  mkdir -p "$deb/DEBIAN"
  cat > "$deb/DEBIAN/control" <<EOF
Package: pangolin
Version: $version
Section: utils
Priority: optional
Architecture: $arch
Depends: libc6, libgl1, libx11-6, libxrandr2, libxxf86vm1, libxi6, libxcursor1, libxinerama1
Maintainer: The Pangolin contributors <noreply@github.com>
Homepage: https://kaanbahasever.github.io/pangolin/
Description: Local-first password keeper
 Pangolin keeps passwords in an encrypted vault on your own computer.
 No account, no server and no network access.
EOF
  dpkg-deb --build --root-owner-group "$deb" "$dist/pangolin-linux-$arch.deb" >/dev/null
}

case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*) build_windows ;;
  Darwin) build_macos ;;
  Linux) build_linux ;;
  *) echo "unsupported platform: $(uname -s)" >&2; exit 1 ;;
esac

ls -l "$dist"
