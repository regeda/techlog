package plugin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const pluginDir = "../../plugin"

func readPlugin(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(pluginDir, rel)) //nolint:gosec // G304: a fixed test fixture directory
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// skillDoc is a SKILL.md split into its front matter fields and its body.
type skillDoc struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Body        string `yaml:"-"`
}

func readSkill(t *testing.T, dir string) skillDoc {
	t.Helper()
	s := readPlugin(t, "skills/"+dir+"/SKILL.md")
	rest, ok := strings.CutPrefix(s, "---\n")
	if !ok {
		t.Fatalf("skills/%s/SKILL.md must start with front matter", dir)
	}
	head, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		t.Fatalf("skills/%s/SKILL.md front matter is not closed", dir)
	}
	var d skillDoc
	if err := yaml.Unmarshal([]byte(head), &d); err != nil {
		t.Fatalf("skills/%s/SKILL.md front matter is not valid YAML: %v", dir, err)
	}
	d.Body = body
	return d
}

func mustContain(t *testing.T, what, s string, phrases ...string) {
	t.Helper()
	for _, p := range phrases {
		if !strings.Contains(s, p) {
			t.Errorf("%s should contain %q", what, p)
		}
	}
}

func mustNotContain(t *testing.T, what, s string, phrases ...string) {
	t.Helper()
	for _, p := range phrases {
		if strings.Contains(s, p) {
			t.Errorf("%s must not contain %q", what, p)
		}
	}
}
