# Record examples

Read this when the user asks for the shape of an event type other than the incident example in `SKILL.md`.

A decision, linked to the proposal it decides:

```json event
{
  "schema_version": "0.1-draft",
  "idempotency_key": "promo-api-spin-proposal-approved",
  "type": "decision.made",
  "time": "2026-03-10T14:05:00Z",
  "actor": {"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak"},
  "subject": {"system": "promo-api", "component": "spin-service"},
  "summary": "Approved the spin-to-win proposal: one spin per new account, discount capped at 20%",
  "links": [{"rel": "decides", "target": "evt_01J9K2A0P004", "basis": "confirmed"}],
  "provenance": {"recorded_by": "agent:claude-code", "method": "agent_submitted"}
}
```
