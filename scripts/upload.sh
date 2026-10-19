#!/usr/bin/env bash
# Uploads a release to the local registry, then publishes the version: the files listed in a
# goreleaser artifacts.json (run it after `goreleaser release`), or every file of a directory.
#
# Usage: upload.sh [--latest] [--dry-run] [--root DIR] [--version V] [--product-file F] <product> [artifacts.json | DIR]
#   --latest     also make this version the product's latest (publish?latest=true)
#   --dry-run    print what would be sent, send nothing (no token needed)
#   --root DIR   directory the "path" fields are relative to (goreleaser's working directory);
#                guessed from where artifacts.json sits, else the current directory
#   --version V  version to publish; default: "tag" (else "version") of metadata.json
#   --product-file F  product card (registry.yaml, required by the server); default: the
#                registry.yaml of the root (or DIR), else of the current directory
#   The default is dist/artifacts.json; with --root, ROOT/dist/artifacts.json, else ROOT as
#   a DIR. A DIR holding an artifacts.json means that file.
#
#   LOCAL_REGISTRY_URL           e.g. http://localhost:8077
#   LOCAL_REGISTRY_UPLOAD_TOKEN  the server's token; sent on stdin, never on a command line
#
# With artifacts.json, each file is sent under its "name" (its "path" is only where it is
# read from), with X-Checksum-Sha256 when the entry carries a sha256 checksum. Build outputs
# (type Binary without extra.Format) are skipped: they are the raw files the archives were
# made from, are not in the checksums file, and several share one name. Then come the
# product card (registry.yaml) and goreleaser's CHANGELOG.md (beside artifacts.json) when
# they exist, and artifacts.json itself last.
# With a DIR without artifacts.json, every regular file of DIR is sent, with
# X-Checksum-Sha256, then the product card. DIR may hold no subdirectory: archive it first
# (tar czf). The server infers each file's kind from its name, and writes checksums.txt
# when none is sent. Needs bash, curl and jq; and sha256sum or shasum for a DIR.
set -euo pipefail

die() { echo "upload.sh: $*" >&2; exit 1; }
usage() { sed -n '4,14s/^# \{0,1\}//p' "$0" >&2; exit 2; }

# shellcheck disable=SC1007
latest=false dry=false root= version= productfile=
while [ $# -gt 0 ]; do
	case $1 in
	--latest) latest=true ;;
	--dry-run) dry=true ;;
	--root) [ $# -ge 2 ] || usage; root=$2; shift ;;
	--root=*) root=${1#*=} ;;
	--version) [ $# -ge 2 ] || usage; version=$2; shift ;;
	--version=*) version=${1#*=} ;;
	--product-file) [ $# -ge 2 ] || usage; productfile=$2; shift ;;
	--product-file=*) productfile=${1#*=} ;;
	-h | --help) usage ;;
	--) shift; break ;;
	-*) echo "upload.sh: unknown option $1" >&2; usage ;;
	*) break ;;
	esac
	shift
done
[ $# -ge 1 ] && [ $# -le 2 ] || usage
product=$1
if [ $# -ge 2 ]; then
	artifacts=$2
elif [ -n "$root" ]; then # relative to the root, never to the current directory
	artifacts=$root
	[ ! -f "$root/dist/artifacts.json" ] || artifacts=$root/dist/artifacts.json
else
	artifacts=dist/artifacts.json
fi
plain=
if [ -d "$artifacts" ]; then
	if [ -f "$artifacts/artifacts.json" ]; then
		artifacts=${artifacts%/}/artifacts.json
	else
		plain=$(cd "$artifacts" && pwd)
	fi
fi

command -v jq >/dev/null || die "jq is required"
command -v curl >/dev/null || die "curl is required"
if ! $dry; then
	url=${LOCAL_REGISTRY_URL:?LOCAL_REGISTRY_URL is not set}
	url=${url%/}
	: "${LOCAL_REGISTRY_UPLOAD_TOKEN:?LOCAL_REGISTRY_UPLOAD_TOKEN is not set}"
fi

# call <curl args...>: sends the token on stdin, prints the server's error body on failure.
call() {
	if ! resp=$(printf 'Authorization: Bearer %s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" |
		curl --fail-with-body -sS -H @- "$@"); then
		echo "$resp" >&2
		exit 1
	fi
}

# put <name> <file> [sha256]
put() {
	local enc
	enc=$(jq -rn --arg n "$1" '$n | @uri')
	echo "PUT $1 <- $2${3:+ (sha256 $3)}"
	$dry && return
	call ${3:+-H "X-Checksum-Sha256: $3"} -T "$2" "$url/$product/artifacts/$version/files/$enc"
}

publish() {
	echo "POST publish $product/$version (latest=$latest)"
	$dry && exit 0
	call -X POST "$url/$product/artifacts/$version/publish?latest=$latest"
	echo "$resp"
}

if [ -n "$plain" ]; then
	if command -v sha256sum >/dev/null; then
		sha256() { sha256sum "$1" | cut -d' ' -f1; }
	elif command -v shasum >/dev/null; then
		sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
	else
		die "sha256sum or shasum is required"
	fi
	if [ -n "$productfile" ]; then
		[ -f "$productfile" ] || die "--product-file $productfile not found"
	elif [ -f "$plain/registry.yaml" ]; then
		productfile=$plain/registry.yaml
	elif [ -f registry.yaml ]; then
		productfile=$PWD/registry.yaml
	else
		die "no registry.yaml in $plain nor in the current directory: pass --product-file"
	fi
	files=()
	for f in "$plain"/*; do
		[ -e "$f" ] || continue
		name=${f##*/}
		[ ! -d "$f" ] || die "$f is a directory: archive it first, e.g. tar czf $name.tar.gz -C $plain $name"
		[ -f "$f" ] || die "$f is not a regular file"
		case $name in
		release.json | index.json | artifacts.goreleaser.json) die "$f: $name is reserved, the server writes it" ;;
		registry.yaml) continue ;; # sent as the product card
		esac
		files+=("$f")
	done
	[ ${#files[@]} -gt 0 ] || die "no file to publish in $plain"
	if [ -z "$version" ]; then
		[ -f "$plain/metadata.json" ] || die "no metadata.json in $plain: pass --version"
		version=$(jq -r '.tag // .version // empty' "$plain/metadata.json")
		[ -n "$version" ] || die "no tag nor version in $plain/metadata.json: pass --version"
	fi

	echo "== $product $version from $plain"
	for f in "${files[@]}"; do
		put "${f##*/}" "$f" "$(sha256 "$f")"
	done
	put registry.yaml "$productfile" "$(sha256 "$productfile")"
	publish
	exit 0
fi

[ -f "$artifacts" ] || die "$artifacts not found: run goreleaser first, or pass a directory of files"
jq -e 'type == "array"' "$artifacts" >/dev/null 2>&1 || die "$artifacts is not a JSON array"

# Root: the "path" fields are relative to goreleaser's working directory. The metadata (or
# checksums) entry sits in the dist directory, beside artifacts.json: strip its relative dir.
dist=$(cd "$(dirname "$artifacts")" && pwd)
if [ -z "$root" ]; then
	rel=$(jq -r 'first(.[] | select(.type == "Metadata" or .type == "Checksum") | .path) // empty' "$artifacts")
	rel=$(dirname "${rel:-.}")
	case $rel in
	/*) root=/ ;;
	.) root=$dist ;;
	*) case $dist in */"$rel") root=${dist%/"$rel"} ;; *) root=$PWD ;; esac ;;
	esac
fi
[ -d "$root" ] || die "--root $root is not a directory"

# resolve <path>: the file on disk for an artifacts.json path.
resolve() {
	case $1 in
	/*) printf '%s\n' "$1" ;;
	*) printf '%s\n' "$root/$1" ;;
	esac
}

# The release files: name, path, sha256 (may be empty), tab-separated.
list=$(jq -r '
	.[]
	| select(
		(.type == "Binary" and (.extra.Format // "") != "")
		or (.type as $t | ["Archive", "File", "Source", "Linux Package", "Checksum",
			"Signature", "Certificate", "SBOM", "Metadata"] | index($t))
	)
	| [.name, .path,
	   ((.extra.Checksum // "") | if startswith("sha256:") then .[7:] else "" end)]
	| @tsv' "$artifacts")
[ -n "$list" ] || die "no uploadable file in $artifacts"

dups=$(printf '%s\n' "$list" | cut -f1 | sort | uniq -d)
[ -z "$dups" ] || die "several entries of $artifacts share a name: $(echo $dups)"
printf '%s\n' "$list" | cut -f1 | grep -qx artifacts.json && die "$artifacts lists a file named artifacts.json"

# The documents of the local-registry/v1 standard, sent before artifacts.json.
if [ -n "$productfile" ]; then
	[ -f "$productfile" ] || die "--product-file $productfile not found"
elif [ -f "$root/registry.yaml" ]; then
	productfile=$root/registry.yaml
elif [ -f registry.yaml ]; then
	productfile=$PWD/registry.yaml
else
	die "no registry.yaml in $root nor in the current directory: pass --product-file"
fi
changelog=
if [ -f "$dist/CHANGELOG.md" ]; then
	changelog=$dist/CHANGELOG.md
fi

# Check every file before sending anything, so a missing one does not leave a half upload.
while IFS=$'\t' read -r name path sum; do
	[ -f "$(resolve "$path")" ] || die "$name: $(resolve "$path") not found (wrong --root?)"
done <<<"$list"

if [ -z "$version" ]; then
	meta=$(printf '%s\n' "$list" | awk -F'\t' '$1 == "metadata.json" { print $2; exit }')
	[ -n "$meta" ] || die "$artifacts lists no metadata.json: pass --version"
	version=$(jq -r '.tag // .version // empty' "$(resolve "$meta")")
	[ -n "$version" ] || die "no tag nor version in $(resolve "$meta"): pass --version"
fi

echo "== $product $version from $artifacts (root $root)"
while IFS=$'\t' read -r name path sum; do
	put "$name" "$(resolve "$path")" "$sum"
done <<<"$list"
put registry.yaml "$productfile"
[ -z "$changelog" ] || put CHANGELOG.md "$changelog"
put artifacts.json "$artifacts"
publish
