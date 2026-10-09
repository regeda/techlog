// Record events in a local Techlog server. Start it first: make seed && make run.
const BASE = process.env.TECHLOG_URL ?? "http://localhost:8080";

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`${method} ${path} -> ${res.status}: ${await res.text()}`);
  return (await res.json()) as T;
}

// 1. Record a proposal, linked to the original problem report.
//    The server assigns the id and, because "time" is omitted, the time.
const { event: proposal } = await call<{ event: { id: string } }>("POST", "/api/v1/events", {
  schema_version: "0.1-draft",
  type: "proposal.recorded",
  actor: { kind: "ai_agent", id: "agent:code-assistant", on_behalf_of: "user:a.novak" },
  subject: { system: "promo-api", component: "spin-service" },
  status: "proposed",
  summary: "Spin-to-win: one spin per new account, discount capped at 20%",
  links: [{ rel: "motivated_by", target: "evt_01J9K2A0P002", basis: "confirmed" }],
  provenance: { recorded_by: "agent:code-assistant", method: "agent_submitted" },
});
console.log("recorded", proposal.id);

// 2. Record the verification result. The test report is evidence, kept as a
//    reference inside the event.
await call("POST", "/api/v1/events", {
  schema_version: "0.1-draft",
  type: "verification.completed",
  actor: { kind: "system", id: "ci:pipeline" },
  subject: { system: "promo-api", component: "spin-service" },
  status: "verified",
  summary: "Unit, integration, and abuse tests passed",
  evidence: [{
    kind: "test.report",
    ref: "ci://run/90412",
    digest: "sha256:7b1e4d9a2c6f08e35a9d1b7c4e2f6a8091d3c5b7e9f0a2c4d6e8b0a1c3e5f7d9",
  }],
  links: [{ rel: "verifies", target: "evt_01J9K2A0P006", basis: "confirmed" }],
  provenance: { recorded_by: "collector:ci", method: "collected" },
});

// 3. Read the latest events of the system, oldest first.
const history = await call<{ events: { time: string; type: string; summary?: string }[] }>(
  "GET", "/api/v1/systems/promo-api/history?limit=5");
for (const e of history.events) console.log(e.time, e.type, e.summary ?? "");
