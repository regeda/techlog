// Record events in a local Techlog server. Start it first: make seed && make run.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var base = env("TECHLOG_URL", "http://localhost:8080")

func call(method, path string, body any, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, base+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s %s -> %d: %s", method, path, res.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func main() {
	// 1. Record a proposal, linked to the original problem report.
	//    The server assigns the id and, because "time" is omitted, the time. The
	//    idempotency_key makes a retry safe: reusing it is rejected with 409.
	var proposal struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	}
	err := call("POST", "/api/v1/events", map[string]any{
		"schema_version":  "0.1-draft",
		"idempotency_key": "promo-api-spin-proposal",
		"type":            "proposal.recorded",
		"actor":           map[string]any{"kind": "ai_agent", "id": "agent:code-assistant", "on_behalf_of": "user:a.novak"},
		"subject":         map[string]any{"system": "promo-api", "component": "spin-service"},
		"status":          "proposed",
		"summary":         "Spin-to-win: one spin per new account, discount capped at 20%",
		"links":           []map[string]any{{"rel": "motivated_by", "target": "evt_01J9K2A0P002", "basis": "confirmed"}},
		"provenance":      map[string]any{"recorded_by": "agent:code-assistant", "method": "agent_submitted"},
	}, &proposal)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("recorded", proposal.Event.ID)

	// 2. Record the verification result. The test report is evidence, kept as
	//    a reference inside the event.
	err = call("POST", "/api/v1/events", map[string]any{
		"schema_version":  "0.1-draft",
		"idempotency_key": "promo-api-spin-verification-ci-90412",
		"type":            "verification.completed",
		"actor":           map[string]any{"kind": "system", "id": "ci:pipeline"},
		"subject":         map[string]any{"system": "promo-api", "component": "spin-service"},
		"status":          "verified",
		"summary":         "Unit, integration, and abuse tests passed",
		"evidence": []map[string]any{{
			"kind":   "test.report",
			"ref":    "ci://run/90412",
			"digest": "sha256:7b1e4d9a2c6f08e35a9d1b7c4e2f6a8091d3c5b7e9f0a2c4d6e8b0a1c3e5f7d9",
		}},
		"links":      []map[string]any{{"rel": "verifies", "target": "evt_01J9K2A0P006", "basis": "confirmed"}},
		"provenance": map[string]any{"recorded_by": "collector:ci", "method": "collected"},
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Read the latest events of the system, oldest first.
	var history struct {
		Events []struct {
			Time    string `json:"time"`
			Type    string `json:"type"`
			Summary string `json:"summary"`
		} `json:"events"`
	}
	if err := call("GET", "/api/v1/systems/promo-api/history?limit=5", nil, &history); err != nil {
		log.Fatal(err)
	}
	for _, e := range history.Events {
		fmt.Println(e.Time, e.Type, e.Summary)
	}
}
