#!/bin/sh
set -eu

# Download first, verify the published digest, then install. No server, tunnel,
# login, DNS mutation or agent configuration is started by this installer.
repo=YangTaeyoung/folio
version=${FOLIO_VERSION:-latest}
install_dir=${FOLIO_INSTALL_DIR:-"$HOME/.local/bin"}
case "$(uname -s)" in Darwin) target_os=darwin;; Linux) target_os=linux;; *) echo 'Use the Windows release archive; see INSTALL.md.' >&2; exit 1;; esac
case "$(uname -m)" in arm64|aarch64) target_arch=arm64;; x86_64|amd64) target_arch=amd64;; *) echo 'Unsupported architecture.' >&2;exit 1;; esac
if [ "$version" = latest ]; then
  version=$(curl --fail --silent --show-error --location --proto '=https' "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
fi
case "$version" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo 'Set FOLIO_VERSION to a release tag such as v0.1.0.' >&2;exit 1;; esac
case "$version" in *[!a-zA-Z0-9._-]*) echo 'Invalid release tag.' >&2;exit 1;; esac
tmp=$(mktemp -d "${TMPDIR:-/tmp}/folio-install.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
archive=folio_${version#v}_${target_os}_${target_arch}.tar.gz
base=https://github.com/$repo/releases/download/$version
curl --fail --silent --show-error --location --proto '=https' "$base/$archive" -o "$tmp/$archive"
curl --fail --silent --show-error --location --proto '=https' "$base/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v file="$archive" '$2 == file {print $1}' "$tmp/checksums.txt")
if [ ${#expected} -ne 64 ]; then echo 'Missing archive checksum.' >&2;exit 1;fi
if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$tmp/$archive" | awk '{print $1}'); else actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}');fi
if [ "$actual" != "$expected" ]; then echo 'Checksum mismatch; nothing installed.' >&2;exit 1;fi
tar -xzf "$tmp/$archive" -C "$tmp" folio
mkdir -p "$install_dir"
if [ -e "$install_dir/folio" ]; then
 if cmp -s "$tmp/folio" "$install_dir/folio"; then echo "Folio $version is already installed.";exit 0;fi
 if [ "${FOLIO_INSTALL_FORCE:-0}" != 1 ]; then echo 'Existing Folio differs. Set FOLIO_INSTALL_FORCE=1 to replace it after stopping the service and backing up data.' >&2;exit 1;fi
fi
install -m 755 "$tmp/folio" "$install_dir/folio"
"$install_dir/folio" version
echo "Installed in $install_dir. Add this directory to PATH. Run: folio serve"
