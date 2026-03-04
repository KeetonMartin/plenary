#!/usr/bin/env bash
set -euo pipefail

need_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
}

need_cmd go
need_cmd node
need_cmd npm
need_cmd jq

echo "==> install web dependencies"
(cd cmd/plenary/web && npm ci)

echo "==> build CLI (no web embed)"
go build -o plenary ./cmd/plenary

echo "==> done"
echo "Run 'make build-full' if you also want embedded web assets."
