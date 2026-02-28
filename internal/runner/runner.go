// Package runner orchestrates check execution across all tiles and slots.
package runner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/halfdane/ilias/internal/checker"
	"github.com/halfdane/ilias/internal/config"
	"github.com/halfdane/ilias/internal/evaluator"
)

// SlotResult holds the evaluated status for a single slot.
type SlotResult struct {
	Name   string
	Status config.Status
	Output string // raw check output, for display on hover
}

// TileResult holds all the evaluated results for a single tile.
type TileResult struct {
	Name    string
	Display string
	Link    string
	Slots   []SlotResult
}

// GroupResult holds all tile results for a group.
type GroupResult struct {
	Name  string
	Tiles []TileResult
}

// DashboardResult holds the full evaluated dashboard state.
type DashboardResult struct {
	Title          string
	Theme          string
	RefreshSeconds int // 0 means no auto-refresh
	Groups         []GroupResult
}

// Options configures the runner behavior.
type Options struct {
	Concurrency int
	Verbose     bool
	Logger      io.Writer // for verbose output, defaults to io.Discard
}

// Run executes all checks for the given config and returns the dashboard result.
func Run(ctx context.Context, cfg *config.Config, opts Options) (*DashboardResult, error) {
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
		if concurrency > 16 {
			concurrency = 16
		}
		if concurrency < 1 {
			concurrency = 1
		}
	}

	logger := opts.Logger
	if logger == nil {
		logger = io.Discard
	}

	sem := make(chan struct{}, concurrency)

	result := &DashboardResult{
		Title:          cfg.Title,
		Theme:          cfg.Theme,
		RefreshSeconds: int(cfg.Refresh.Seconds()),
		Groups:         make([]GroupResult, len(cfg.Groups)),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for gi, group := range cfg.Groups {
		result.Groups[gi] = GroupResult{
			Name:  group.Name,
			Tiles: make([]TileResult, len(group.Tiles)),
		}

		for ti, tile := range group.Tiles {
			result.Groups[gi].Tiles[ti] = TileResult{
				Name:    tile.Name,
				Display: tile.Display,
				Link:    tile.Link,
				Slots:   make([]SlotResult, len(tile.Slots)),
			}

			// Run generate command if specified
			if tile.Generate != nil {
				wg.Add(1)
				gi, ti := gi, ti
				gen := tile.Generate
				tileName := tile.Name
				go func() {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					if err := runGenerate(ctx, gen, logger, tileName); err != nil {
						mu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						mu.Unlock()
						fmt.Fprintf(logger, "  [warn] generate for %q failed: %v\n", tileName, err)
					}

					_ = gi
					_ = ti
				}()
			}

			for si, slot := range tile.Slots {
				wg.Add(1)
				gi, ti, si := gi, ti, si
				slot := slot
				tileName := tile.Name
				go func() {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					sr := runSlot(ctx, slot, logger, tileName)

					mu.Lock()
					result.Groups[gi].Tiles[ti].Slots[si] = sr
					mu.Unlock()
				}()
			}
		}
	}

	wg.Wait()

	// Generate failures are warnings, not errors — the dashboard still renders
	return result, nil
}

func runGenerate(ctx context.Context, gen *config.Generate, logger io.Writer, tileName string) error {
	timeout := gen.Timeout.Duration
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fmt.Fprintf(logger, "  [generate] %s: %s\n", tileName, gen.Command)

	cmd := exec.CommandContext(ctx, "sh", "-c", gen.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %q: %w (output: %s)", gen.Command, err, string(output))
	}
	return nil
}

func runSlot(ctx context.Context, slot config.Slot, logger io.Writer, tileName string) SlotResult {
	fmt.Fprintf(logger, "  [check] %s/%s: %s %s\n", tileName, slot.Name, slot.Check.Type, slot.Check.Target)

	chk, err := checker.NewChecker(slot.Check.Type, slot.Check.Target, slot.Check.Timeout.Duration)
	if err != nil {
		fmt.Fprintf(logger, "  [error] %s/%s: %v\n", tileName, slot.Name, err)
		status := evaluator.BuiltinErrorStatus
		if slot.DefaultStatus != nil {
			status = *slot.DefaultStatus
		}
		return SlotResult{Name: slot.Name, Status: status}
	}

	result := chk.Check(ctx)
	if result.Err != nil {
		fmt.Fprintf(logger, "  [warn] %s/%s: check error: %v\n", tileName, slot.Name, result.Err)
	}

	status := evaluator.Evaluate(result, slot.Rules, slot.DefaultStatus)
	fmt.Fprintf(logger, "  [result] %s/%s: %s %s\n", tileName, slot.Name, status.ID, status.Label)

	return SlotResult{Name: slot.Name, Status: status, Output: result.Output}
}
