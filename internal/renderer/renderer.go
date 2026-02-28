// Package renderer generates the static HTML dashboard from run results.
package renderer

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/halfdane/ilias/internal/runner"
)

//go:embed templates/dashboard.tmpl
var templateFS embed.FS

//go:embed templates/style.css
var embeddedCSS string

// templateData is the data structure passed to the HTML template.
type templateData struct {
	Title          string
	Theme          string
	CSS            template.CSS
	GeneratedAt    string
	RefreshSeconds int
	Version        string
	Groups         []groupData
}

type groupData struct {
	Name  string
	Tiles []tileData
}

type tileData struct {
	Name      string
	Link      string
	HasIcon   bool         // true when icon field was specified in config
	IconData  template.URL // data URI or empty
	BannerURI template.URL // data URI for full-width banner, or empty
	Slots     []slotData
}

type slotData struct {
	Name   string
	Label  string
	Output string // raw check output, shown as tooltip
}

// Render generates the HTML dashboard from the dashboard result.
// The configDir is used to resolve relative file paths in tile icon/banner fields.
// The version string is embedded in the page footer.
func Render(result *runner.DashboardResult, configDir, version string) ([]byte, error) {
	css, err := loadCSS()
	if err != nil {
		return nil, fmt.Errorf("loading CSS: %w", err)
	}

	funcMap := template.FuncMap{
		"firstChar": func(s string) string {
			for _, r := range s {
				return string(r)
			}
			return "?"
		},
	}

	tmplData, err := templateFS.ReadFile("templates/dashboard.tmpl")
	if err != nil {
		return nil, fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("dashboard").Funcs(funcMap).Parse(string(tmplData))
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	data := templateData{
		Title:          result.Title,
		Theme:          result.Theme,
		CSS:            template.CSS(css),
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
		RefreshSeconds: result.RefreshSeconds,
		Version:        version,
		Groups:         make([]groupData, len(result.Groups)),
	}

	for gi, g := range result.Groups {
		gd := groupData{
			Name:  g.Name,
			Tiles: make([]tileData, len(g.Tiles)),
		}
		for ti, t := range g.Tiles {
			td := tileData{
				Name: t.Name,
				Link: t.Link,
			}

			// Resolve icon
			if t.Icon != "" {
				td.HasIcon = true
				iconURI, err := resolveIcon(t.Icon, configDir)
				if err != nil {
					// Not fatal; just skip the icon
					fmt.Fprintf(os.Stderr, "[warn] resolving icon for %q: %v\n", t.Name, err)
					td.IconData = ""
				} else {
					td.IconData = template.URL(iconURI)
				}
			}

			// Resolve banner
			if t.Banner != nil {
				bannerURI, err := resolveIcon(t.Banner.Src, configDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[warn] resolving banner for %q: %v\n", t.Name, err)
				} else {
					td.BannerURI = template.URL(bannerURI)
				}
			}

			td.Slots = make([]slotData, len(t.Slots))
			for si, s := range t.Slots {
				td.Slots[si] = slotData{
					Name:   s.Name,
					Label:  s.Status.Label,
					Output: strings.TrimSpace(s.Output),
				}
			}
			gd.Tiles[ti] = td
		}
		data.Groups[gi] = gd
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	return buf.Bytes(), nil
}

func loadCSS() (string, error) {
	return embeddedCSS, nil
}

// resolveIcon converts a display value to a data URI.
// If it's a URL (http/https), it's fetched and embedded.
// If it's a file path, it's read and embedded.
// Returns empty string if the display doesn't resolve to an image.
func resolveIcon(display, configDir string) (string, error) {
	if display == "" {
		return "", nil
	}

	// URL: fetch and embed
	if strings.HasPrefix(display, "http://") || strings.HasPrefix(display, "https://") {
		return fetchAndEmbed(display)
	}

	// File path: resolve relative to config directory
	path := display
	if !filepath.IsAbs(path) {
		path = filepath.Join(configDir, path)
	}

	return fileToDataURI(path)
}

func fetchAndEmbed(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MiB limit
	if err != nil {
		return "", fmt.Errorf("reading response from %s: %w", url, err)
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = http.DetectContentType(data)
	}

	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}

func fileToDataURI(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file %s: %w", path, err)
	}

	mime := http.DetectContentType(data)

	// Refine MIME type based on extension for common image formats
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".svg":
		mime = "image/svg+xml"
	case ".webp":
		mime = "image/webp"
	case ".ico":
		mime = "image/x-icon"
	}

	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}
