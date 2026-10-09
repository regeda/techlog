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

// `localhost` can resolve to IPv6 and reach another process on the same port,
// while the server listens on IPv4 only. The skill's commands must name
// 127.0.0.1, and it must tell the reader how to check who answers.
func TestSkillUsesTheLoopbackAddress(t *testing.T) {
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"http://127.0.0.1:8080/mcp",
		"http://127.0.0.1:8080/api/v1/events",
		"http://127.0.0.1:8080/api/v1/systems",
		"lsof -nP -iTCP:8080",
		"`event.id`",
		"make flush",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("SKILL.md should contain %q", want)
		}
	}
	if strings.Contains(s, "http://localhost:8080") {
		t.Error("commands in SKILL.md must use 127.0.0.1; the text may name localhost only to explain why")
	}
}
