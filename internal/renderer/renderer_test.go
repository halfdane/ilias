package renderer

import (
	"strings"
	"testing"

	"github.com/halfdane/ilias/internal/config"
	"github.com/halfdane/ilias/internal/runner"
)

func TestRender_BasicOutput(t *testing.T) {
	result := &runner.DashboardResult{
		Title: "Test Dashboard",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "Services",
				Tiles: []runner.TileResult{
					{
						Name:    "Example",
						Icon: "",
						Link:    "https://example.com",
						Slots: []runner.SlotResult{
							{Name: "status", Status: config.Status{ID: "ok", Label: "✅"}},
						},
					},
				},
			},
		},
	}

	html, err := Render(result, "/tmp", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(html)

	// Check essential parts
	checks := []string{
		"<!DOCTYPE html>",
		"Test Dashboard",
		`data-theme="dark"`,
		"Services",
		"Example",
		"https://example.com",
		"✅",
		"status",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q", check)
		}
	}
}

func TestRender_LightTheme(t *testing.T) {
	result := &runner.DashboardResult{
		Title:  "Test",
		Theme:  "light",
		Groups: []runner.GroupResult{{Name: "G", Tiles: []runner.TileResult{{Name: "T", Icon: ""}}}},
	}

	html, err := Render(result, "/tmp", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(html), `data-theme="light"`) {
		t.Error("output missing light theme")
	}
}

func TestRender_TileWithoutLink(t *testing.T) {
	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{Name: "NoLink", Icon: ""},
				},
			},
		},
	}

	html, err := Render(result, "/tmp", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(html)
	// Should use div, not anchor
	if strings.Contains(output, `<a class="tile"`) {
		t.Error("tile without link should not be an anchor")
	}
}

func TestRender_TileWithLink(t *testing.T) {
	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{Name: "Linked", Icon: "", Link: "https://example.com"},
				},
			},
		},
	}

	html, err := Render(result, "/tmp", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(html)
	if !strings.Contains(output, `<a class="tile"`) {
		t.Error("tile with link should be an anchor")
	}
	if !strings.Contains(output, `href="https://example.com"`) {
		t.Error("output missing href")
	}
}

func TestRender_MultipleSlots(t *testing.T) {
	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{
						Name:    "Multi",
						Icon: "",
						Slots: []runner.SlotResult{
							{Name: "avail", Status: config.Status{ID: "ok", Label: "✅"}},
							{Name: "updates", Status: config.Status{ID: "update", Label: "🔄"}},
						},
					},
				},
			},
		},
	}

	html, err := Render(result, "/tmp", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(html)
	if !strings.Contains(output, "✅") || !strings.Contains(output, "🔄") {
		t.Error("output missing slot labels")
	}
	if !strings.Contains(output, "avail") || !strings.Contains(output, "updates") {
		t.Error("output missing slot names")
	}
}

func TestRender_CSSEmbedded(t *testing.T) {
	result := &runner.DashboardResult{
		Title:  "Test",
		Theme:  "dark",
		Groups: []runner.GroupResult{{Name: "G", Tiles: []runner.TileResult{{Name: "T", Icon: ""}}}},
	}

	html, err := Render(result, "/tmp", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(html)
	if !strings.Contains(output, "<style>") {
		t.Error("output missing <style> tag")
	}
	if !strings.Contains(output, "--bg:") {
		t.Error("output missing CSS variables")
	}
}
