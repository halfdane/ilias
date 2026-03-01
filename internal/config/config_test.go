package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_ValidConfig(t *testing.T) {
	yaml := `
title: "My Dashboard"
theme: "dark"
groups:
  - name: "Infrastructure"
    tiles:
      - name: "Nextcloud"
        icon: "icons/nextcloud.png"
        link: "https://cloud.example.com"
        generate:
          command: "screenshot https://cloud.example.com -o /tmp/nc.png"
          timeout: "30s"
        slots:
          - name: "availability"
            check:
              type: "http"
              target: "https://cloud.example.com/status.php"
              timeout: "10s"
            rules:
              - match:
                  code: 200
                status:
                  id: "ok"
                  label: "✅"
              - match:
                  output: "maintenance.*true"
                status:
                  id: "warn"
                  label: "⚠️"
              - match: {}
                status:
                  id: "unknown"
                  label: "❓"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Title != "My Dashboard" {
		t.Errorf("title = %q, want %q", cfg.Title, "My Dashboard")
	}
	if cfg.Theme != "dark" {
		t.Errorf("theme = %q, want %q", cfg.Theme, "dark")
	}
	if len(cfg.Groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(cfg.Groups))
	}

	g := cfg.Groups[0]
	if g.Name != "Infrastructure" {
		t.Errorf("group name = %q, want %q", g.Name, "Infrastructure")
	}
	if len(g.Tiles) != 1 {
		t.Fatalf("len(tiles) = %d, want 1", len(g.Tiles))
	}

	tile := g.Tiles[0]
	if tile.Name != "Nextcloud" {
		t.Errorf("tile name = %q, want %q", tile.Name, "Nextcloud")
	}
	if tile.Icon != "icons/nextcloud.png" {
		t.Errorf("tile icon = %q, want %q", tile.Icon, "icons/nextcloud.png")
	}
	if tile.Link != "https://cloud.example.com" {
		t.Errorf("tile link = %q, want %q", tile.Link, "https://cloud.example.com")
	}
	if tile.Generate == nil {
		t.Fatal("tile generate is nil")
	}
	if tile.Generate.Timeout.Seconds() != 30 {
		t.Errorf("generate timeout = %v, want 30s", tile.Generate.Timeout)
	}

	if len(tile.Slots) != 1 {
		t.Fatalf("len(slots) = %d, want 1", len(tile.Slots))
	}

	slot := tile.Slots[0]
	if slot.Name != "availability" {
		t.Errorf("slot name = %q, want %q", slot.Name, "availability")
	}
	if slot.Check.Type != "http" {
		t.Errorf("check type = %q, want %q", slot.Check.Type, "http")
	}
	if slot.Check.Timeout.Seconds() != 10 {
		t.Errorf("check timeout = %v, want 10s", slot.Check.Timeout)
	}

	if len(slot.Rules) != 3 {
		t.Fatalf("len(rules) = %d, want 3", len(slot.Rules))
	}

	// First rule: exact code match
	r0 := slot.Rules[0]
	if r0.Match.Code == nil || r0.Match.Code.Exact == nil || *r0.Match.Code.Exact != 200 {
		t.Errorf("rule[0] code match: want exact 200")
	}
	if r0.Status.ID != "ok" {
		t.Errorf("rule[0] status.id = %q, want %q", r0.Status.ID, "ok")
	}

	// Second rule: output regex match
	r1 := slot.Rules[1]
	if r1.Match.Output == nil || !r1.Match.Output.MatchString(`{"maintenance": true}`) {
		t.Errorf("rule[1] output regex should match maintenance text, got %v", r1.Match.Output)
	}

	// Third rule: catch-all (empty match)
	r2 := slot.Rules[2]
	if r2.Match.Code != nil || r2.Match.Output != nil {
		t.Errorf("rule[2] should be catch-all, got code=%v output=%v", r2.Match.Code, r2.Match.Output)
	}
}

func TestParse_DefaultTheme(t *testing.T) {
	yaml := `
title: "Test"
groups:
  - name: "G"
    tiles:
      - name: "T"
        icon: "icon.png"
        slots:
          - name: "s"
            check:
              type: "command"
              target: "echo ok"
            rules:
              - match: {}
                status: { id: "ok", label: "✅" }
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Theme != "dark" {
		t.Errorf("default theme = %q, want %q", cfg.Theme, "dark")
	}
}

func TestParse_RegexCodeMatch(t *testing.T) {
	yaml := `
title: "Test"
groups:
  - name: "G"
    tiles:
      - name: "T"
        icon: "icon.png"
        slots:
          - name: "s"
            check:
              type: "http"
              target: "https://example.com"
            rules:
              - match:
                  code: "5\\d\\d"
                status: { id: "error", label: "❌" }
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := cfg.Groups[0].Tiles[0].Slots[0].Rules[0]
	if rule.Match.Code == nil || rule.Match.Code.Regex == nil {
		t.Fatal("expected regex code match")
	}
	if !rule.Match.Code.Regex.MatchString("503") {
		t.Error("regex should match 503")
	}
	if rule.Match.Code.Regex.MatchString("200") {
		t.Error("regex should not match 200")
	}
}

func TestParse_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing title",
			yaml:    `groups: [{name: "G", tiles: [{name: "T", icon: "x", slots: [{name: "s", check: {type: "command", target: "echo"}, rules: [{match: {}, status: {id: "ok", label: "✅"}}]}]}]}]`,
			wantErr: "title is required",
		},
		{
			name: "invalid theme",
			yaml: "title: \"T\"\ntheme: \"blue\"\ngroups: [{name: \"G\", tiles: [{name: \"T\", icon: \"x\", slots: [{name: \"s\", check: {type: \"command\", target: \"echo\"}, rules: [{match: {}, status: {id: \"ok\", label: \"ok\"}}]}]}]}]",
			wantErr: "theme must be",
		},
		{
			name:    "no groups",
			yaml:    `title: "T"`,
			wantErr: "at least one group is required",
		},
		{
			name: "missing tile name",
			yaml: "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - icon: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"command\", target: \"echo\"}\n            rules: [{match: {}, status: {id: \"ok\", label: \"ok\"}}]",
			wantErr: "name is required",
		},
		{
			name:    "invalid check type",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        icon: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"ftp\", target: \"x\"}\n            rules: [{match: {}, status: {id: \"ok\", label: \"✅\"}}]",
			wantErr: "check.type must be",
		},
		{
			name:    "missing check target",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        icon: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"http\"}\n            rules: [{match: {}, status: {id: \"ok\", label: \"✅\"}}]",
			wantErr: "check.target is required",
		},
		{
			name:    "no rules",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        icon: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"command\", target: \"echo\"}",
			wantErr: "at least one rule is required",
		},
		{
			name:    "missing status id in rule",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        icon: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"command\", target: \"echo\"}\n            rules: [{match: {}, status: {label: \"✅\"}}]",
			wantErr: "status.id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestParse_TestdataFiles ensures all YAML files in testdata/ parse and validate
// successfully. This catches regressions in the example configs (including the
// one embedded into the README via embedmd).
func TestParse_TestdataFiles(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("reading testdata dir: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(testdataDir, e.Name()))
			if err != nil {
				t.Fatalf("reading file: %v", err)
			}
			_, err = Parse(data)
			if err != nil {
				t.Errorf("parse/validate failed: %v", err)
			}
		})
	}
}

func TestParse_DefaultRules_Applied(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: { code: 0 }
      status: { id: ok, label: "✅" }
    - match: {}
      status: { id: error, label: "❌" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: { type: command, target: "echo ok" }
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if len(slot.Rules) != 2 {
		t.Fatalf("expected 2 rules from defaults, got %d", len(slot.Rules))
	}
	if slot.Rules[0].Status.ID != "ok" {
		t.Errorf("rule[0].status.id = %q, want %q", slot.Rules[0].Status.ID, "ok")
	}
	if slot.Rules[1].Status.ID != "error" {
		t.Errorf("rule[1].status.id = %q, want %q", slot.Rules[1].Status.ID, "error")
	}
}

func TestParse_DefaultRules_OverriddenBySlot(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: { code: 0 }
      status: { id: ok, label: "✅" }
    - match: {}
      status: { id: error, label: "❌" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: { type: command, target: "echo ok" }
            rules:
              - match: { code: 42 }
                status: { id: custom, label: "🔧" }
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if len(slot.Rules) != 1 {
		t.Fatalf("expected 1 explicit rule, got %d", len(slot.Rules))
	}
	if slot.Rules[0].Status.ID != "custom" {
		t.Errorf("rule[0].status.id = %q, want %q", slot.Rules[0].Status.ID, "custom")
	}
}

func TestParse_NoDefaultRules_SlotWithoutRules_Error(t *testing.T) {
	yaml := `
title: "Test"
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: { type: command, target: "echo ok" }
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for slot without rules and no defaults")
	}
	if !strings.Contains(err.Error(), "at least one rule is required") {
		t.Errorf("error = %q, want to contain 'at least one rule is required'", err.Error())
	}
}

func TestParse_DefaultRules_Validated(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: {}
      status: { label: "✅" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: { type: command, target: "echo ok" }
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for default rule missing status.id")
	}
	if !strings.Contains(err.Error(), "defaults") {
		t.Errorf("error = %q, want to mention 'defaults'", err.Error())
	}
}

func TestParse_CheckStringShorthand_Command(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: {}
      status: { id: ok, label: "✅" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: "uptime"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if slot.Check.Type != "command" {
		t.Errorf("check.type = %q, want %q", slot.Check.Type, "command")
	}
	if slot.Check.Target != "uptime" {
		t.Errorf("check.target = %q, want %q", slot.Check.Target, "uptime")
	}
}

func TestParse_CheckStringShorthand_HTTP(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: {}
      status: { id: ok, label: "✅" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: "https://example.com"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if slot.Check.Type != "http" {
		t.Errorf("check.type = %q, want %q", slot.Check.Type, "http")
	}
	if slot.Check.Target != "https://example.com" {
		t.Errorf("check.target = %q, want %q", slot.Check.Target, "https://example.com")
	}
}

func TestParse_CheckMapWithoutType_InferCommand(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: {}
      status: { id: ok, label: "✅" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: { target: "echo ok" }
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if slot.Check.Type != "command" {
		t.Errorf("check.type = %q, want %q", slot.Check.Type, "command")
	}
}

func TestParse_CheckMapWithoutType_InferHTTP(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: {}
      status: { id: ok, label: "✅" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: { target: "http://localhost:8080" }
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if slot.Check.Type != "http" {
		t.Errorf("check.type = %q, want %q", slot.Check.Type, "http")
	}
}

func TestParse_CheckMapWithExplicitType_Unchanged(t *testing.T) {
	yaml := `
title: "Test"
defaults:
  rules:
    - match: {}
      status: { id: ok, label: "✅" }
groups:
  - name: "G"
    tiles:
      - name: "T"
        slots:
          - name: "s"
            check: { type: http, target: "https://example.com", timeout: "5s" }
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if slot.Check.Type != "http" {
		t.Errorf("check.type = %q, want %q", slot.Check.Type, "http")
	}
	if slot.Check.Timeout.Seconds() != 5 {
		t.Errorf("check.timeout = %v, want 5s", slot.Check.Timeout)
	}
}
