package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/halfdane/ilias/internal/config"
	"github.com/halfdane/ilias/internal/renderer"
	"github.com/halfdane/ilias/internal/runner"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

const usage = `ilias - static dashboard homepage generator

Usage:
  ilias <command> [flags]

Commands:
  generate    Run checks and generate the static HTML dashboard
  validate    Parse and validate the configuration file
  version     Print the version and exit

Flags (for generate):
  -c, --config        Path to config file (default: ./ilias.yaml)
  -o, --output        Output HTML file path (default: ./index.html)
  --dry-run           Show what would be checked without executing
  --concurrency       Max parallel checks (default: auto)
  -v, --verbose       Verbose logging to stderr
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "validate":
		if err := runValidate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-version":
		fmt.Printf("ilias %s\n", version)
		os.Exit(0)
	case "-h", "--help", "help":
		fmt.Fprint(os.Stderr, usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}

// GenerateOptions holds the parsed flags for the generate command.
type GenerateOptions struct {
	ConfigPath  string
	OutputPath  string
	DryRun      bool
	Concurrency int
	Verbose     bool
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)

	opts := GenerateOptions{}
	fs.StringVar(&opts.ConfigPath, "c", "ilias.yaml", "Path to config file")
	fs.StringVar(&opts.ConfigPath, "config", "ilias.yaml", "Path to config file")
	fs.StringVar(&opts.OutputPath, "o", "index.html", "Output HTML file path")
	fs.StringVar(&opts.OutputPath, "output", "index.html", "Output HTML file path")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "Show what would be checked without executing")
	fs.IntVar(&opts.Concurrency, "concurrency", 0, "Max parallel checks (0 = auto)")
	fs.BoolVar(&opts.Verbose, "v", false, "Verbose logging to stderr")
	fs.BoolVar(&opts.Verbose, "verbose", false, "Verbose logging to stderr")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return err
	}

	var logger io.Writer = io.Discard
	if opts.Verbose {
		logger = os.Stderr
		fmt.Fprintf(logger, "loaded config: %s (%d groups, %s theme)\n",
			opts.ConfigPath, len(cfg.Groups), cfg.Theme)
	}

	// Dry-run mode: print what would be done and exit
	if opts.DryRun {
		return printDryRun(cfg)
	}

	// Run all checks
	result, err := runner.Run(context.Background(), cfg, runner.Options{
		Concurrency: opts.Concurrency,
		Verbose:     opts.Verbose,
		Logger:      logger,
	})
	if err != nil {
		return fmt.Errorf("running checks: %w", err)
	}

	// Render HTML
	configDir := filepath.Dir(opts.ConfigPath)
	html, err := renderer.Render(result, configDir, version)
	if err != nil {
		return fmt.Errorf("rendering: %w", err)
	}

	// Write output
	if err := os.WriteFile(opts.OutputPath, html, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintf(logger, "wrote %d bytes to %s\n", len(html), opts.OutputPath)
	}

	return nil
}

func printDryRun(cfg *config.Config) error {
	fmt.Fprintf(os.Stderr, "Dashboard: %s (theme: %s)\n\n", cfg.Title, cfg.Theme)

	for _, g := range cfg.Groups {
		fmt.Fprintf(os.Stderr, "Group: %s\n", g.Name)
		for _, t := range g.Tiles {
			fmt.Fprintf(os.Stderr, "  Tile: %s\n", t.Name)
			fmt.Fprintf(os.Stderr, "    Display: %s\n", t.Display)
			if t.Link != "" {
				fmt.Fprintf(os.Stderr, "    Link: %s\n", t.Link)
			}
			if t.Generate != nil {
				fmt.Fprintf(os.Stderr, "    Generate: %s (timeout: %s)\n", t.Generate.Command, t.Generate.Timeout.Duration)
			}
			for _, s := range t.Slots {
				fmt.Fprintf(os.Stderr, "    Slot: %s\n", s.Name)
				fmt.Fprintf(os.Stderr, "      Check: %s %s", s.Check.Type, s.Check.Target)
				if s.Check.Timeout.Duration > 0 {
					fmt.Fprintf(os.Stderr, " (timeout: %s)", s.Check.Timeout.Duration)
				}
				fmt.Fprintln(os.Stderr)
				fmt.Fprintf(os.Stderr, "      Rules: %d\n", len(s.Rules))
				if s.DefaultStatus != nil {
					fmt.Fprintf(os.Stderr, "      Default: %s %s\n", s.DefaultStatus.ID, s.DefaultStatus.Label)
				}
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	return nil
}

// ValidateOptions holds the parsed flags for the validate command.
type ValidateOptions struct {
	ConfigPath string
	Verbose    bool
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)

	opts := ValidateOptions{}
	fs.StringVar(&opts.ConfigPath, "c", "ilias.yaml", "Path to config file")
	fs.StringVar(&opts.ConfigPath, "config", "ilias.yaml", "Path to config file")
	fs.BoolVar(&opts.Verbose, "v", false, "Verbose logging to stderr")
	fs.BoolVar(&opts.Verbose, "verbose", false, "Verbose logging to stderr")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return err
	}

	tileCount := 0
	for _, g := range cfg.Groups {
		tileCount += len(g.Tiles)
	}

	fmt.Fprintf(os.Stderr, "config OK: %d groups, %d tiles\n", len(cfg.Groups), tileCount)
	return nil
}
