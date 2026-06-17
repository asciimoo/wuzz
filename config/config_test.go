package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigDoesNotAliasDefaultKeys ensures that when a config file
// provides a Keys table but omits a category, the omitted category in the
// returned config is a copy of the default, not a reference to the shared
// DefaultKeys map. Otherwise mutating one loaded config corrupts the
// global defaults seen by every subsequent LoadConfig call.
func TestLoadConfigDoesNotAliasDefaultKeys(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	// A config that defines Keys but omits the "url" category.
	contents := "" +
		"[keys.global]\n" +
		"CtrlR = \"submit\"\n"
	if err := os.WriteFile(cfgPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	originalURLSubmit := DefaultKeys["url"]["Enter"]

	conf, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	// The omitted "url" category must be present (copied from defaults).
	if got := conf.Keys["url"]["Enter"]; got != originalURLSubmit {
		t.Fatalf("expected url.Enter %q, got %q", originalURLSubmit, got)
	}

	// Mutating the returned config's "url" category must not leak into the
	// shared DefaultKeys map.
	conf.Keys["url"]["Enter"] = "mutated"

	if DefaultKeys["url"]["Enter"] != originalURLSubmit {
		t.Fatalf("DefaultKeys was mutated via the loaded config: url.Enter = %q, expected %q",
			DefaultKeys["url"]["Enter"], originalURLSubmit)
	}
}
