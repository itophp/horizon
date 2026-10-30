#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ -x "$ROOT/.tools/go/bin/gofmt" ]; then export PATH="$ROOT/.tools/go/bin:$PATH"; fi
if [ "${1:-}" = "--write" ]; then exec gofmt -w "$ROOT/services/api"; fi
files=$(gofmt -l "$ROOT/services/api")
if [ -n "$files" ]; then printf '%s\n' "$files"; exit 1; fi
