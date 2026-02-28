package evaluator

import (
	"errors"
	"regexp"
	"testing"

	"github.com/halfdane/ilias/internal/checker"
	"github.com/halfdane/ilias/internal/config"
)

func intPtr(i int) *int { return &i }

func TestEvaluate_ExactCodeMatch(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
		{
			Match:  config.Match{},
			Status: config.Status{ID: "unknown", Label: "❓"},
		},
	}

	result := checker.Result{Code: 200, Output: "hello"}
	status := Evaluate(result, rules, nil)
	if status.ID != "ok" {
		t.Errorf("status = %q, want %q", status.ID, "ok")
	}
}

func TestEvaluate_RegexCodeMatch(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Regex: regexp.MustCompile(`5\d\d`)}},
			Status: config.Status{ID: "error", Label: "❌"},
		},
	}

	result := checker.Result{Code: 503, Output: ""}
	status := Evaluate(result, rules, nil)
	if status.ID != "error" {
		t.Errorf("status = %q, want %q", status.ID, "error")
	}
}

func TestEvaluate_OutputRegexMatch(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Output: "maintenance.*true"},
			Status: config.Status{ID: "warn", Label: "⚠️"},
		},
	}

	result := checker.Result{Code: 200, Output: `{"maintenance": true}`}
	status := Evaluate(result, rules, nil)
	if status.ID != "warn" {
		t.Errorf("status = %q, want %q", status.ID, "warn")
	}
}

func TestEvaluate_CombinedMatch(t *testing.T) {
	rules := []config.Rule{
		{
			Match: config.Match{
				Code:   &config.MatchValue{Exact: intPtr(0)},
				Output: "update available",
			},
			Status: config.Status{ID: "update", Label: "🔄"},
		},
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(0)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
	}

	// Matches both code and output → first rule wins
	result := checker.Result{Code: 0, Output: "update available"}
	status := Evaluate(result, rules, nil)
	if status.ID != "update" {
		t.Errorf("status = %q, want %q", status.ID, "update")
	}

	// Matches code but not output → falls through to second rule
	result2 := checker.Result{Code: 0, Output: "all good"}
	status2 := Evaluate(result2, rules, nil)
	if status2.ID != "ok" {
		t.Errorf("status = %q, want %q", status2.ID, "ok")
	}
}

func TestEvaluate_FirstMatchWins(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "first", Label: "1"},
		},
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "second", Label: "2"},
		},
	}

	result := checker.Result{Code: 200}
	status := Evaluate(result, rules, nil)
	if status.ID != "first" {
		t.Errorf("status = %q, want %q", status.ID, "first")
	}
}

func TestEvaluate_CatchAll(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
		{
			Match:  config.Match{}, // catch-all
			Status: config.Status{ID: "fallback", Label: "❓"},
		},
	}

	result := checker.Result{Code: 404}
	status := Evaluate(result, rules, nil)
	if status.ID != "fallback" {
		t.Errorf("status = %q, want %q", status.ID, "fallback")
	}
}

func TestEvaluate_NoMatchUsesDefault(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
	}

	defaultStatus := &config.Status{ID: "custom-error", Label: "💥"}
	result := checker.Result{Code: 404}
	status := Evaluate(result, rules, defaultStatus)
	if status.ID != "custom-error" {
		t.Errorf("status = %q, want %q", status.ID, "custom-error")
	}
}

func TestEvaluate_NoMatchNoDefaultUsesBuiltin(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
	}

	result := checker.Result{Code: 404}
	status := Evaluate(result, rules, nil)
	if status.ID != BuiltinErrorStatus.ID {
		t.Errorf("status = %q, want %q", status.ID, BuiltinErrorStatus.ID)
	}
}

func TestEvaluate_CheckError_CatchAllMatches(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
		{
			Match:  config.Match{}, // catch-all
			Status: config.Status{ID: "down", Label: "🔴"},
		},
	}

	result := checker.Result{Err: errors.New("connection refused")}
	status := Evaluate(result, rules, nil)
	if status.ID != "down" {
		t.Errorf("status = %q, want %q (catch-all should match errors)", status.ID, "down")
	}
}

func TestEvaluate_CheckError_NoMatch(t *testing.T) {
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(200)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
	}

	result := checker.Result{Err: errors.New("timeout")}
	status := Evaluate(result, rules, nil)
	if status.ID != BuiltinErrorStatus.ID {
		t.Errorf("status = %q, want %q", status.ID, BuiltinErrorStatus.ID)
	}
}

func TestEvaluate_CheckError_NonCatchAllDoesNotMatch(t *testing.T) {
	// When check has error, only catch-all rules should match.
	// Specific code/output rules should NOT match.
	rules := []config.Rule{
		{
			Match:  config.Match{Code: &config.MatchValue{Exact: intPtr(0)}},
			Status: config.Status{ID: "ok", Label: "✅"},
		},
	}

	result := checker.Result{Code: 0, Err: errors.New("some error")}
	status := Evaluate(result, rules, nil)
	// Should NOT match the code=0 rule because there's an error
	if status.ID != BuiltinErrorStatus.ID {
		t.Errorf("status = %q, want %q (non-catch-all should not match on error)", status.ID, BuiltinErrorStatus.ID)
	}
}
