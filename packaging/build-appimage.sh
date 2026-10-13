#!/usr/bin/env bash
# Usage: build-appimage.sh <tag> [outdir]   (linux/amd64 only)
set -euo pipefail

tag=${1:?tag}
out=${2:-dist}

APPIMAGETOOL_VERSION=1.9.1
APPIMAGETOOL_SHA256="ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0"

[[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$ ]] || { echo "invalid tag: $tag" >&2; exit 2; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
appdir=$work/AppDir
mkdir -p "$appdir/usr/bin" "$out"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-s -w -X github.com/kushiemoon-dev/quaver-stream-companion/internal/version.Version=$tag" \
  -o "$appdir/usr/bin/qsc" ./cmd/qsc
install -m 755 packaging/appimage/AppRun "$appdir/AppRun"
install -m 644 packaging/appimage/qsc.desktop "$appdir/qsc.desktop"
install -m 644 web/dashboard/favicon.svg "$appdir/qsc.svg"
ln -s qsc.svg "$appdir/.DirIcon"

tool=$work/appimagetool
curl -fsSL -o "$tool" "https://github.com/AppImage/appimagetool/releases/download/$APPIMAGETOOL_VERSION/appimagetool-x86_64.AppImage"
echo "$APPIMAGETOOL_SHA256  $tool" | sha256sum -c -
chmod +x "$tool"

ARCH=x86_64 "$tool" --appimage-extract-and-run "$appdir" "$out/qsc_${tag}_linux_x86_64.AppImage"
