package renderer

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// ---------------------------------------------------------------------------
// resolveIcon tests
// ---------------------------------------------------------------------------

func TestResolveIcon_Empty(t *testing.T) {
	got, err := resolveIcon("", "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveIcon_URL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("fake-png"))
	}))
	defer srv.Close()

	got, err := resolveIcon(srv.URL+"/icon.png", "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Errorf("expected data URI with image/png, got %q", got)
	}
}

func TestResolveIcon_RelativeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), []byte("fake-png"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveIcon("logo.png", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "data:") {
		t.Errorf("expected data URI, got %q", got)
	}
}

func TestResolveIcon_AbsoluteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path, []byte("<svg></svg>"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveIcon(path, "/some/other/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "data:image/svg+xml;base64,") {
		t.Errorf("expected SVG data URI, got %q", got)
	}
}

func TestResolveIcon_MissingFile(t *testing.T) {
	_, err := resolveIcon("nonexistent.png", t.TempDir())
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ---------------------------------------------------------------------------
// fetchAndEmbed tests
// ---------------------------------------------------------------------------

func TestFetchAndEmbed_Success(t *testing.T) {
	body := []byte("fake-image-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(body)
	}))
	defer srv.Close()

	got, err := fetchAndEmbed(srv.URL + "/img.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(body))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFetchAndEmbed_NoContentType(t *testing.T) {
	// When server doesn't set Content-Type, DetectContentType is used.
	body := []byte("not-really-an-image")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Explicitly remove Content-Type.
		w.Header().Del("Content-Type")
		w.Write(body)
	}))
	defer srv.Close()

	got, err := fetchAndEmbed(srv.URL + "/unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "data:") {
		t.Errorf("expected data URI, got %q", got)
	}
	if !strings.Contains(got, ";base64,") {
		t.Errorf("expected base64 encoding in data URI")
	}
}

func TestFetchAndEmbed_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchAndEmbed(srv.URL + "/missing")
	if err == nil {
		t.Error("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// fileToDataURI tests
// ---------------------------------------------------------------------------

func TestFileToDataURI_PNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	content := []byte("fake-png-content")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := fileToDataURI(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(content))
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFileToDataURI_SVG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	content := []byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := fileToDataURI(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "data:image/svg+xml;base64,") {
		t.Errorf("expected SVG MIME type, got %q", got)
	}
}

func TestFileToDataURI_NotFound(t *testing.T) {
	_, err := fileToDataURI("/nonexistent/file.png")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ---------------------------------------------------------------------------
// Render with icon/banner integration tests
// ---------------------------------------------------------------------------

func TestRender_TileWithFileIcon(t *testing.T) {
	dir := t.TempDir()
	iconContent := []byte("fake-icon")
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), iconContent, 0644); err != nil {
		t.Fatal(err)
	}

	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{
						Name: "WithIcon",
						Icon: "logo.png",
					},
				},
			},
		},
	}

	html, err := Render(result, dir, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(html)
	encoded := base64.StdEncoding.EncodeToString(iconContent)
	if !strings.Contains(output, encoded) {
		t.Error("output should contain base64-encoded icon data")
	}
}

func TestRender_TileWithMissingIcon(t *testing.T) {
	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{
						Name: "NoIcon",
						Icon: "nonexistent.png",
					},
				},
			},
		},
	}

	// Should not return an error — missing icons are warnings, not fatal.
	html, err := Render(result, t.TempDir(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(html), "NoIcon") {
		t.Error("tile name should still appear in output")
	}
}

func TestRender_TileWithBanner(t *testing.T) {
	dir := t.TempDir()
	bannerContent := []byte("fake-banner-jpg")
	if err := os.WriteFile(filepath.Join(dir, "banner.jpg"), bannerContent, 0644); err != nil {
		t.Fatal(err)
	}

	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{
						Name:   "WithBanner",
						Banner: &config.Banner{Src: "banner.jpg"},
					},
				},
			},
		},
	}

	html, err := Render(result, dir, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := string(html)
	encoded := base64.StdEncoding.EncodeToString(bannerContent)
	if !strings.Contains(output, encoded) {
		t.Error("output should contain base64-encoded banner data")
	}
}

func TestRender_TileWithURLIcon(t *testing.T) {
	body := []byte("remote-icon-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(body)
	}))
	defer srv.Close()

	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{
						Name: "RemoteIcon",
						Icon: srv.URL + "/icon.png",
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
	encoded := base64.StdEncoding.EncodeToString(body)
	if !strings.Contains(output, encoded) {
		t.Error("output should contain base64-encoded remote icon data")
	}
}

func TestRender_NoTooltips(t *testing.T) {
	result := &runner.DashboardResult{
		Title: "Test",
		Theme: "dark",
		Groups: []runner.GroupResult{
			{
				Name: "G",
				Tiles: []runner.TileResult{
					{
						Name: "T",
						Slots: []runner.SlotResult{
							{
								Name:   "status",
								Status: config.Status{ID: "ok", Label: "✅"},
								Output: "HTTP 200 OK\n\ninternal host: db.corp:5432",
							},
						},
					},
				},
			},
		},
	}

	// Default: tooltips present
	html, err := Render(result, "/tmp", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(html), `data-tooltip="`) {
		t.Error("expected data-tooltip attribute with default options")
	}
	if !strings.Contains(string(html), "db.corp") {
		t.Error("expected tooltip content in default mode")
	}

	// NoTooltips: output stripped entirely
	html, err = Render(result, "/tmp", "test", Options{NoTooltips: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(html), `data-tooltip="`) {
		t.Error("expected no data-tooltip attribute with NoTooltips=true")
	}
	if strings.Contains(string(html), "db.corp") {
		t.Error("expected tooltip content removed with NoTooltips=true")
	}
}
