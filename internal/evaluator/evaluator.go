// Package evaluator matches check results against rules to determine status.
package evaluator

import (
	"strconv"

	"github.com/halfdane/ilias/internal/checker"
	"github.com/halfdane/ilias/internal/config"
)

// BuiltinErrorStatus is the default status when a check fails and no rules match.
var BuiltinErrorStatus = config.Status{
	ID:    "error",
	Label: "⚡",
}

// Evaluate matches a check result against a list of rules (first match wins).
// Returns BuiltinErrorStatus when no rule matches.
func Evaluate(result checker.Result, rules []config.Rule) config.Status {
	for _, rule := range rules {
		if matchesRule(result, rule.Match) {
			return rule.Status
		}
	}
	return BuiltinErrorStatus
}

// matchesRule checks whether a result satisfies a match condition.
// An empty match (no code, no output) is a catch-all that always matches.
func matchesRule(result checker.Result, match config.Match) bool {
	// If the check errored, code matching is meaningless (no status code).
	// Output matching is still allowed so rules can match on the error message.
	if result.Err != nil {
		if match.Code != nil {
			return false
		}
		if match.Output != nil {
			return match.Output.MatchString(result.Output)
		}
		// match: {} — catch-all
		return true
	}

	hasCondition := false

	// Check code match
	if match.Code != nil {
		hasCondition = true
		if !matchCode(result.Code, match.Code) {
			return false
		}
	}

	// Check output match
	if match.Output != nil {
		hasCondition = true
		if !match.Output.MatchString(result.Output) {
			return false
		}
	}

	// If no conditions were specified, this is a catch-all → always matches
	if !hasCondition {
		return true
	}

	// All specified conditions matched
	return true
}

// matchCode checks if the result code matches the MatchValue (exact int or regex).
func matchCode(code int, mv *config.MatchValue) bool {
	if mv.Exact != nil {
		return code == *mv.Exact
	}
	if mv.Regex != nil {
		return mv.Regex.MatchString(strconv.Itoa(code))
	}
	return false
}
