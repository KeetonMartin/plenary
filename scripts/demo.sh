#!/usr/bin/env bash
set -euo pipefail

need_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
}

need_cmd jq

DB="${PLENARY_DB:-.plenary/events.jsonl}"
BIN="${PLENARY_BIN:-./plenary}"

if [[ ! -f "$BIN" ]]; then
  echo "plenary binary not found at $BIN; building it now"
  go build -o plenary ./cmd/plenary
fi

if [[ ! -f "$DB" ]]; then
  echo "no local dataset found at $DB" >&2
  echo "set PLENARY_DB to an existing real events jsonl file" >&2
  exit 1
fi

DEMO_PLENARY_ID="${DEMO_PLENARY_ID:-}"
if [[ -z "$DEMO_PLENARY_ID" ]]; then
  DEMO_PLENARY_ID="$(
    PLENARY_DB="$DB" "$BIN" list \
      | jq -r '[.[] | select(.closed == true)][0].plenary_id'
  )"
fi

if [[ -z "$DEMO_PLENARY_ID" || "$DEMO_PLENARY_ID" == "null" ]]; then
  echo "no closed plenary found in $DB; set DEMO_PLENARY_ID explicitly" >&2
  exit 1
fi

OUT="${DEMO_OUT:-.plenary/demo/$DEMO_PLENARY_ID}"
mkdir -p "$OUT"

echo "==> exporting real plenary data"
echo "db=$DB"
echo "plenary_id=$DEMO_PLENARY_ID"
echo "out=$OUT"
PLENARY_DB="$DB" "$BIN" export --plenary "$DEMO_PLENARY_ID" --out "$OUT" >/dev/null

echo "==> artifacts"
ls -1 "$OUT"
echo "OK: demo export complete"
