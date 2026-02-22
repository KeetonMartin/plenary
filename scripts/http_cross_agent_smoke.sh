#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 1
fi

BASE="${BASE:-http://127.0.0.1:8080}"
ACTOR1_ID="${ACTOR1_ID:-agent-a}"
ACTOR2_ID="${ACTOR2_ID:-agent-b}"
TOPIC="${TOPIC:-Cross-agent HTTP smoke test}"

post() {
  local path="$1"
  local body="$2"
  curl -fsS -X POST "$BASE$path" \
    -H 'Content-Type: application/json' \
    -d "$body"
}

get() {
  local path="$1"
  curl -fsS "$BASE$path"
}

actor_json() {
  local actor_id="$1"
  jq -nc --arg id "$actor_id" '{"actor_id":$id,"actor_type":"agent"}'
}

ACTOR1="$(actor_json "$ACTOR1_ID")"
ACTOR2="$(actor_json "$ACTOR2_ID")"

echo "==> Create plenary"
CREATE_BODY="$(jq -nc --argjson actor "$ACTOR1" --arg topic "$TOPIC" '{actor:$actor,topic:$topic,decision_rule:"unanimity"}')"
PID="$(post "/api/plenaries" "$CREATE_BODY" | jq -r '.plenary_id')"

if [[ -z "$PID" || "$PID" == "null" ]]; then
  echo "failed to create plenary" >&2
  exit 1
fi
echo "plenary_id=$PID"

echo "==> Join two actors"
post "/api/plenaries/$PID/join" "$(jq -nc --argjson actor "$ACTOR1" '{actor:$actor}')" >/dev/null
post "/api/plenaries/$PID/join" "$(jq -nc --argjson actor "$ACTOR2" '{actor:$actor}')" >/dev/null

echo "==> Framing contributions"
post "/api/plenaries/$PID/speak" "$(jq -nc --argjson actor "$ACTOR1" --arg text "Actor A framing" '{actor:$actor,text:$text}')" >/dev/null
post "/api/plenaries/$PID/speak" "$(jq -nc --argjson actor "$ACTOR2" --arg text "Actor B framing" '{actor:$actor,text:$text}')" >/dev/null

echo "==> Phase progression"
post "/api/plenaries/$PID/phase" "$(jq -nc --argjson actor "$ACTOR1" '{actor:$actor,to:"divergence",from:"framing"}')" >/dev/null
post "/api/plenaries/$PID/phase" "$(jq -nc --argjson actor "$ACTOR1" '{actor:$actor,to:"proposal",from:"divergence"}')" >/dev/null

echo "==> Propose"
PROPOSAL_ID="$(
  post "/api/plenaries/$PID/propose" \
    "$(jq -nc --argjson actor "$ACTOR1" --arg text "Proceed with the HTTP path" '{actor:$actor,text:$text}')" \
  | jq -r '.proposal_id'
)"
echo "proposal_id=$PROPOSAL_ID"

post "/api/plenaries/$PID/phase" "$(jq -nc --argjson actor "$ACTOR1" '{actor:$actor,to:"consensus_check",from:"proposal"}')" >/dev/null

echo "==> Consent from both actors"
post "/api/plenaries/$PID/consent" "$(jq -nc --argjson actor "$ACTOR1" --arg proposal_id "$PROPOSAL_ID" --arg reason "Looks good" '{actor:$actor,proposal_id:$proposal_id,reason:$reason}')" >/dev/null
post "/api/plenaries/$PID/consent" "$(jq -nc --argjson actor "$ACTOR2" --arg proposal_id "$PROPOSAL_ID" --arg reason "Agree" '{actor:$actor,proposal_id:$proposal_id,reason:$reason}')" >/dev/null

echo "==> Close"
post "/api/plenaries/$PID/close" "$(jq -nc --argjson actor "$ACTOR1" --arg resolution "Cross-agent HTTP smoke passed" '{actor:$actor,outcome:"consensus",resolution:$resolution}')" >/dev/null

echo "==> Verify"
STATUS="$(get "/api/plenaries/$PID")"
echo "$STATUS" | jq '{plenary_id,phase,closed,outcome,event_count,ready_to_close}'

echo "$STATUS" | jq -e '.closed == true and .outcome == "consensus"' >/dev/null
echo "OK: cross-agent HTTP smoke completed"
