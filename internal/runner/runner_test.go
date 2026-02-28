package runner

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/halfdane/ilias/internal/config"
)

func TestRun_BasicConfig(t *testing.T) {
	cfg := &config.Config{
		Title: "Test Dashboard",
		Theme: "dark",
		Groups: []config.Group{
			{
				Name: "Test",
				Tiles: []config.Tile{
					{
						Name:    "Echo",
						Icon: "icon.png",
						Slots: []config.Slot{
							{
								Name: "status",
								Check: config.Check{
									Type:   "command",
									Target: "echo hello",
								},
								Rules: []config.Rule{
									{
										Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(0)}},
										Status: config.Status{ID: "ok", Label: "✅"},
									},
									{
										Match:  config.Match{},
										Status: config.Status{ID: "fail", Label: "❌"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	result, err := Run(context.Background(), cfg, Options{
		Concurrency: 2,
		Verbose:     true,
		Logger:      &buf,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Title != "Test Dashboard" {
		t.Errorf("title = %q, want %q", result.Title, "Test Dashboard")
	}
	if len(result.Groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(result.Groups))
	}
	if len(result.Groups[0].Tiles) != 1 {
		t.Fatalf("len(tiles) = %d, want 1", len(result.Groups[0].Tiles))
	}

	tile := result.Groups[0].Tiles[0]
	if tile.Name != "Echo" {
		t.Errorf("tile name = %q, want %q", tile.Name, "Echo")
	}
	if len(tile.Slots) != 1 {
		t.Fatalf("len(slots) = %d, want 1", len(tile.Slots))
	}
	if tile.Slots[0].Status.ID != "ok" {
		t.Errorf("slot status = %q, want %q", tile.Slots[0].Status.ID, "ok")
	}

	// Verify verbose output
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("[check]")) {
		t.Errorf("verbose output missing [check]: %s", output)
	}
}

func TestRun_FailingCommand(t *testing.T) {
	cfg := &config.Config{
		Title: "Test",
		Theme: "dark",
		Groups: []config.Group{
			{
				Name: "Test",
				Tiles: []config.Tile{
					{
						Name:    "Fail",
						Icon: "icon.png",
						Slots: []config.Slot{
							{
								Name: "status",
								Check: config.Check{
									Type:   "command",
									Target: "exit 1",
								},
								Rules: []config.Rule{
									{
										Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(0)}},
										Status: config.Status{ID: "ok", Label: "✅"},
									},
									{
										Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(1)}},
										Status: config.Status{ID: "down", Label: "🔴"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := Run(context.Background(), cfg, Options{Concurrency: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	slot := result.Groups[0].Tiles[0].Slots[0]
	if slot.Status.ID != "down" {
		t.Errorf("slot status = %q, want %q", slot.Status.ID, "down")
	}
}

func TestRun_ConcurrentExecution(t *testing.T) {
	// Two tiles with commands that each take 200ms.
	// With concurrency=2, they should finish in ~200ms, not ~400ms.
	cfg := &config.Config{
		Title: "Test",
		Theme: "dark",
		Groups: []config.Group{
			{
				Name: "G",
				Tiles: []config.Tile{
					{
						Name:    "A",
						Icon: "a.png",
						Slots: []config.Slot{
							{
								Name:  "s",
								Check: config.Check{Type: "command", Target: "sleep 0.2 && echo done"},
								Rules: []config.Rule{{Match: config.Match{}, Status: config.Status{ID: "ok", Label: "✅"}}},
							},
						},
					},
					{
						Name:    "B",
						Icon: "b.png",
						Slots: []config.Slot{
							{
								Name:  "s",
								Check: config.Check{Type: "command", Target: "sleep 0.2 && echo done"},
								Rules: []config.Rule{{Match: config.Match{}, Status: config.Status{ID: "ok", Label: "✅"}}},
							},
						},
					},
				},
			},
		},
	}

	start := time.Now()
	_, err := Run(context.Background(), cfg, Options{Concurrency: 2})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With parallel execution, should be well under 400ms
	if elapsed > 350*time.Millisecond {
		t.Errorf("elapsed = %v, expected < 350ms (checks should run in parallel)", elapsed)
	}
}

func TestRun_WithGenerate(t *testing.T) {
	cfg := &config.Config{
		Title: "Test",
		Theme: "dark",
		Groups: []config.Group{
			{
				Name: "G",
				Tiles: []config.Tile{
					{
						Name:    "Gen",
						Icon: "icon.png",
						Generate: &config.Generate{
							Command: "echo generated",
						},
						Slots: []config.Slot{
							{
								Name:  "s",
								Check: config.Check{Type: "command", Target: "echo ok"},
								Rules: []config.Rule{{Match: config.Match{}, Status: config.Status{ID: "ok", Label: "✅"}}},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	_, err := Run(context.Background(), cfg, Options{
		Concurrency: 1,
		Logger:      &buf,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("[generate]")) {
		t.Errorf("verbose output missing [generate]: %s", buf.String())
	}
}

func intPtr(i int) *int { return &i }
