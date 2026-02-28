// Package evaluator matches check results against rules to determine status.
package evaluator

import (
	"fmt"
	"regexp"
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
// If the check itself errored and no rules match, returns the defaultStatus
// (or BuiltinErrorStatus if nil).
func Evaluate(result checker.Result, rules []config.Rule, defaultStatus *config.Status) config.Status {
	for _, rule := range rules {
		if matchesRule(result, rule.Match) {
			return rule.Status
		}
	}

	// No rule matched
	if defaultStatus != nil {
		return *defaultStatus
	}

	// If the check had an error, use error status
	if result.Err != nil {
		return BuiltinErrorStatus
	}

	// Nothing matched and no error — still return error status as fallback
	return BuiltinErrorStatus
}

// matchesRule checks whether a result satisfies a match condition.
// An empty match (no code, no output) is a catch-all that always matches.
func matchesRule(result checker.Result, match config.Match) bool {
	// If the check itself failed (Err != nil), only match catch-all rules
	if result.Err != nil {
		return match.Code == nil && match.Output == ""
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
	if match.Output != "" {
		hasCondition = true
		if !matchOutput(result.Output, match.Output) {
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

// matchOutput checks if the result output matches the regex pattern.
func matchOutput(output, pattern string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		// This should have been caught during config validation,
		// but be defensive.
		fmt.Printf("warning: invalid output regex %q: %v\n", pattern, err)
		return false
	}
	return re.MatchString(output)
}
