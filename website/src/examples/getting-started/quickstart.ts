// Illustrative: the Techlog API is a proposal; no hosted endpoint exists.
const BASE = process.env.TECHLOG_URL ?? "https://techlog.example.invalid";
const TOKEN = process.env.TECHLOG_TOKEN ?? "demo-token";

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: { Authorization: `Bearer ${TOKEN}`, "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status}`);
  return (await res.json()) as T;
}

// 1. Record a proposal, linked to the original problem report.
const event = await call<{ id: string }>("POST", "/v0/events", {
  schema_version: "0.1-draft",
  type: "proposal.recorded",
  time: "2026-03-10T11:30:00Z",
  actor: { kind: "ai_agent", id: "agent:code-assistant", on_behalf_of: "user:a.novak" },
  subject: { system: "promo-api", component: "spin-service" },
  status: "proposed",
  summary: "Spin-to-win: one spin per new account, discount capped at 20%",
  links: [{ rel: "motivated_by", target: "evt_01J9K2A0P002", basis: "confirmed" }],
  provenance: { recorded_by: "agent:code-assistant", method: "agent_submitted" },
});
console.log("recorded", event.id);

// 2. Attach verification evidence by reference.
await call("POST", `/v0/events/${event.id}/evidence`, { kind: "test.report", ref: "ci://run/90412" });

// 3. Read recent history for the system.
const history = await call<{ events: { time: string; type: string; summary?: string }[] }>(
  "GET", "/v0/systems/promo-api/history?limit=5");
for (const e of history.events) console.log(e.time, e.type, e.summary ?? "");
