package event_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/regeda/techlog/internal/event"
)

func validDoc() map[string]any {
	return map[string]any{
		"schema_version":  "0.1-draft",
		"id":              "evt_01J9K2A0P001",
		"idempotency_key": "support-desk-ticket-790",
		"type":            "problem.reported",
		"time":            "2026-03-09T14:00:00Z",
		"actor":           map[string]any{"kind": "human", "id": "user:t.nguyen"},
		"subject":         map[string]any{"system": "support-desk"},
		"status":          "recorded",
		"provenance":      map[string]any{"recorded_by": "user:t.nguyen", "method": "human_entered"},
	}
}

func problemPaths(err error) []string {
	var ve *event.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	paths := make([]string, len(ve.Problems))
	for i, p := range ve.Problems {
		paths[i] = p.Path
	}
	return paths
}

func newValidator(t *testing.T) *event.Validator {
	t.Helper()
	v, err := event.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestValidate(t *testing.T) {
	v := newValidator(t)
	cases := []struct {
		name     string
		mutate   func(d map[string]any)
		wantPath string // "" means the document must be valid
	}{
		{"valid", func(d map[string]any) {}, ""},
		{"offset time is valid", func(d map[string]any) { d["time"] = "2026-03-11T12:00:00+02:00" }, ""},
		{"integer extension values are valid", func(d map[string]any) {
			d["extensions"] = map[string]any{"org.techlog.code": map[string]any{"files_changed": 5}}
		}, ""},
		{"bad actor kind", func(d map[string]any) { d["actor"].(map[string]any)["kind"] = "robot" }, "/actor/kind"},
		{"missing subject", func(d map[string]any) { delete(d, "subject") }, "/"},
		{"time without zone", func(d map[string]any) { d["time"] = "2026-03-11T10:12:00" }, "/time"},
		{"summary too long", func(d map[string]any) { d["summary"] = strings.Repeat("x", 281) }, "/summary"},
		{"unknown top-level field", func(d map[string]any) { d["foo"] = "bar" }, "/"},
		{"missing idempotency_key", func(d map[string]any) { delete(d, "idempotency_key") }, "/"},
		{"empty idempotency_key", func(d map[string]any) { d["idempotency_key"] = "" }, "/idempotency_key"},
		{"idempotency_key too long", func(d map[string]any) { d["idempotency_key"] = strings.Repeat("k", 129) }, "/idempotency_key"},
		{"idempotency_key at the limit", func(d map[string]any) { d["idempotency_key"] = strings.Repeat("k", 128) }, ""},
		{"idempotency_key is not a string", func(d map[string]any) { d["idempotency_key"] = 7 }, "/idempotency_key"},
		{"bad link rel", func(d map[string]any) {
			d["links"] = []any{map[string]any{"rel": "nope", "target": "evt_01J9K2A0P002", "basis": "confirmed"}}
		}, "/links/0/rel"},
		{"bad evidence digest", func(d map[string]any) {
			d["evidence"] = []any{map[string]any{"kind": "report", "ref": "ci://1", "digest": "sha256:abc"}}
		}, "/evidence/0/digest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validDoc()
			tc.mutate(d)
			err := v.Validate(d)
			if tc.wantPath == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if !slices.Contains(problemPaths(err), tc.wantPath) {
				t.Fatalf("expected a problem at %q, got paths %v (err: %v)", tc.wantPath, problemPaths(err), err)
			}
		})
	}
}

func TestValidateRejectsBadExtensionKey(t *testing.T) {
	d := validDoc()
	d["extensions"] = map[string]any{"Bad Key": map[string]any{}}
	if err := newValidator(t).Validate(d); err == nil {
		t.Fatal("expected an error for a non-namespaced extension key")
	}
}

func TestValidateDoesNotMutateInput(t *testing.T) {
	d := validDoc()
	d["extensions"] = map[string]any{"org.techlog.x": map[string]any{"n": 1}}
	if err := newValidator(t).Validate(d); err != nil {
		t.Fatal(err)
	}
	if _, ok := d["extensions"].(map[string]any)["org.techlog.x"].(map[string]any)["n"].(int); !ok {
		t.Fatal("Validate must not rewrite the caller's document")
	}
}

func TestValidateLink(t *testing.T) {
	v := newValidator(t)
	ok := map[string]any{"rel": "relates_to", "target": "evt_01J9K2A0P002", "basis": "confirmed"}
	if err := v.ValidateLink(ok); err != nil {
		t.Fatalf("expected valid link, got %v", err)
	}
	if err := v.ValidateLink(map[string]any{"rel": "relates_to", "target": "evt_01J9K2A0P002"}); err == nil {
		t.Error("a link without basis must be rejected")
	}
	err := v.ValidateLink(map[string]any{"rel": "relates_to", "target": "nope", "basis": "confirmed"})
	if !slices.Contains(problemPaths(err), "/target") {
		t.Errorf("expected a problem at /target, got %v", problemPaths(err))
	}
}

func TestNormalizeNilDocument(t *testing.T) {
	out, err := event.Normalize(nil)
	if err != nil || out == nil {
		t.Fatalf("Normalize(nil) = %v, %v; want an empty non-nil map", out, err)
	}
}

func TestTypesMatchSchema(t *testing.T) {
	var s struct {
		Properties struct {
			Type struct {
				Enum []string `json:"enum"`
			} `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(event.SchemaJSON(), &s); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Properties.Type.Enum, event.Types) {
		t.Fatalf("event.Types is out of sync with the schema enum:\n got %v\nwant %v", event.Types, s.Properties.Type.Enum)
	}
}

func TestSchemaMatchesWebsiteCopy(t *testing.T) {
	site, err := os.ReadFile("../../website/src/examples/json-schema-core.json")
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(event.SchemaJSON(), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(site, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("internal/event/schema/event.json and website/src/examples/json-schema-core.json differ; keep them identical")
	}
}
