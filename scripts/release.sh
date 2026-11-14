#!/bin/sh
set -eu
version=${1:-dev}
case "$version" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo 'Usage: scripts/release.sh v0.1.0' >&2;exit 1;; esac
case "$version" in *[!a-zA-Z0-9._-]*) echo 'Invalid release tag.' >&2;exit 1;; esac
test -f internal/webassets/dist/index.html || { echo 'Run make web first.' >&2;exit 1; }
out=dist/release
mkdir -p "$out"
release_root=$(pwd)
for target_os in linux darwin windows; do
 for target_arch in amd64 arm64; do
  name=folio_${version#v}_${target_os}_${target_arch}
  work=$(mktemp -d "${TMPDIR:-/tmp}/folio-release.XXXXXX")
  executable=folio
  if [ "$target_os" = windows ]; then executable=folio.exe;fi
  CGO_ENABLED=0 GOOS=$target_os GOARCH=$target_arch go build -trimpath -ldflags "-s -w -buildid= -X main.version=$version" -o "$work/$executable" ./cmd/folio
  cp LICENSE README.md INSTALL.md THIRD_PARTY_NOTICES.md "$work/"
  cp web/public/fonts/OFL.txt "$work/PRETENDARD-OFL.txt"
  if [ "$target_os" = windows ]; then (cd "$work" && zip -X -q "$release_root/$out/$name.zip" "$executable" LICENSE README.md INSTALL.md THIRD_PARTY_NOTICES.md PRETENDARD-OFL.txt); else tar -czf "$out/$name.tar.gz" -C "$work" "$executable" LICENSE README.md INSTALL.md THIRD_PARTY_NOTICES.md PRETENDARD-OFL.txt;fi
  rm -rf "$work"
 done
done
(cd "$out" && if command -v sha256sum >/dev/null 2>&1;then sha256sum ./*.tar.gz ./*.zip;else shasum -a 256 ./*.tar.gz ./*.zip;fi) | sed 's#  \./#  #' > "$out/checksums.txt"
