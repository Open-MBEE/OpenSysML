// Package diag holds the diagnostic type every layer reports findings in, the
// quick-fix edits a diagnostic carries, and the conformance mode that decides
// how strictly findings are judged.
package diag

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Severity classifies the impact of a Diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
	SeverityHint
)

var severityNames = map[Severity]string{
	SeverityError:   "error",
	SeverityWarning: "warning",
	SeverityInfo:    "info",
	SeverityHint:    "hint",
}

// String returns the lowercase name of the severity, or "unknown".
func (s Severity) String() string {
	if name, ok := severityNames[s]; ok {
		return name
	}
	return "unknown"
}

// Diagnostic is a single validation finding produced by a pass.
type Diagnostic struct {
	Severity Severity
	Span     source.Span
	Message  string
	Code     string // stable identifier for filtering / quick-fix selection
	Source   string // the pass ID that produced this diagnostic
	// Fixes are the unambiguous edits resolving the diagnostic, offered by an
	// editor as quick fixes.
	Fixes []Fix
	// Notation marks a finding about how the model is written rather than about
	// what it means: the document still reads, so it does not gate higher tiers.
	Notation bool
}

// Blocking reports whether the finding stops what depends on the model being
// readable — a higher validation tier, a summary of what a submission declared.
// A notation error is about the writing, not the meaning, so it stops neither.
func (d Diagnostic) Blocking() bool {
	return d.Severity == SeverityError && !d.Notation
}
