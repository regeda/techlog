// Illustrative: the Techlog API is a proposal; no hosted endpoint exists.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

var (
	base  = env("TECHLOG_URL", "https://techlog.example.invalid")
	token = env("TECHLOG_TOKEN", "demo-token")
)

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
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s -> %d", method, path, res.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func main() {
	// 1. Record a proposal, linked to the original problem report.
	var event struct {
		ID string `json:"id"`
	}
	err := call("POST", "/v0/events", map[string]any{
		"schema_version": "0.1-draft",
		"type":           "proposal.recorded",
		"time":           "2026-03-10T11:30:00Z",
		"actor":          map[string]any{"kind": "ai_agent", "id": "agent:code-assistant", "on_behalf_of": "user:a.novak"},
		"subject":        map[string]any{"system": "promo-api", "component": "spin-service"},
		"status":         "proposed",
		"summary":        "Spin-to-win: one spin per new account, discount capped at 20%",
		"links":          []map[string]any{{"rel": "motivated_by", "target": "evt_01J9K2A0P002", "basis": "confirmed"}},
		"provenance":     map[string]any{"recorded_by": "agent:code-assistant", "method": "agent_submitted"},
	}, &event)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("recorded", event.ID)

	// 2. Attach verification evidence by reference.
	if err := call("POST", "/v0/events/"+event.ID+"/evidence",
		map[string]any{"kind": "test.report", "ref": "ci://run/90412"}, nil); err != nil {
		log.Fatal(err)
	}

	// 3. Read recent history for the system.
	var history struct {
		Events []struct {
			Time    string `json:"time"`
			Type    string `json:"type"`
			Summary string `json:"summary"`
		} `json:"events"`
	}
	if err := call("GET", "/v0/systems/promo-api/history?limit=5", nil, &history); err != nil {
		log.Fatal(err)
	}
	for _, e := range history.Events {
		fmt.Println(e.Time, e.Type, e.Summary)
	}
}
