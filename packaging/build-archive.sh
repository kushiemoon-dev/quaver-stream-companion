#!/usr/bin/env bash
# Usage: build-archive.sh <tag> <goos> <goarch> [outdir]
set -euo pipefail

tag=${1:?tag}
goos=${2:?goos}
goarch=${3:?goarch}
out=${4:-dist}

[[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$ ]] || { echo "invalid tag: $tag" >&2; exit 2; }
case "$goos/$goarch" in
  windows/amd64 | linux/amd64 | linux/arm64) ;;
  *) echo "unsupported target: $goos/$goarch" >&2; exit 2 ;;
esac

name=qsc_${tag}_${goos}_${goarch}
stage=$out/$name
bin=qsc
[[ $goos == windows ]] && bin=qsc.exe

rm -rf "$stage"
mkdir -p "$stage/docs"
CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
  -ldflags "-s -w -X github.com/kushiemoon-dev/quaver-stream-companion/internal/version.Version=$tag" \
  -o "$stage/$bin" ./cmd/qsc
cp LICENSE README.md "$stage/"
cp docs/*.md "$stage/docs/"
[[ $goos == linux ]] && cp packaging/qsc.service "$stage/"

if [[ $goos == windows ]]; then
  (cd "$out" && zip -qr "$name.zip" "$name")
else
  tar -C "$out" -czf "$out/$name.tar.gz" "$name"
fi
rm -rf "$stage"
