package main

import "strings"

// Category is the coarse classification the two implementations are compared on.
// Message wording never matches between them, so the comparison is made on this
// plus file, line and severity.
type Category string

const (
	CategoryUnresolved   Category = "unresolved-reference"
	CategoryKindMismatch Category = "kind-mismatch"
	CategoryMultiplicity Category = "multiplicity"
	CategoryUnits        Category = "units"
	CategorySyntax       Category = "syntax"
	// CategoryUnmapped is for a diagnostic no rule below claims. It is
	// deliberately never merged into another category: an unmapped diagnostic
	// must show up as a disagreement rather than as accidental agreement.
	// Specialization cycles stay here by adjudication: the pilot has no such
	// check, and no category above describes one (F4 in
	// docs/project/pilot-differential.md).
	CategoryUnmapped Category = "unmapped"
)

// categorizeOpenSysML maps one of our diagnostics to a category, preferring its
// stable code and pass ID over the message text.
func categorizeOpenSysML(code, pass, message string) Category {
	switch {
	case code == "unresolved", strings.HasPrefix(code, "unresolved."):
		return CategoryUnresolved
	case code == "syntax", strings.HasPrefix(code, "syntax."), pass == "syntax":
		return CategorySyntax
	case strings.Contains(code, "multiplicity"):
		return CategoryMultiplicity
	case strings.Contains(code, "unit"), strings.Contains(code, "quantity"):
		return CategoryUnits
	// Worded as the reference words it, whose copy `conforming` maps below.
	case code == "bound-feature-types":
		return CategoryKindMismatch
	}

	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "unresolved reference"),
		strings.Contains(lower, "undefined"),
		strings.Contains(lower, "cannot resolve"),
		strings.Contains(lower, "not found"),
		strings.Contains(lower, "no member named"):
		return CategoryUnresolved
	case strings.Contains(lower, "unit"), strings.Contains(lower, "commensurab"),
		strings.Contains(lower, "quantity"):
		return CategoryUnits
	case strings.Contains(lower, "multiplicity"), strings.Contains(lower, "bound"):
		return CategoryMultiplicity
	case strings.Contains(lower, "expected"), strings.Contains(lower, "unexpected"),
		strings.Contains(lower, "missing"):
		return CategorySyntax
	// `must have`/`must invoke` mirror the pilot side below: both implementations
	// word the metaclass, result-type and invocation constraints identically.
	case strings.Contains(lower, "must be"), strings.Contains(lower, "must have"),
		strings.Contains(lower, "must invoke"),
		strings.Contains(lower, "not a "),
		strings.Contains(lower, "type"), strings.Contains(lower, "kind"),
		strings.Contains(lower, "conform"):
		return CategoryKindMismatch
	}
	return CategoryUnmapped
}

// categorizePilot maps a pilot diagnostic to a category from its message: the
// wrapper reports Xtext issues, whose codes it does not print.
func categorizePilot(message string) Category {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "couldn't resolve reference"),
		strings.Contains(lower, "cannot be resolved"):
		return CategoryUnresolved
	// ANTLR parse errors, all reported by the Xtext syntax layer.
	case strings.Contains(lower, "mismatched input"),
		strings.Contains(lower, "no viable alternative"),
		strings.Contains(lower, "extraneous input"),
		strings.Contains(lower, "missing "),
		strings.Contains(lower, "required (...)"),
		strings.Contains(lower, "rule end"),
		strings.Contains(lower, "unexpected"):
		return CategorySyntax
	case strings.Contains(lower, "unit"), strings.Contains(lower, "quantity"),
		strings.Contains(lower, "measurement reference"):
		return CategoryUnits
	case strings.Contains(lower, "multiplicity"), strings.Contains(lower, "at most"),
		strings.Contains(lower, "at least"), strings.Contains(lower, "cardinality"):
		return CategoryMultiplicity
	// `must be` mirrors the clause categorizeOpenSysML applies to our own text:
	// a message both implementations word alike must not land in two categories.
	case strings.Contains(lower, "must be"),
		strings.Contains(lower, "must invoke"),
		strings.Contains(lower, "must reference"),
		strings.Contains(lower, "must subset"),
		strings.Contains(lower, "must specialize"),
		strings.Contains(lower, "must redefine"),
		strings.Contains(lower, "conforming"),
		strings.Contains(lower, "should be"),
		strings.Contains(lower, "must have"),
		strings.Contains(lower, "must not"):
		return CategoryKindMismatch
	}
	return CategoryUnmapped
}
