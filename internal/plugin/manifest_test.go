package plugin_test

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPluginManifest(t *testing.T) {
	var m struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Version     string `json:"version"`
	}
	if err := json.Unmarshal([]byte(readPlugin(t, ".claude-plugin/plugin.json")), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "techlog" {
		t.Errorf("plugin name = %q, want techlog", m.Name)
	}
	if m.Description == "" || m.Version == "" {
		t.Errorf("plugin.json needs a description and a version: %+v", m)
	}
}

func TestMarketplacePointsAtThePlugin(t *testing.T) {
	b, err := os.ReadFile("../../.claude-plugin/marketplace.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Name == "" {
		t.Error("marketplace.json needs a name")
	}
	if len(m.Plugins) != 1 || m.Plugins[0].Name != "techlog" || m.Plugins[0].Source != "./plugin" {
		t.Errorf("marketplace must list exactly the techlog plugin with source ./plugin, got %+v", m.Plugins)
	}
}
