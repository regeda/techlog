#!/usr/bin/env bash
# Record events in a local Techlog server. Start it first: make seed && make run.
set -euo pipefail
TECHLOG_URL="${TECHLOG_URL:-http://localhost:8080}"

# 1. Record a proposal, linked to the original problem report.
#    The server assigns the id and, because "time" is omitted, the time. The
#    idempotency_key makes a retry safe: reusing it is rejected with 409.
ID=$(curl -fsS -X POST "$TECHLOG_URL/api/v1/events" -H "Content-Type: application/json" \
  -d '{
    "schema_version": "0.1-draft",
    "idempotency_key": "promo-api-spin-proposal",
    "type": "proposal.recorded",
    "actor": { "kind": "ai_agent", "id": "agent:code-assistant", "on_behalf_of": "user:a.novak" },
    "subject": { "system": "promo-api", "component": "spin-service" },
    "status": "proposed",
    "summary": "Spin-to-win: one spin per new account, discount capped at 20%",
    "links": [{ "rel": "motivated_by", "target": "evt_01J9K2A0P002", "basis": "confirmed" }],
    "provenance": { "recorded_by": "agent:code-assistant", "method": "agent_submitted" }
  }' | grep -o '"id":"evt_[0-9A-Z]*"' | head -n 1 | cut -d'"' -f4)
echo "recorded $ID"

# 2. Record the verification result. The test report is evidence, kept as a
#    reference inside the event.
curl -fsS -X POST "$TECHLOG_URL/api/v1/events" -H "Content-Type: application/json" \
  -d '{
    "schema_version": "0.1-draft",
    "idempotency_key": "promo-api-spin-verification-ci-90412",
    "type": "verification.completed",
    "actor": { "kind": "system", "id": "ci:pipeline" },
    "subject": { "system": "promo-api", "component": "spin-service" },
    "status": "verified",
    "summary": "Unit, integration, and abuse tests passed",
    "evidence": [{
      "kind": "test.report",
      "ref": "ci://run/90412",
      "digest": "sha256:7b1e4d9a2c6f08e35a9d1b7c4e2f6a8091d3c5b7e9f0a2c4d6e8b0a1c3e5f7d9"
    }],
    "links": [{ "rel": "verifies", "target": "evt_01J9K2A0P006", "basis": "confirmed" }],
    "provenance": { "recorded_by": "collector:ci", "method": "collected" }
  }' > /dev/null

# 3. Read the latest events of the system, oldest first.
curl -fsS "$TECHLOG_URL/api/v1/systems/promo-api/history?limit=5"
