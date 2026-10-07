package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Panels.Status.Show || cfg.Refresh.Debounce != 150*time.Millisecond || cfg.Keys.Quit[0] != "q" {
		t.Errorf("defaults not applied: %+v", cfg)
	}
}

func TestLoadOverridesOnlyWhatIsSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(path, []byte("theme:\n  branch: \"#ff8800\"\nrefresh:\n  poll: 0s\n"), 0o644)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.Branch != "#ff8800" || cfg.Refresh.Poll != 0 {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if cfg.Theme.ActiveBorder != "2" || cfg.Refresh.Debounce != 150*time.Millisecond {
		t.Errorf("defaults lost: %+v", cfg)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(path, []byte("theme:\n  brnach: \"1\"\n"), 0o644)
	if _, err := Load(path); err == nil {
		t.Error("expected an error for a misspelled key")
	}
}

func TestLoadRejectsBadChoices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	os.WriteFile(path, []byte("panels:\n  files:\n    view: list\n"), 0o644)
	if _, err := Load(path); err == nil {
		t.Error("expected an error for view: list")
	}
}

func TestLoadRejectsBadOrder(t *testing.T) {
	for _, order := range []string{"[files, diff, files]", "[files, log]"} {
		path := filepath.Join(t.TempDir(), "config.yml")
		os.WriteFile(path, []byte("layout:\n  order: "+order+"\n"), 0o644)
		if _, err := Load(path); err == nil {
			t.Errorf("order %s: expected an error", order)
		}
	}
}
