package passes

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// lintSource is the source of the lints: warnings about a model the
// specification accepts, each of which a surface can switch off by its code.
const lintSource = "lint"

// CodeUndeclaredSignal marks a `when <name>` naming no declaration visible
// where it is written and no signal the model sends.
const CodeUndeclaredSignal = "undeclared-signal"

// CodePortTypeMismatch marks a connection, interface or flow between ports
// whose definitions are unrelated and whose directed features are not conjugate.
const CodePortTypeMismatch = "port-type-mismatch"

// CodeDeferredKeeperUnmarked marks an accept of a deferred signal at the root
// of a deferring state's do action that is not marked as the keeping accept.
const CodeDeferredKeeperUnmarked = "deferred-keeper-unmarked"

// lintCodes are the codes of the lints, in the order surfaces list them.
var lintCodes = []string{CodeUndeclaredSignal, CodePortTypeMismatch, CodeDeferredKeeperUnmarked}

// LintCodes returns the codes of the diagnostics a surface may disable.
func LintCodes() []string { return slices.Clone(lintCodes) }

// IsLintCode reports whether code is the code of a lint.
func IsLintCode(code string) bool { return slices.Contains(lintCodes, code) }

// WithoutLints returns diags without the lint findings whose codes disabled
// holds; diags itself is returned when it has none to drop.
func WithoutLints(diags []diag.Diagnostic, disabled map[string]bool) []diag.Diagnostic {
	if len(disabled) == 0 {
		return diags
	}
	drop := func(d diag.Diagnostic) bool { return d.Source == lintSource && disabled[d.Code] }
	if !slices.ContainsFunc(diags, drop) {
		return diags
	}
	out := make([]diag.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if !drop(d) {
			out = append(out, d)
		}
	}
	return out
}

// CheckLintCodes reports the first of codes that names no lint.
func CheckLintCodes(codes []string) error {
	for _, code := range codes {
		if !IsLintCode(code) {
			return fmt.Errorf("unknown lint %q (want one of %s)", code, strings.Join(lintCodes, ", "))
		}
	}
	return nil
}
