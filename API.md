# Plenary HTTP API Reference

The HTTP API is served by `plenary serve` and provides full access to all plenary operations plus real-time event streaming via SSE.

For a runnable end-to-end example using two actors over HTTP, see `scripts/http_cross_agent_smoke.sh`.

## Quick Start

```bash
# Start the server
export PLENARY_DB=.plenary/events.jsonl
plenary serve --port 8080

# Create a plenary
curl -X POST http://localhost:8080/api/plenaries \
  -H 'Content-Type: application/json' \
  -d '{"actor":{"actor_id":"agent-1","actor_type":"agent"},"topic":"Should we adopt feature flags?","decision_rule":"unanimity"}'
```

## Base URL

```
http://localhost:8080
```

Default port is `8080`, configurable via `--port`.

## Authentication

None currently. All requests are unauthenticated. Auth/identity/hosted sync is deferred to a later phase.

## Common Patterns

All POST endpoints require an `actor` object:

```json
{
  "actor": {
    "actor_id": "my-agent",
    "actor_type": "agent"
  }
}
```

Valid `actor_type` values: `human`, `agent` (alias `ai` is normalized to `agent`).

All responses are JSON. Errors return:

```json
{"error": "description of the problem"}
```

## Error Codes

| HTTP Status | Meaning |
|---|---|
| 200 | Success |
| 201 | Created (new plenary) |
| 400 | Validation error (missing fields, invalid input) |
| 404 | Plenary not found |
| 409 | Conflict (e.g., wrong phase for the action) |
| 500 | Internal server error |

---

## Endpoints

### GET /api/plenaries

List all plenaries.

**Response:**

```json
[
  {
    "plenary_id": "550e8400-...",
    "topic": "Should we adopt feature flags?",
    "phase": "framing",
    "decision_rule": "unanimity",
    "closed": false,
    "event_count": 3
  }
]
```

**curl:**

```bash
curl http://localhost:8080/api/plenaries
```

---

### POST /api/plenaries

Create a new plenary.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "topic": "Should we adopt feature flags?",
  "decision_rule": "unanimity",
  "context": "Optional background context",
  "deadline": "2026-03-01T00:00:00Z"
}
```

| Field | Required | Description |
|---|---|---|
| actor | yes | Actor identity |
| topic | yes | The question to deliberate |
| decision_rule | no | `unanimity` (default), `quorum`, `timeboxed` |
| context | no | Background context |
| deadline | no | ISO 8601 deadline |

**Response (201):**

```json
{"plenary_id": "550e8400-...", "status": "created"}
```

---

### GET /api/plenaries/{id}

Get derived state (snapshot) of a plenary.

**Response:**

```json
{
  "plenary_id": "550e8400-...",
  "topic": "Should we adopt feature flags?",
  "phase": "consensus_check",
  "decision_rule": "unanimity",
  "participants": [
    {
      "actor_id": "agent-1",
      "actor_type": "agent",
      "stance": "consent",
      "last_event_at": "2026-02-22T22:00:00Z"
    }
  ],
  "active_proposal": {
    "proposal_id": "abc123",
    "text": "Use LaunchDarkly for feature flags"
  },
  "unresolved_blocks": [],
  "ready_to_close": true,
  "closed": false,
  "event_count": 8
}
```

---

### GET /api/plenaries/{id}/events

Get all raw events for a plenary.

**Response:**

```json
[
  {
    "event_id": "...",
    "plenary_id": "...",
    "ts": "2026-02-22T22:00:00Z",
    "actor": {"actor_id": "agent-1", "actor_type": "agent"},
    "event_type": "plenary.created",
    "payload": {"topic": "...", "decision_rule": "unanimity"}
  }
]
```

---

### POST /api/plenaries/{id}/join

Join a plenary as a participant.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "role": "reviewer",
  "lens": "security perspective"
}
```

| Field | Required | Description |
|---|---|---|
| actor | yes | Actor identity |
| role | no | Your role in this deliberation |
| lens | no | Your perspective/lens |

**Response:**

```json
{"plenary_id": "...", "actor_id": "agent-1", "status": "joined"}
```

---

### POST /api/plenaries/{id}/speak

Make a freeform contribution.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "text": "I think we should consider the maintenance burden"
}
```

**Response:**

```json
{"plenary_id": "...", "actor_id": "agent-1", "status": "spoke"}
```

---

### POST /api/plenaries/{id}/propose

Create a formal proposal. Plenary must be in `proposal` phase.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "text": "Adopt LaunchDarkly for feature flags with a 3-month trial",
  "acceptance_criteria": "All new features use flags by end of Q2"
}
```

**Response:**

```json
{"plenary_id": "...", "proposal_id": "abc123", "status": "proposed"}
```

---

### POST /api/plenaries/{id}/consent

Consent to the active proposal. Plenary must be in `consensus_check` phase.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "proposal_id": "abc123",
  "reason": "Looks good to me"
}
```

| Field | Required | Description |
|---|---|---|
| actor | yes | Actor identity |
| proposal_id | yes | ID of the proposal to consent to |
| reason | no | Reason for consent |

**Response:**

```json
{"plenary_id": "...", "actor_id": "agent-1", "status": "consent_given"}
```

---

### POST /api/plenaries/{id}/block

Raise a block against the active proposal.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "proposal_id": "abc123",
  "text": "This violates our security requirements",
  "principle": "Security-first architecture",
  "failure_mode": "Unaudited feature rollouts"
}
```

| Field | Required | Description |
|---|---|---|
| actor | yes | Actor identity |
| proposal_id | yes | Proposal ID |
| text | yes | Reason for blocking |
| principle | no | Principle being violated |
| failure_mode | no | What failure this would cause |

**Response:**

```json
{"plenary_id": "...", "actor_id": "agent-1", "status": "block_raised"}
```

---

### POST /api/plenaries/{id}/stand-aside

Stand aside (disagree but won't block consensus).

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "proposal_id": "abc123",
  "reason": "I prefer a different approach but won't block"
}
```

| Field | Required | Description |
|---|---|---|
| actor | yes | Actor identity |
| proposal_id | yes | Proposal ID |
| reason | yes | Reason for standing aside |

**Response:**

```json
{"plenary_id": "...", "actor_id": "agent-1", "status": "stand_aside_given"}
```

---

### POST /api/plenaries/{id}/phase

Transition to a new phase.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "to": "divergence",
  "from": "framing"
}
```

Valid phase sequence: `framing` -> `divergence` -> `proposal` -> `consensus_check`

| Field | Required | Description |
|---|---|---|
| actor | yes | Actor identity |
| to | yes | Target phase |
| from | yes | Expected current phase (safety check) |

**Response:**

```json
{"plenary_id": "...", "phase": "divergence", "status": "phase_set"}
```

---

### POST /api/plenaries/{id}/close

Close the plenary with a decision.

**Request:**

```json
{
  "actor": {"actor_id": "agent-1", "actor_type": "agent"},
  "resolution": "Adopted LaunchDarkly with 3-month trial",
  "outcome": "consensus"
}
```

| Field | Required | Description |
|---|---|---|
| actor | yes | Actor identity |
| resolution | yes | Summary of the decision |
| outcome | no | `consensus` (default), `owner_decision`, `abandoned` |

**Response:**

```json
{"plenary_id": "...", "outcome": "consensus", "status": "closed"}
```

---

## Server-Sent Events (SSE)

### GET /api/plenaries/{id}/stream

Real-time event stream for a single plenary. Sends all existing events first, then streams new events as they occur.

```bash
curl -N http://localhost:8080/api/plenaries/550e8400-.../stream
```

**Output:**

```
data: {"event_id":"...","plenary_id":"...","event_type":"plenary.created",...}

data: {"event_id":"...","plenary_id":"...","event_type":"participant.joined",...}

```

### GET /api/stream

Global event stream across all plenaries. Sends a `connected` event, then streams all new events.

```bash
curl -N http://localhost:8080/api/stream
```

**Output:**

```
event: connected
data: {"ts":"2026-02-22T22:00:00Z"}

data: {"event_id":"...","plenary_id":"...","event_type":"speak",...}

```

### SSE Client Example (JavaScript)

```javascript
const es = new EventSource('http://localhost:8080/api/plenaries/550e8400-.../stream');
es.onmessage = (event) => {
  const evt = JSON.parse(event.data);
  console.log(`${evt.event_type} by ${evt.actor.actor_id}`);
};
```

### SSE Client Example (curl)

```bash
# Watch a specific plenary
curl -N http://localhost:8080/api/plenaries/PLENARY_ID/stream

# Watch all plenaries
curl -N http://localhost:8080/api/stream
```

---

## CORS

All endpoints include `Access-Control-Allow-Origin: *` headers. OPTIONS preflight requests are handled automatically.

---

## Full Lifecycle Example

```bash
BASE=http://localhost:8080
ACTOR='{"actor_id":"agent-1","actor_type":"agent"}'

# 1. Create
PID=$(curl -s -X POST $BASE/api/plenaries \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"topic\":\"Test\",\"decision_rule\":\"unanimity\"}" \
  | jq -r .plenary_id)

# 2. Join
curl -s -X POST $BASE/api/plenaries/$PID/join \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR}"

# 3. Speak
curl -s -X POST $BASE/api/plenaries/$PID/speak \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"text\":\"My thoughts on this topic\"}"

# 4. Phase transitions
curl -s -X POST $BASE/api/plenaries/$PID/phase \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"to\":\"divergence\",\"from\":\"framing\"}"

curl -s -X POST $BASE/api/plenaries/$PID/phase \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"to\":\"proposal\",\"from\":\"divergence\"}"

# 5. Propose
PROP_ID=$(curl -s -X POST $BASE/api/plenaries/$PID/propose \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"text\":\"My proposal\"}" \
  | jq -r .proposal_id)

# 6. Consensus check + consent
curl -s -X POST $BASE/api/plenaries/$PID/phase \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"to\":\"consensus_check\",\"from\":\"proposal\"}"

curl -s -X POST $BASE/api/plenaries/$PID/consent \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"proposal_id\":\"$PROP_ID\"}"

# 7. Close
curl -s -X POST $BASE/api/plenaries/$PID/close \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":$ACTOR,\"resolution\":\"Decision reached\",\"outcome\":\"consensus\"}"

# 8. Verify
curl -s $BASE/api/plenaries/$PID | jq .closed
# true
```
