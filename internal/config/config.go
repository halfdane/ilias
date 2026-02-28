// Package config handles parsing and validation of ilias configuration files.
package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for the dashboard.
type Config struct {
	Title  string  `yaml:"title"`
	Theme  string  `yaml:"theme"`
	Groups []Group `yaml:"groups"`
}

// Group is a named collection of tiles.
type Group struct {
	Name  string `yaml:"name"`
	Tiles []Tile `yaml:"tiles"`
}

// Tile represents a single dashboard tile.
type Tile struct {
	Name     string    `yaml:"name"`
	Display  string    `yaml:"display"`
	Link     string    `yaml:"link,omitempty"`
	Generate *Generate `yaml:"generate,omitempty"`
	Slots    []Slot    `yaml:"slots,omitempty"`
}

// Generate defines an optional command to run before rendering a tile.
type Generate struct {
	Command string   `yaml:"command"`
	Timeout Duration `yaml:"timeout,omitempty"`
}

// Slot is a named status indicator on a tile.
type Slot struct {
	Name          string  `yaml:"name"`
	Check         Check   `yaml:"check"`
	Rules         []Rule  `yaml:"rules"`
	DefaultStatus *Status `yaml:"default_status,omitempty"`
}

// Check defines how to obtain status information (HTTP request or CLI command).
type Check struct {
	Type    string   `yaml:"type"`   // "http" or "command"
	Target  string   `yaml:"target"` // URL or command string
	Timeout Duration `yaml:"timeout,omitempty"`
}

// Rule defines a condition and the resulting status if matched.
type Rule struct {
	Match  Match  `yaml:"match"`
	Status Status `yaml:"status"`
}

// Match defines the conditions for a rule. An empty match is a catch-all.
type Match struct {
	Code   *MatchValue `yaml:"code,omitempty"`
	Output string      `yaml:"output,omitempty"`
}

// MatchValue can be an integer (exact match) or a string (regex match).
// We use yaml.Node to handle both cases during unmarshalling.
type MatchValue struct {
	Exact *int
	Regex *regexp.Regexp
}

// UnmarshalYAML implements custom unmarshalling for MatchValue.
func (m *MatchValue) UnmarshalYAML(value *yaml.Node) error {
	// Try integer first
	var intVal int
	if err := value.Decode(&intVal); err == nil {
		m.Exact = &intVal
		return nil
	}

	// Try string (regex)
	var strVal string
	if err := value.Decode(&strVal); err == nil {
		re, err := regexp.Compile(strVal)
		if err != nil {
			return fmt.Errorf("invalid regex in code match %q: %w", strVal, err)
		}
		m.Regex = re
		return nil
	}

	return fmt.Errorf("code match must be an integer or a regex string, got %v", value.Tag)
}

// Status defines a status identifier and its display label.
type Status struct {
	ID    string `yaml:"id"`
	Label string `yaml:"label"`
}

// Duration wraps time.Duration for YAML string parsing (e.g., "10s", "5m").
type Duration struct {
	time.Duration
}

// UnmarshalYAML parses a duration string like "10s" or "5m".
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

// Load reads and parses a config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	return Parse(data)
}

// Parse parses YAML data into a Config.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate checks the config for required fields and consistency.
func (c *Config) validate() error {
	if c.Title == "" {
		return fmt.Errorf("config: title is required")
	}

	if c.Theme == "" {
		c.Theme = "dark"
	}
	if c.Theme != "dark" && c.Theme != "light" {
		return fmt.Errorf("config: theme must be \"dark\" or \"light\", got %q", c.Theme)
	}

	if len(c.Groups) == 0 {
		return fmt.Errorf("config: at least one group is required")
	}

	for gi, g := range c.Groups {
		if g.Name == "" {
			return fmt.Errorf("config: group[%d]: name is required", gi)
		}
		if len(g.Tiles) == 0 {
			return fmt.Errorf("config: group[%d] %q: at least one tile is required", gi, g.Name)
		}
		for ti, t := range g.Tiles {
			if err := validateTile(gi, g.Name, ti, t); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTile(gi int, gname string, ti int, t Tile) error {
	prefix := fmt.Sprintf("config: group[%d] %q, tile[%d]", gi, gname, ti)

	if t.Name == "" {
		return fmt.Errorf("%s: name is required", prefix)
	}
	prefix = fmt.Sprintf("config: group[%d] %q, tile[%d] %q", gi, gname, ti, t.Name)

	if t.Display == "" {
		return fmt.Errorf("%s: display is required", prefix)
	}

	if t.Generate != nil && t.Generate.Command == "" {
		return fmt.Errorf("%s: generate.command is required when generate is specified", prefix)
	}

	for si, s := range t.Slots {
		if err := validateSlot(prefix, si, s); err != nil {
			return err
		}
	}
	return nil
}

func validateSlot(prefix string, si int, s Slot) error {
	slotPrefix := fmt.Sprintf("%s, slot[%d]", prefix, si)

	if s.Name == "" {
		return fmt.Errorf("%s: name is required", slotPrefix)
	}
	slotPrefix = fmt.Sprintf("%s, slot[%d] %q", prefix, si, s.Name)

	if s.Check.Type == "" {
		return fmt.Errorf("%s: check.type is required", slotPrefix)
	}
	if s.Check.Type != "http" && s.Check.Type != "command" {
		return fmt.Errorf("%s: check.type must be \"http\" or \"command\", got %q", slotPrefix, s.Check.Type)
	}
	if s.Check.Target == "" {
		return fmt.Errorf("%s: check.target is required", slotPrefix)
	}

	if len(s.Rules) == 0 {
		return fmt.Errorf("%s: at least one rule is required", slotPrefix)
	}

	for ri, r := range s.Rules {
		if r.Status.ID == "" {
			return fmt.Errorf("%s, rule[%d]: status.id is required", slotPrefix, ri)
		}
		if r.Status.Label == "" {
			return fmt.Errorf("%s, rule[%d]: status.label is required", slotPrefix, ri)
		}
		// Validate output regex if present
		if r.Match.Output != "" {
			if _, err := regexp.Compile(r.Match.Output); err != nil {
				return fmt.Errorf("%s, rule[%d]: invalid output regex %q: %w", slotPrefix, ri, r.Match.Output, err)
			}
		}
	}

	if s.DefaultStatus != nil {
		if s.DefaultStatus.ID == "" {
			return fmt.Errorf("%s: default_status.id is required", slotPrefix)
		}
		if s.DefaultStatus.Label == "" {
			return fmt.Errorf("%s: default_status.label is required", slotPrefix)
		}
	}

	return nil
}
