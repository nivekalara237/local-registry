#!/usr/bin/env bash
# Runs the Hurl scenarios of the HTTP API against a fresh local-registry.
#
#   e2e/hurl/run.sh                     build, start a server on an empty asset path, run every scenario
#   e2e/hurl/run.sh products.hurl       run only these scenarios (seed/ always runs first)
#   URL=https://registry.example.com e2e/hurl/run.sh products.hurl
#                                       run against a running server: no build, no seed
#
# Environment:
#   PORT          port of the local server (default 18090)
#   HURL_REPORT   directory for the HTML and JUnit reports (default: none)
#   HURL_OPTS     extra hurl options, such as --very-verbose
#
# The upload token is drawn for the run and handed to hurl through a secrets file:
# it never appears on a command line nor in the reports.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
port=${PORT:-18090}
work=$(mktemp -d)
pid=

cleanup() {
  if [ -n "$pid" ]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

command -v hurl >/dev/null || { echo "hurl is not installed: https://hurl.dev/docs/installation.html" >&2; exit 1; }

cd "$here"
if [ "$#" -gt 0 ]; then
  scenarios=("$@")
else
  scenarios=(*.hurl)
fi

report=()
if [ -n "${HURL_REPORT:-}" ]; then
  mkdir -p "$HURL_REPORT"
  report=(--report-html "$HURL_REPORT/html" --report-junit "$HURL_REPORT/junit.xml")
fi

# checkFixtures fails when a fixture no longer has the sha256 its X-Checksum-Sha256 header expects,
# typically after an editor reformatted it: the server would only answer 422.
checkFixtures() {
  local hurl line want file got bad=0
  for hurl in *.hurl seed/*.hurl; do
    want=
    while IFS= read -r line; do
      case $line in
        "X-Checksum-Sha256: "*) want=$(printf '%s' "${line#X-Checksum-Sha256: }" | tr 'A-F' 'a-f') ;;
        file,*\;)
          file=${line#file,}
          file=${file%;}
          if [ -n "$want" ]; then
            got=$(shasum -a 256 "$file" 2>/dev/null | cut -d' ' -f1)
            if [ "$got" != "$want" ]; then
              echo "$hurl: $file has sha256 ${got:-<missing file>}, X-Checksum-Sha256 says $want" >&2
              bad=1
            fi
          fi
          want=
          ;;
        "") want= ;;
      esac
    done <"$hurl"
  done
  if [ "$bad" -ne 0 ]; then
    echo "a fixture changed (reformatted by an editor?): restore it or update its X-Checksum-Sha256" >&2
    return 1
  fi
}
checkFixtures

secrets="$work/secrets"
(umask 077 && : >"$secrets")

if [ -n "${URL:-}" ]; then
  url=$URL
  if [ -n "${LOCAL_REGISTRY_UPLOAD_TOKEN:-}" ]; then
    printf 'token=%s\n' "$LOCAL_REGISTRY_UPLOAD_TOKEN" >>"$secrets"
  else
    printf 'token=none\n' >>"$secrets"
  fi
else
  url="http://127.0.0.1:$port"
  if curl -fsS -o /dev/null "$url/healthz" 2>/dev/null; then
    echo "port $port is already in use: set PORT" >&2
    exit 1
  fi
  echo "building local-registry"
  (cd "$repo" && go build -o "$work/local-registry" ./cmd)
  token=$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')
  printf 'token=%s\n' "$token" >>"$secrets"
  mkdir -p "$work/data"
  LOCAL_REGISTRY_UPLOAD_TOKEN=$token "$work/local-registry" serve \
    --asset-path "$work/data" --address 127.0.0.1 --port "$port" \
    --public-url "$url" >"$work/server.log" 2>&1 &
  pid=$!
  unset token
  for _ in $(seq 50); do
    curl -fsS -o /dev/null "$url/healthz" 2>/dev/null && break
    kill -0 "$pid" 2>/dev/null || { cat "$work/server.log" >&2; exit 1; }
    sleep 0.1
  done
  echo "seeding $url"
  hurl --test --jobs 1 --file-root "$here" --secrets-file "$secrets" --variable "url=$url" ${HURL_OPTS:-} seed/*.hurl
fi

status=0
hurl --test --file-root "$here" --secrets-file "$secrets" --variable "url=$url" ${report[@]+"${report[@]}"} ${HURL_OPTS:-} "${scenarios[@]}" || status=$?
if [ "$status" -ne 0 ] && [ -n "$pid" ]; then
  echo "--- server log" >&2
  cat "$work/server.log" >&2
fi
exit "$status"
