package plugin_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const sitePage = "../../website/src/pages/docs/claude-plugin.html"

func readSitePage(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(sitePage)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The website describes the plugin. The website tests run in a sandbox that
// cannot read the plugin, so this test keeps the page honest from this side:
// every skill and every mapping stage the plugin ships must appear on it.
func TestClaudePluginPageCoversThePlugin(t *testing.T) {
	page := readSitePage(t)
	mustContain(t, "claude-plugin page", page, "layout: docs", "section: Integrate")

	entries, err := os.ReadDir(filepath.Join(pluginDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			mustContain(t, "claude-plugin page", page, "/techlog:"+e.Name())
		}
	}

	var m map[string]devPlugin
	decodeStrict(t, "config/mappings.yaml", &m)
	for name, p := range m {
		mustContain(t, "claude-plugin page", page, "<code>"+name+"</code>")
		for stage, s := range p.Stages {
			mustContain(t, "claude-plugin page", page, "<em>"+stage+"</em>", s.Skill)
			for _, b := range s.Before {
				mustContain(t, "claude-plugin page", page, b)
			}
		}
	}

	// The events the journey records.
	mustContain(t, "claude-plugin page", page,
		"problem.reported", "proposal.recorded", "decision.made", "change.implemented")

	// A website reader installs from the GitHub repo, not from a checkout.
	mustContain(t, "claude-plugin page", page, "https://github.com/regeda/techlog")
	mustNotContain(t, "claude-plugin page", page, "make plugin", "from a checkout")
}

// Skills are typed with a slash everywhere on the page, as a reader types them.
func TestClaudePluginPageNamesSkillsWithASlash(t *testing.T) {
	page := readSitePage(t)
	bare := regexp.MustCompile(`(^|[^/])techlog:(problem|proposal|welcome|record)`)
	if m := bare.FindString(page); m != "" {
		t.Errorf("the page names a skill without its slash: %q", m)
	}
}

// The pipeline is told as a journey that starts at /techlog:problem and uses
// /techlog:record at any step; the mapping internals have no section of their own.
func TestClaudePluginPageTellsTheJourney(t *testing.T) {
	page := readSitePage(t)
	mustContain(t, "claude-plugin page", page,
		"problem solvers", "<ol>", "/techlog:problem", "wherever you want to record a step")
	mustNotContain(t, "claude-plugin page", page, "How the mapping works", `id="mapping"`)
}

// The page names only the superpowers skills the plugin integrates: the ones the
// mapping points at or holds back. Others would suggest an integration that is
// not there.
func TestClaudePluginPageNamesOnlyIntegratedSkills(t *testing.T) {
	page := readSitePage(t)
	var m map[string]devPlugin
	decodeStrict(t, "config/mappings.yaml", &m)
	allowed := map[string]bool{}
	for _, p := range m {
		for _, s := range p.Stages {
			allowed[s.Skill] = true
			for _, b := range s.Before {
				allowed[b] = true
			}
		}
	}
	for _, ref := range regexp.MustCompile(`superpowers:[a-z-]+`).FindAllString(page, -1) {
		if !allowed[ref] {
			t.Errorf("the page names %q, which the mapping does not integrate", ref)
		}
	}
	mustNotContain(t, "claude-plugin page", page,
		"executing-plans", "subagent-driven-development", "finishing-a-development-branch")
}

// The journey stays short, and the switch section is gone.
func TestClaudePluginPageStaysShort(t *testing.T) {
	page := readSitePage(t)
	_, rest, ok := strings.Cut(page, `<h2 id="pipeline">`)
	if !ok {
		t.Fatal("the page has no journey section")
	}
	journey, _, _ := strings.Cut(rest, "<h2 ")
	if len(journey) > 1700 {
		t.Errorf("the journey is %d bytes; keep it under 1700", len(journey))
	}
	mustNotContain(t, "claude-plugin page", page, `id="switch"`, "Switch or add a development plugin")
}
