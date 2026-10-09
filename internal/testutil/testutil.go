// Package testutil holds helpers shared by the tests of several packages.
package testutil

import (
	"testing"

	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store/sqlite"
)

// NewService returns a Service backed by a fresh in-memory SQLite database.
func NewService(t testing.TB) *service.Service {
	t.Helper()
	st, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc, err := service.New(st)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// Doc returns a valid event document without an id or provenance (Record
// supplies both). Callers may overwrite any key, for example "time".
func Doc(system, typ, summary string, links ...map[string]any) map[string]any {
	d := map[string]any{
		"type":    typ,
		"time":    "2026-03-10T09:00:00Z",
		"actor":   map[string]any{"kind": "human", "id": "user:test"},
		"subject": map[string]any{"system": system},
		"status":  "recorded",
		"summary": summary,
	}
	if len(links) > 0 {
		arr := make([]any, len(links))
		for i, l := range links {
			arr[i] = l
		}
		d["links"] = arr
	}
	return d
}

// Link returns a link object; the basis defaults to "confirmed".
func Link(rel, target string, basis ...string) map[string]any {
	b := "confirmed"
	if len(basis) > 0 {
		b = basis[0]
	}
	return map[string]any{"rel": rel, "target": target, "basis": b}
}
