#!/usr/bin/env bash
# Send a first event to a local Techlog server. Start it first: techlog seed && techlog serve.
# The server assigns the id and, because "time" is omitted, the time.
curl -fsS -X POST "${TECHLOG_URL:-http://localhost:8080}/api/v1/events" \
  -H "Content-Type: application/json" \
  -d '{
    "schema_version": "0.1-draft",
    "idempotency_key": "promo-api-report-2026-03-12-m.okafor",
    "type": "problem.reported",
    "actor": { "kind": "human", "id": "user:m.okafor" },
    "subject": { "system": "promo-api" },
    "provenance": { "recorded_by": "user:m.okafor", "method": "human_entered" }
  }'
