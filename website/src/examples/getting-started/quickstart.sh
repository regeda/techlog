#!/usr/bin/env bash
# Illustrative: the Techlog API is a proposal; no hosted endpoint exists.
set -euo pipefail
TECHLOG_URL="${TECHLOG_URL:-https://techlog.example.invalid}"
TECHLOG_TOKEN="${TECHLOG_TOKEN:-demo-token}"

# 1. Record a proposal, linked to the original problem report.
ID=$(curl -fsS -X POST "$TECHLOG_URL/v0/events" \
  -H "Authorization: Bearer $TECHLOG_TOKEN" -H "Content-Type: application/json" \
  -d '{
    "schema_version": "0.1-draft",
    "type": "proposal.recorded",
    "time": "2026-03-10T11:30:00Z",
    "actor": { "kind": "ai_agent", "id": "agent:code-assistant", "on_behalf_of": "user:a.novak" },
    "subject": { "system": "promo-api", "component": "spin-service" },
    "status": "proposed",
    "summary": "Spin-to-win: one spin per new account, discount capped at 20%",
    "links": [{ "rel": "motivated_by", "target": "evt_01J9K2A0P002", "basis": "confirmed" }],
    "provenance": { "recorded_by": "agent:code-assistant", "method": "agent_submitted" }
  }' | sed -n 's/.*"id" *: *"\([^"]*\)".*/\1/p')
echo "recorded $ID"

# 2. Attach verification evidence by reference.
curl -fsS -X POST "$TECHLOG_URL/v0/events/$ID/evidence" \
  -H "Authorization: Bearer $TECHLOG_TOKEN" -H "Content-Type: application/json" \
  -d '{ "kind": "test.report", "ref": "ci://run/90412" }'

# 3. Read recent history for the system.
curl -fsS "$TECHLOG_URL/v0/systems/promo-api/history?limit=5" \
  -H "Authorization: Bearer $TECHLOG_TOKEN"
