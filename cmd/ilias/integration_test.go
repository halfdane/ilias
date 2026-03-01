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

// TestTestdata_GenerateAndValidate regenerates the HTML files in testdata/ from
// their YAML configs, then validates that key structural elements from the
// config appear in the generated HTML. This keeps the checked-in HTML in sync
// and catches regressions in the config→HTML pipeline.
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
		htmlPath := filepath.Join(testdataDir, base+".html")

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

			// Write the generated HTML to testdata/.
			if err := os.WriteFile(htmlPath, html, 0644); err != nil {
				t.Fatalf("writing %s: %v", htmlPath, err)
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
