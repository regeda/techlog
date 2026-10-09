package event_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

const skillPath = "../../skills/techlog-record/SKILL.md"

func TestSkillFrontMatter(t *testing.T) {
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "---\nname: techlog-record\ndescription: ") {
		t.Fatalf("SKILL.md must start with front matter naming the skill and describing when to use it:\n%.200s", s)
	}
}

// Every ```json event block in the skill is an event as a model would send it
// (without an id). Each must validate once the server's id is added.
func TestSkillExamplesAreValid(t *testing.T) {
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```json event\n(.*?)\n```").FindAllStringSubmatch(string(b), -1)
	if len(blocks) < 2 {
		t.Fatalf("expected at least two ```json event examples in SKILL.md, found %d", len(blocks))
	}
	v := newValidator(t)
	for i, m := range blocks {
		var doc map[string]any
		if err := json.Unmarshal([]byte(m[1]), &doc); err != nil {
			t.Fatalf("example %d is not valid JSON: %v", i+1, err)
		}
		if _, ok := doc["id"]; ok {
			t.Errorf("example %d must not contain an id: the server assigns it", i+1)
		}
		doc["id"] = "evt_000000000000"
		if err := v.Validate(doc); err != nil {
			t.Errorf("example %d is invalid: %v", i+1, err)
		}
	}
}

// The skill works through MCP only and assumes a running server: it must not
// carry HTTP calls or server lifecycle instructions.
func TestSkillUsesMCPOnlyAndIgnoresServerLifecycle(t *testing.T) {
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "claude mcp add --transport http techlog") {
		t.Error("SKILL.md should tell the user how to configure the MCP server")
	}
	for _, banned := range []string{"curl", "/api/v1", "event.id", "localhost", "127.0.0.1", "lsof", "techlog serve", "make flush", "make reset"} {
		if strings.Contains(s, banned) {
			t.Errorf("SKILL.md must not contain %q: it uses MCP only and does not manage the server", banned)
		}
	}
}
