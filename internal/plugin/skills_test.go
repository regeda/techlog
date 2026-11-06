package plugin_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestEverySkillIsWellFormed(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(pluginDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("plugin/skills is empty")
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d := readSkill(t, e.Name())
		if d.Name != e.Name() {
			t.Errorf("skills/%s: front matter name is %q, want %q", e.Name(), d.Name, e.Name())
		}
		if d.Description == "" || len(d.Description) > 1024 {
			t.Errorf("skills/%s: description must be 1 to 1024 characters, got %d", e.Name(), len(d.Description))
		}
	}
}

// Pinning a model keeps a simple skill from spending an expensive session model's
// tokens. A skill left out of the table keeps the session model on purpose.
func TestSkillModelPins(t *testing.T) {
	for dir, want := range map[string]string{
		"problem":  "haiku",
		"welcome":  "haiku",
		"proposal": "",
		"record":   "",
	} {
		if got := readSkill(t, dir).Model; got != want {
			t.Errorf("skills/%s: front matter model is %q, want %q", dir, got, want)
		}
	}
}

// A skill that names a plugin file that does not exist only fails when a user
// runs it, so every ${CLAUDE_PLUGIN_ROOT}/... reference must resolve.
func TestSkillsReferenceExistingPluginFiles(t *testing.T) {
	ref := regexp.MustCompile(`\$\{CLAUDE_PLUGIN_ROOT\}/([A-Za-z0-9_./-]+)`)
	paths, err := filepath.Glob(filepath.Join(pluginDir, "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec // G304: p comes from a glob of the plugin directory
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range ref.FindAllStringSubmatch(string(b), -1) {
			rel := strings.TrimRight(m[1], ".,")
			if _, err := os.Stat(filepath.Join(pluginDir, rel)); err != nil { //nolint:gosec // G703: rel is read from the plugin's own skill files
				t.Errorf("%s references ${CLAUDE_PLUGIN_ROOT}/%s, which does not exist", p, rel)
			}
		}
	}
}

func TestProblemSkill(t *testing.T) {
	d := readSkill(t, "problem")
	mustContain(t, "problem description", d.Description,
		"create a Jira ticket", "file a ticket", "log a bug", "problem.reported")
	mustContain(t, "problem skill", d.Body,
		"${CLAUDE_PLUGIN_ROOT}/config/mappings.yaml",
		"${CLAUDE_PLUGIN_ROOT}/config/problem-tree.yaml",
		".claude/techlog.json", "~/.claude/techlog.json", "malformed",
		"/techlog:welcome", "techlog:record",
		"problem.reported", "jira://", "280", "techlog_search_events",
		"Do not call any Jira or Atlassian tool")
	// It must never reach for a Jira or Atlassian tool by name.
	mustNotContain(t, "problem skill", d.Body, "mcp__", "createJiraIssue")
	// A key is not a creation time: reading the time would mean fetching the ticket.
	mustContain(t, "problem skill", d.Body, "Do not fetch the ticket", "If the server rejects the event")
	mustNotContain(t, "problem skill", d.Body, "ticket's creation time")
}

func TestProposalSkill(t *testing.T) {
	d := readSkill(t, "proposal")
	mustContain(t, "proposal description", d.Description, "proposal", "proposal.recorded")
	mustContain(t, "proposal skill", d.Body,
		"${CLAUDE_PLUGIN_ROOT}/config/mappings.yaml",
		".claude/techlog.json", "~/.claude/techlog.json", "malformed",
		"/techlog:welcome", "/techlog:problem", "techlog:record",
		"techlog_get_system_history", "techlog_search_events", "techlog_get_event",
		"context note", "motivated_by", "proposal.recorded", `"kind": "spec"`,
		"280", "writing-plans")
	// Events are immutable, so the user must see exactly what is recorded.
	mustContain(t, "proposal skill", d.Body,
		"Record nothing until the user says yes",
		"Record exactly one event",
		"If the server rejects the event")
}

// The decision rule only works if both skills read the stage and hand it on.
func TestSkillsCarryTheDecisionRule(t *testing.T) {
	for _, name := range []string{"problem", "proposal"} {
		d := readSkill(t, name)
		mustContain(t, name+" skill", d.Body,
			"stages.decision",
			"no decision rule is configured",
			"Skills held back until the decision is recorded")
	}
	p := readSkill(t, "proposal")
	// The proposal still records one event; the decision comes later, on request.
	mustContain(t, "proposal skill", p.Body,
		"the decision is recorded first",
		"Record exactly one event",
		"do not also record a decision")
}

// The mapped skill runs in the same conversation, so it already holds the framing,
// the rule, the tree and the note. The args point at them instead of writing them out again.
func TestSkillsPassPointersNotConfigText(t *testing.T) {
	for _, name := range []string{"problem", "proposal"} {
		d := readSkill(t, name)
		mustContain(t, name+" skill", d.Body,
			"the `framing` text", "stages.decision",
			"Skills held back until the decision is recorded")
		mustNotContain(t, name+" skill", d.Body, "the `decision` `framing`, followed by the line")
	}
	// The framing says the caller passes the tree or the note; the pointer has to say
	// that what it read above is that tree or note, or a small model walks nothing.
	mustContain(t, "problem skill", readSkill(t, "problem").Body,
		"walk the tree you read above", "the tree the framing says the caller passes", "from its first node")
	mustNotContain(t, "problem skill", readSkill(t, "problem").Body, "then the whole tree")
	mustContain(t, "proposal skill", readSkill(t, "proposal").Body,
		"the confirmed context note above", "the note the framing says the caller passes")
}

// A description that tells the model what the skill does invites it to follow the
// description and skip the body, so each one only says when to use the skill.
func TestDescriptionsStateWhenNotWhat(t *testing.T) {
	for _, name := range []string{"problem", "proposal", "record", "welcome"} {
		d := readSkill(t, name).Description
		if !strings.HasPrefix(d, "Use when") {
			t.Errorf("skills/%s: description must start with %q, got %q", name, "Use when", d)
		}
		mustNotContain(t, name+" description", d,
			"then records", "Records through", "Narrows", "Collects context",
			"runs the configured", "Takes a free-text", "Never creates a Jira ticket")
	}
}

func TestWelcomeSkill(t *testing.T) {
	d := readSkill(t, "welcome")
	mustContain(t, "welcome description", d.Description, "help", "use <plugin>")
	mustContain(t, "welcome skill", d.Body,
		"## No arguments", "## `use <plugin>`", "## Anything else",
		"${CLAUDE_PLUGIN_ROOT}/config/mappings.yaml",
		"${CLAUDE_PLUGIN_ROOT}/docs/",
		".claude/techlog.json", "keep any other keys", "change nothing",
		"techlog_search_events", "https://techlog-e7k.pages.dev/docs",
		"Never start, stop or repair the Techlog server")
}

func TestHelpPagesExistAndAnswerTheBasics(t *testing.T) {
	for _, page := range []string{"connect-mcp", "skills", "configuration", "lifecycle"} {
		s := readPlugin(t, "docs/"+page+".md")
		if !strings.HasPrefix(s, "# ") || len(s) < 200 {
			t.Errorf("docs/%s.md needs a title and real content", page)
		}
	}
	mustContain(t, "connect-mcp.md", readPlugin(t, "docs/connect-mcp.md"),
		"claude mcp add --transport http techlog", "/mcp", "/techlog:welcome")
	mustContain(t, "configuration.md", readPlugin(t, "docs/configuration.md"),
		".claude/techlog.json", "mappings.yaml", "problem-tree.yaml", "/techlog:welcome use",
		"decision", "before", "not enforced")
	mustContain(t, "skills.md", readPlugin(t, "docs/skills.md"),
		"/techlog:problem", "/techlog:proposal", "/techlog:welcome", "decision.made", "skip")
	mustContain(t, "lifecycle.md", readPlugin(t, "docs/lifecycle.md"),
		"decision.made", "writing-plans")
}

// The proposal skill and the decision rule own proposal.recorded and decision.made.
// If record also said when to write them, it could write either one on its own.
func TestRecordSkillLeavesProposalAndDecisionToTheirOwners(t *testing.T) {
	d := readSkill(t, "record")
	mustNotContain(t, "record skill", d.Body,
		"`brainstorming`: design or spec written",
		"`brainstorming`: user approves the written spec",
		"user approves the design")
	mustContain(t, "record skill", d.Body,
		"systematic-debugging", "verification-before-completion", "finishing-a-development-branch",
		"owned by")
}

// The record body loads on every record call. What a call rarely needs lives in
// references/ and is read on demand, so the body must name each file it relies on.
func TestRecordSkillBodyIsSmall(t *testing.T) {
	const limit = 8704
	d := readSkill(t, "record")
	if len(d.Body) > limit {
		t.Errorf("record skill body is %d bytes, want at most %d", len(d.Body), limit)
	}
	mustContain(t, "record skill", d.Body,
		"skills/record/references/examples.md", "skills/record/references/errors.md",
		"isError: true", "Already recorded", "idempotency_key",
		"/techlog:welcome how to connect techlog mcp")
	mustNotContain(t, "record skill", d.Body, "claude mcp add")
}

// A check's result or an action's state has no field of its own: the record
// skill tells the model to put it in the summary.
func TestRecordSkillSaysResultsGoInSummary(t *testing.T) {
	mustContain(t, "record skill", readSkill(t, "record").Body, "goes in the `summary`")
}
