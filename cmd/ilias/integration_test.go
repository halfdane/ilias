package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/halfdane/ilias/internal/config"
	"github.com/halfdane/ilias/internal/renderer"
	"github.com/halfdane/ilias/internal/runner"
)

// fixedTime is used for deterministic HTML generation so that the "Generated at"
// timestamp does not cause spurious diffs.
var fixedTime = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// deterministic lists configs whose check commands produce stable output.
// Only these have their HTML written to disk (and tracked in git).
var deterministic = map[string]bool{
	"basic": true,
}

// TestTestdata_GenerateAndValidate runs the full config→HTML pipeline for every
// YAML in testdata/. For deterministic configs (basic.yaml) it writes the HTML
// to disk so it can be embedded in the README. For non-deterministic configs
// (complex.yaml) it validates the HTML structurally without persisting it.
func TestTestdata_GenerateAndValidate(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("reading testdata dir: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".yaml")
		yamlPath := filepath.Join(testdataDir, e.Name())

		t.Run(base, func(t *testing.T) {
			// Parse config.
			cfg, err := config.Load(yamlPath)
			if err != nil {
				t.Fatalf("loading config %s: %v", e.Name(), err)
			}

			// Run checks and generate HTML.
			result, err := runner.Run(context.Background(), cfg, runner.Options{
				Logger: io.Discard,
			})
			if err != nil {
				t.Fatalf("running checks: %v", err)
			}

			html, err := renderer.Render(result, testdataDir, "test", fixedTime)
			if err != nil {
				t.Fatalf("rendering: %v", err)
			}

			// Only write to disk for deterministic configs.
			if deterministic[base] {
				htmlPath := filepath.Join(testdataDir, base+".html")
				if err := os.WriteFile(htmlPath, html, 0644); err != nil {
					t.Fatalf("writing %s: %v", htmlPath, err)
				}
			}

			// Validate structural elements.
			output := string(html)

			if !strings.Contains(output, cfg.Title) {
				t.Errorf("HTML missing title %q", cfg.Title)
			}
			for _, g := range cfg.Groups {
				if !strings.Contains(output, g.Name) {
					t.Errorf("HTML missing group name %q", g.Name)
				}
				for _, tile := range g.Tiles {
					if !strings.Contains(output, tile.Name) {
						t.Errorf("HTML missing tile name %q (group %q)", tile.Name, g.Name)
					}
				}
			}
		})
	}
}
