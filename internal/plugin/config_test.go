package plugin_test

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/regeda/techlog/internal/event"
	"gopkg.in/yaml.v3"
)

func decodeStrict(t *testing.T, rel string, out any) {
	t.Helper()
	dec := yaml.NewDecoder(strings.NewReader(readPlugin(t, rel)))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
}

// ---- mappings ----

type stageMapping struct {
	Skill   string   `yaml:"skill"`
	Framing string   `yaml:"framing"`
	Before  []string `yaml:"before"`
}

type devPlugin struct {
	Stages map[string]stageMapping `yaml:"stages"`
}

func TestMappingsCoverTheMVPStages(t *testing.T) {
	var m map[string]devPlugin
	decodeStrict(t, "config/mappings.yaml", &m)
	sp, ok := m["superpowers"]
	if !ok {
		t.Fatal("mappings.yaml needs a block for the default plugin, superpowers")
	}
	skillRef := regexp.MustCompile(`^[a-z0-9-]+:[a-z0-9-]+$`)
	for name, p := range m {
		for _, stage := range []string{"problem", "proposal"} {
			s, ok := p.Stages[stage]
			if !ok {
				t.Errorf("%s: missing stage %q", name, stage)
				continue
			}
			if !skillRef.MatchString(s.Skill) {
				t.Errorf("%s.%s: skill %q must look like plugin:skill", name, stage, s.Skill)
			}
			if strings.TrimSpace(s.Framing) == "" {
				t.Errorf("%s.%s: framing is empty", name, stage)
			}
		}
	}
	mustContain(t, "problem framing", sp.Stages["problem"].Framing,
		"decision tree", "Do not propose solutions", "Do not write a spec")
	mustContain(t, "proposal framing", sp.Stages["proposal"].Framing,
		"context note", "rejected options", "Do not invoke writing-plans",
		"Do not implement", "architectural path", "spec file")
}

// The decision stage is a standing rule, not a step: it names the skills it
// holds back, and its text tells Claude to confirm before it records.
func TestMappingsDecisionStage(t *testing.T) {
	var m map[string]devPlugin
	decodeStrict(t, "config/mappings.yaml", &m)
	skillRef := regexp.MustCompile(`^[a-z0-9-]+:[a-z0-9-]+$`)
	for name, p := range m {
		s, ok := p.Stages["decision"]
		if !ok {
			t.Errorf("%s: missing stage %q", name, "decision")
			continue
		}
		if s.Skill != "techlog:record" {
			t.Errorf("%s.decision: skill is %q, want techlog:record, the writer of events", name, s.Skill)
		}
		if len(s.Before) == 0 {
			t.Errorf("%s.decision: before lists no skill, so the rule would hold nothing back", name)
		}
		for _, b := range s.Before {
			if !skillRef.MatchString(b) {
				t.Errorf("%s.decision: before entry %q must look like plugin:skill", name, b)
			}
		}
		mustContain(t, name+".decision framing", s.Framing,
			"standing rule", "decision.made", "decides", `"kind": "spec"`, "280",
			"Record nothing until the user says yes", "skip", "at most once",
			"unreachable", "rejects the event")
		// A decision is immutable: it needs a spec to point at, and the server
		// must be able to refuse a second one for the same spec.
		mustContain(t, name+".decision framing", s.Framing,
			"do not draft a decision", "/techlog:proposal first",
			"already decides", "idempotency_key")
		// The model has no clock: the decision leaves its time out and the draft
		// says the server sets it.
		mustContain(t, name+".decision framing", s.Framing,
			"Leave its time out", "time: set by the server when recorded")
	}
	if got := m["superpowers"].Stages["decision"].Before; !slices.Equal(got, []string{"superpowers:writing-plans"}) {
		t.Errorf("superpowers.decision.before = %v, want [superpowers:writing-plans]", got)
	}
}

// The rule asks Claude to build this event. It already passes today: the test
// pins that the shape the rule describes stays valid for the event schema.
func TestDecisionDraftIsValid(t *testing.T) {
	v, err := event.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	d := map[string]any{
		"schema_version":  "0.1-draft",
		"id":              "evt_000000000000",
		"idempotency_key": "decision-techlog-plugin-draft",
		"type":            "decision.made",
		"time":            "2026-10-14T12:36:11Z",
		"actor":           map[string]any{"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak"},
		"subject":         map[string]any{"system": "techlog-plugin", "component": "mappings.yaml"},
		"summary":         "Approved the spec to record decision.made before writing-plans through a standing rule in the mapping",
		"links":           []any{map[string]any{"rel": "decides", "target": "evt_MV83D3FGXJF7", "basis": "confirmed"}},
		"evidence":        []any{map[string]any{"kind": "spec", "ref": "docs/superpowers/specs/2026-10-14-decision-gate-design.md"}},
		"provenance":      map[string]any{"recorded_by": "agent:claude-code", "method": "agent_submitted"},
	}
	if err := v.Validate(d); err != nil {
		t.Errorf("a decision.made built the way the rule says is invalid: %v", err)
	}
}

// ---- problem tree ----

type option struct {
	Label string `yaml:"label"`
	Next  string `yaml:"next"`
}

type node struct {
	Ask      string   `yaml:"ask"`
	SkipIf   string   `yaml:"skip_if"`
	Next     string   `yaml:"next"`
	Sets     []string `yaml:"sets"`
	Options  []option `yaml:"options"`
	Optional bool     `yaml:"optional"`
	Confirm  bool     `yaml:"confirm"`
}

type tree struct {
	Nodes map[string]node `yaml:"nodes"`
	Start string          `yaml:"start"`
}

func (n node) successors() []string {
	var out []string
	if n.Next != "" {
		out = append(out, n.Next)
	}
	for _, o := range n.Options {
		out = append(out, o.Next)
	}
	return out
}

func loadTree(t *testing.T) tree {
	t.Helper()
	var tr tree
	decodeStrict(t, "config/problem-tree.yaml", &tr)
	return tr
}

var allowedSets = []string{"summary", "subject.system", "subject.component", "evidence", "links"}

func TestTreeShape(t *testing.T) {
	tr := loadTree(t)
	if _, ok := tr.Nodes[tr.Start]; !ok {
		t.Fatalf("start node %q does not exist", tr.Start)
	}
	confirms := 0
	for id, n := range tr.Nodes {
		checkNode(t, tr, id, n)
		if n.Confirm {
			confirms++
		}
	}
	if confirms != 1 {
		t.Errorf("the tree needs exactly one confirm node, has %d", confirms)
	}
}

// kinds counts how many of options, next and confirm the node has.
func (n node) kinds() int {
	kinds := 0
	if len(n.Options) > 0 {
		kinds++
	}
	if n.Next != "" {
		kinds++
	}
	if n.Confirm {
		kinds++
	}
	return kinds
}

func checkNode(t *testing.T, tr tree, id string, n node) {
	t.Helper()
	if strings.TrimSpace(n.Ask) == "" {
		t.Errorf("node %s: ask is empty", id)
	}
	if kinds := n.kinds(); kinds != 1 {
		t.Errorf("node %s must have exactly one of options, next, confirm; has %d", id, kinds)
	}
	if len(n.Options) == 1 {
		t.Errorf("node %s: a choice needs at least two options", id)
	}
	for _, o := range n.Options {
		if strings.TrimSpace(o.Label) == "" {
			t.Errorf("node %s: an option has no label", id)
		}
	}
	for _, next := range n.successors() {
		if _, ok := tr.Nodes[next]; !ok {
			t.Errorf("node %s points at %q, which does not exist", id, next)
		}
	}
	for _, s := range n.Sets {
		if !slices.Contains(allowedSets, s) {
			t.Errorf("node %s sets %q; allowed: %v", id, s, allowedSets)
		}
	}
}

// pathSets returns, for every route from the start to the confirm node, the
// union of what its nodes set. It fails the test on a cycle.
func pathSets(t *testing.T, tr tree) []map[string]bool {
	t.Helper()
	var out []map[string]bool
	var walk func(id string, onPath []string, sets map[string]bool)
	walk = func(id string, onPath []string, sets map[string]bool) {
		if slices.Contains(onPath, id) {
			t.Fatalf("the tree loops: %v -> %s", onPath, id)
		}
		n := tr.Nodes[id]
		next := maps.Clone(sets)
		for _, s := range n.Sets {
			next[s] = true
		}
		if n.Confirm {
			out = append(out, next)
			return
		}
		for _, s := range n.successors() {
			walk(s, append(slices.Clone(onPath), id), next)
		}
	}
	walk(tr.Start, nil, map[string]bool{})
	return out
}

func TestTreeIsAcyclicAndEveryNodeIsReachable(t *testing.T) {
	tr := loadTree(t)
	if len(pathSets(t, tr)) == 0 {
		t.Fatal("no path from start reaches the confirm node")
	}
	seen := map[string]bool{}
	var visit func(id string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		for _, s := range tr.Nodes[id].successors() {
			visit(s)
		}
	}
	visit(tr.Start)
	for id := range tr.Nodes {
		if !seen[id] {
			t.Errorf("node %s cannot be reached from %s", id, tr.Start)
		}
	}
}

func draftEvent(sets map[string]bool) map[string]any {
	subject := map[string]any{"system": "promo-api"}
	if sets["subject.component"] {
		subject["component"] = "spin-service"
	}
	d := map[string]any{
		"schema_version":  "0.1-draft",
		"id":              "evt_000000000000",
		"idempotency_key": "problem-promo-api-draft",
		"type":            "problem.reported",
		"time":            "2026-03-09T14:00:00Z",
		"actor":           map[string]any{"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak"},
		"subject":         subject,
		"summary":         "Spin endpoint returns 500 for new accounts, blocking sign-up promotions",
		"provenance":      map[string]any{"recorded_by": "agent:claude-code", "method": "agent_submitted"},
	}
	if sets["evidence"] {
		d["evidence"] = []any{map[string]any{"kind": "ticket", "ref": "jira://PROMO-123"}}
	}
	if sets["links"] {
		d["links"] = []any{map[string]any{"rel": "relates_to", "target": "evt_01J9K2A0P001", "basis": "confirmed"}}
	}
	return d
}

func TestEveryPathYieldsAValidEvent(t *testing.T) {
	v, err := event.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	for i, sets := range pathSets(t, loadTree(t)) {
		if !sets["subject.system"] || !sets["summary"] {
			t.Errorf("path %d collects %v: every path must set subject.system and summary", i, sets)
		}
		if err := v.Validate(draftEvent(sets)); err != nil {
			t.Errorf("path %d builds an invalid event: %v", i, err)
		}
	}
}
