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

			html, err := renderer.Render(result, testdataDir, "test", renderer.Options{GeneratedAt: fixedTime})
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

// TestNoTooltips verifies that --no-tooltips strips all data-tooltip attributes
// and check output from the generated HTML.
func TestNoTooltips(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata")
	cfg, err := config.Load(filepath.Join(testdataDir, "basic.yaml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	result, err := runner.Run(context.Background(), cfg, runner.Options{Logger: io.Discard})
	if err != nil {
		t.Fatalf("running checks: %v", err)
	}

	// Sanity check: default render has tooltips.
	withTooltips, err := renderer.Render(result, testdataDir, "test", renderer.Options{GeneratedAt: fixedTime})
	if err != nil {
		t.Fatalf("rendering with tooltips: %v", err)
	}
	if !strings.Contains(string(withTooltips), `data-tooltip="`) {
		t.Fatal("expected data-tooltip in default render (sanity check failed)")
	}

	// With --no-tooltips: no data-tooltip attribute and no raw output values.
	html, err := renderer.Render(result, testdataDir, "test", renderer.Options{
		GeneratedAt: fixedTime,
		NoTooltips:  true,
	})
	if err != nil {
		t.Fatalf("rendering without tooltips: %v", err)
	}
	output := string(html)

	if strings.Contains(output, `data-tooltip="`) {
		t.Error("HTML must not contain data-tooltip attributes when NoTooltips is set")
	}
	// basic.yaml runs "echo ok"; its stdout should not appear anywhere in the HTML.
	if strings.Contains(output, ">ok<") || strings.Contains(output, `"ok"`) {
		t.Error("HTML must not contain raw command output when NoTooltips is set")
	}
}

// TestNoTimestamp verifies that --no-timestamp omits the "Generated at" line
// from the generated HTML.
func TestNoTimestamp(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata")
	cfg, err := config.Load(filepath.Join(testdataDir, "basic.yaml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	result, err := runner.Run(context.Background(), cfg, runner.Options{Logger: io.Discard})
	if err != nil {
		t.Fatalf("running checks: %v", err)
	}

	formattedTime := fixedTime.Format("2006-01-02 15:04:05")

	// Sanity check: default render has the timestamp.
	withTS, err := renderer.Render(result, testdataDir, "test", renderer.Options{GeneratedAt: fixedTime})
	if err != nil {
		t.Fatalf("rendering with timestamp: %v", err)
	}
	if !strings.Contains(string(withTS), formattedTime) {
		t.Fatal("expected timestamp in default render (sanity check failed)")
	}

	// With --no-timestamp: timestamp must be absent.
	html, err := renderer.Render(result, testdataDir, "test", renderer.Options{
		GeneratedAt: fixedTime,
		NoTimestamp: true,
	})
	if err != nil {
		t.Fatalf("rendering without timestamp: %v", err)
	}
	if strings.Contains(string(html), formattedTime) {
		t.Errorf("HTML must not contain timestamp %q when NoTimestamp is set", formattedTime)
	}
	if strings.Contains(string(html), `class="generated"`) {
		t.Error("HTML must not contain the generated-at div when NoTimestamp is set")
	}
}

// TestNoTooltipsAndNoTimestamp verifies both flags together suppress their
// respective information independently.
func TestNoTooltipsAndNoTimestamp(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata")
	cfg, err := config.Load(filepath.Join(testdataDir, "basic.yaml"))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	result, err := runner.Run(context.Background(), cfg, runner.Options{Logger: io.Discard})
	if err != nil {
		t.Fatalf("running checks: %v", err)
	}

	html, err := renderer.Render(result, testdataDir, "test", renderer.Options{
		GeneratedAt: fixedTime,
		NoTooltips:  true,
		NoTimestamp: true,
	})
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	output := string(html)

	if strings.Contains(output, `data-tooltip="`) {
		t.Error("HTML must not contain data-tooltip attributes")
	}
	if strings.Contains(output, fixedTime.Format("2006-01-02 15:04:05")) {
		t.Error("HTML must not contain the timestamp")
	}
	// Dashboard structure must still be intact.
	if !strings.Contains(output, cfg.Title) {
		t.Errorf("HTML must still contain title %q", cfg.Title)
	}
}
