"""Illustrative: the Techlog API is a proposal; no hosted endpoint exists."""
import json
import os
import urllib.request

BASE = os.environ.get("TECHLOG_URL", "https://techlog.example.invalid")
TOKEN = os.environ.get("TECHLOG_TOKEN", "demo-token")


def call(method, path, body=None):
    request = urllib.request.Request(
        BASE + path,
        method=method,
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request) as response:
        return json.load(response)


# 1. Record a proposal, linked to the original problem report.
event = call("POST", "/v0/events", {
    "schema_version": "0.1-draft",
    "type": "proposal.recorded",
    "time": "2026-03-10T11:30:00Z",
    "actor": {"kind": "ai_agent", "id": "agent:code-assistant", "on_behalf_of": "user:a.novak"},
    "subject": {"system": "promo-api", "component": "spin-service"},
    "status": "proposed",
    "summary": "Spin-to-win: one spin per new account, discount capped at 20%",
    "links": [{"rel": "motivated_by", "target": "evt_01J9K2A0P002", "basis": "confirmed"}],
    "provenance": {"recorded_by": "agent:code-assistant", "method": "agent_submitted"},
})
print("recorded", event["id"])

# 2. Attach verification evidence by reference.
call("POST", f"/v0/events/{event['id']}/evidence", {"kind": "test.report", "ref": "ci://run/90412"})

# 3. Read recent history for the system.
history = call("GET", "/v0/systems/promo-api/history?limit=5")
for e in history.get("events", []):
    print(e["time"], e["type"], e.get("summary", ""))
