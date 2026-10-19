package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteToURL(t *testing.T) {
	tests := map[string]string{
		"git@github.com:taptapdan/kansha-go.git":     "https://github.com/taptapdan/kansha-go",
		"https://github.com/taptapdan/kansha-go.git": "https://github.com/taptapdan/kansha-go",
		"ssh://git@github.com/taptapdan/kansha-go":   "https://github.com/taptapdan/kansha-go",
		"/some/local/path":                           "",
	}
	for in, want := range tests {
		if got := RemoteToURL(in); got != want {
			t.Errorf("RemoteToURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExcluded(t *testing.T) {
	c := &Config{Exclude: []string{"internal/scratch/"}}
	if !c.Excluded("internal/scratch") || !c.Excluded("internal/scratch/deep") {
		t.Error("expected internal/scratch to be excluded")
	}
	if c.Excluded("internal/scratchpad") {
		t.Error("internal/scratchpad should not be excluded")
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != "_site" || cfg.Branch != "main" {
		t.Errorf("defaults: output=%q branch=%q", cfg.Output, cfg.Branch)
	}
}

func TestLoadOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("output: public\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != "public" {
		t.Errorf("output = %q, want public", cfg.Output)
	}
}

func TestShowUnexported(t *testing.T) {
	for _, yml := range []string{"show_unexported: true\n", "unexported: true\n"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte(yml), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.ShowUnexported {
			t.Errorf("%q: ShowUnexported = false, want true", yml)
		}
	}
}
