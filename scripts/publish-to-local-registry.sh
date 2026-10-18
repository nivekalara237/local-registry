#!/bin/sh
# Uploads a goreleaser dist/ to the local registry, then publishes it.
# Run it after `goreleaser release`: goreleaser writes dist/artifacts.json after its
# publishers, so a `publishers:` block alone cannot send it.
#
# Usage: publish-to-local-registry.sh [--latest] <product> [dist-dir]
#   LOCAL_REGISTRY_URL           e.g. http://localhost:8077
#   LOCAL_REGISTRY_UPLOAD_TOKEN  the server's token; sent on stdin, never on a command line
#
# Sends the files listed in the checksums file (each with X-Checksum-Sha256), the checksums
# file, any *.sig/*.pem/*.sigstore.json beside it, metadata.json, registry.yaml (current
# directory, required), CHANGELOG.md (dist-dir) when it exists, and artifacts.json, then
# POSTs publish. For files not built by goreleaser, use upload.sh with a directory.
set -eu

latest=false
if [ "${1:-}" = "--latest" ]; then
	latest=true
	shift
fi
product=${1:?usage: $0 [--latest] <product> [dist-dir]}
dist=${2:-dist}
url=${LOCAL_REGISTRY_URL:?LOCAL_REGISTRY_URL is not set}
: "${LOCAL_REGISTRY_UPLOAD_TOKEN:?LOCAL_REGISTRY_UPLOAD_TOKEN is not set}"

for f in metadata.json artifacts.json; do
	[ -f "$dist/$f" ] || {
		echo "$dist/$f not found: run goreleaser first, or publish a directory of files with upload.sh --version <v> $product <dir>" >&2
		exit 1
	}
done
[ -f registry.yaml ] || { echo "registry.yaml not found in the current directory: the server requires it" >&2; exit 1; }
version=$(sed -n 's/.*"tag": *"\([^"]*\)".*/\1/p' "$dist/metadata.json")
[ -n "$version" ] || { echo "no tag in $dist/metadata.json" >&2; exit 1; }

set -- "$dist"/*checksums.txt
[ $# -eq 1 ] && [ -f "$1" ] || { echo "expected exactly one *checksums.txt in $dist" >&2; exit 1; }
checksums=$1

# call <curl args...>: sends the token on stdin, prints the server's error body on failure.
call() {
	if ! resp=$(printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
		curl --fail-with-body -sS -H @- "$@"); then
		echo "$resp" >&2
		exit 1
	fi
}

# put <file> [sha256]
put() {
	name=$(basename "$1")
	echo "PUT $name"
	call ${2:+-H "X-Checksum-Sha256: $2"} -T "$1" "$url/$product/artifacts/$version/files/$name"
}

while read -r sum name; do
	[ -n "$name" ] || continue
	put "$dist/${name#\*}" "$sum"
done < "$checksums"
put "$checksums"
for f in "$dist"/*.sig "$dist"/*.pem "$dist"/*.sigstore.json; do
	[ -f "$f" ] && put "$f"
done
put "$dist/metadata.json"
put registry.yaml
[ ! -f "$dist/CHANGELOG.md" ] || put "$dist/CHANGELOG.md"
put "$dist/artifacts.json"

echo "POST publish $product/$version (latest=$latest)"
call -X POST "$url/$product/artifacts/$version/publish?latest=$latest"
echo "$resp"
