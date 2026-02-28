package config

import (
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
        display: "icons/nextcloud.png"
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
	if tile.Display != "icons/nextcloud.png" {
		t.Errorf("tile display = %q, want %q", tile.Display, "icons/nextcloud.png")
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
	if r1.Match.Output != "maintenance.*true" {
		t.Errorf("rule[1] output = %q, want %q", r1.Match.Output, "maintenance.*true")
	}

	// Third rule: catch-all (empty match)
	r2 := slot.Rules[2]
	if r2.Match.Code != nil || r2.Match.Output != "" {
		t.Errorf("rule[2] should be catch-all, got code=%v output=%q", r2.Match.Code, r2.Match.Output)
	}
}

func TestParse_DefaultTheme(t *testing.T) {
	yaml := `
title: "Test"
groups:
  - name: "G"
    tiles:
      - name: "T"
        display: "icon.png"
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
        display: "icon.png"
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

func TestParse_DefaultStatus(t *testing.T) {
	yaml := `
title: "Test"
groups:
  - name: "G"
    tiles:
      - name: "T"
        display: "icon.png"
        slots:
          - name: "s"
            default_status: { id: "err", label: "💥" }
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

	slot := cfg.Groups[0].Tiles[0].Slots[0]
	if slot.DefaultStatus == nil {
		t.Fatal("default_status is nil")
	}
	if slot.DefaultStatus.ID != "err" {
		t.Errorf("default_status.id = %q, want %q", slot.DefaultStatus.ID, "err")
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
			yaml:    `groups: [{name: "G", tiles: [{name: "T", display: "x", slots: [{name: "s", check: {type: "command", target: "echo"}, rules: [{match: {}, status: {id: "ok", label: "✅"}}]}]}]}]`,
			wantErr: "title is required",
		},
		{
			name: "invalid theme",
			yaml: "title: \"T\"\ntheme: \"blue\"\ngroups: [{name: \"G\", tiles: [{name: \"T\", display: \"x\", slots: [{name: \"s\", check: {type: \"command\", target: \"echo\"}, rules: [{match: {}, status: {id: \"ok\", label: \"ok\"}}]}]}]}]",
			wantErr: "theme must be",
		},
		{
			name:    "no groups",
			yaml:    `title: "T"`,
			wantErr: "at least one group is required",
		},
		{
			name: "missing tile name",
			yaml: "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - display: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"command\", target: \"echo\"}\n            rules: [{match: {}, status: {id: \"ok\", label: \"ok\"}}]",
			wantErr: "name is required",
		},
		{
			name:    "invalid check type",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        display: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"ftp\", target: \"x\"}\n            rules: [{match: {}, status: {id: \"ok\", label: \"✅\"}}]",
			wantErr: "check.type must be",
		},
		{
			name:    "missing check target",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        display: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"http\"}\n            rules: [{match: {}, status: {id: \"ok\", label: \"✅\"}}]",
			wantErr: "check.target is required",
		},
		{
			name:    "no rules",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        display: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"command\", target: \"echo\"}",
			wantErr: "at least one rule is required",
		},
		{
			name:    "missing status id in rule",
			yaml:    "title: \"T\"\ngroups:\n  - name: \"G\"\n    tiles:\n      - name: \"T\"\n        display: \"x\"\n        slots:\n          - name: \"s\"\n            check: {type: \"command\", target: \"echo\"}\n            rules: [{match: {}, status: {label: \"✅\"}}]",
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
