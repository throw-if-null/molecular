package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Missing(t *testing.T) {
	d, err := os.MkdirTemp("", "molecular-config-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	res := Load(d)
	if res.Found {
		t.Fatalf("expected not found")
	}
	if res.ParseError != nil {
		t.Fatalf("unexpected parse error: %v", res.ParseError)
	}
	// defaults
	def := Default()
	if res.Config.Retry.CarbonBudget != def.Retry.CarbonBudget {
		t.Fatalf("unexpected default carbon budget: %d", res.Config.Retry.CarbonBudget)
	}
}

func TestLoad_ValidOverrides(t *testing.T) {
	d, err := os.MkdirTemp("", "molecular-config-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)
	mm := filepath.Join(d, ".molecular")
	if err := os.Mkdir(mm, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(mm, "config.toml")
	content := `
[retry]
carbon_budget = 7
helium_budget = 8
review_budget = 9
`
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Load(d)
	if !res.Found {
		t.Fatalf("expected found true")
	}
	if res.ParseError != nil {
		t.Fatalf("unexpected parse error: %v", res.ParseError)
	}
	if res.Config.Retry.CarbonBudget != 7 {
		t.Fatalf("carbon budget not applied: %d", res.Config.Retry.CarbonBudget)
	}
}

func TestLoad_InvalidToml(t *testing.T) {
	d, err := os.MkdirTemp("", "molecular-config-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)
	mm := filepath.Join(d, ".molecular")
	if err := os.Mkdir(mm, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(mm, "config.toml")
	// invalid TOML
	if err := os.WriteFile(cfg, []byte("x = [1,\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Load(d)
	if !res.Found {
		t.Fatalf("expected found true")
	}
	if res.ParseError == nil {
		t.Fatalf("expected parse error")
	}
}
