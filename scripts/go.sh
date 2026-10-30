#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ -x "$ROOT/.tools/go/bin/go" ]; then export PATH="$ROOT/.tools/go/bin:$PATH"; fi
export GOMODCACHE="${GOMODCACHE:-$ROOT/.tools/gomod}"
export GOCACHE="${GOCACHE:-$ROOT/.tools/gocache}"
cd "$ROOT/services/api"
exec go "$@"
