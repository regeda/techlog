package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(context.Background(), args, &out)
	return out.String(), err
}

func TestRunErrors(t *testing.T) {
	if _, err := runCmd(t); err == nil || !strings.Contains(err.Error(), "missing command") {
		t.Errorf("no args: %v", err)
	}
	if _, err := runCmd(t, "bogus"); err == nil || !strings.Contains(err.Error(), `unknown command "bogus"`) {
		t.Errorf("bogus: %v", err)
	}
	dir := t.TempDir()
	if _, err := runCmd(t, "serve", "--store", "nope", "--db", filepath.Join(dir, "x.db")); err == nil || !strings.Contains(err.Error(), `unknown store "nope"`) {
		t.Errorf("unknown store: %v", err)
	}
}

func TestHelp(t *testing.T) {
	out, err := runCmd(t, "help")
	if err != nil || !strings.Contains(out, "techlog serve") || !strings.Contains(out, "techlog seed") {
		t.Fatalf("help = %q, %v", out, err)
	}
}

func TestSeedCommandIsRepeatable(t *testing.T) {
	db := filepath.Join(t.TempDir(), "share", "techlog.db")
	out, err := runCmd(t, "seed", "--db", db)
	if err != nil || !strings.Contains(out, "seeded 25 new events (0 already present)") {
		t.Fatalf("first seed = %q, %v", out, err)
	}
	out, err = runCmd(t, "seed", "--db", db)
	if err != nil || !strings.Contains(out, "seeded 0 new events (25 already present)") {
		t.Fatalf("second seed = %q, %v", out, err)
	}
}

func TestServeRejectsNegativeClockSkew(t *testing.T) {
	db := filepath.Join(t.TempDir(), "x.db")
	_, err := runCmd(t, "serve", "--db", db, "--clock-skew", "-1s")
	if err == nil || !strings.Contains(err.Error(), "clock skew") {
		t.Fatalf("negative skew: %v", err)
	}
}
