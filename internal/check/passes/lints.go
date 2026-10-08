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
var lintCodes = []string{CodeUndeclaredSignal, CodePortTypeMismatch, CodeDeferredKeeperUnmarked, CodeRoundedRealLiteral}

// optInLints are the lints left out of the diagnostics until a surface enables
// them: their findings hold of ordinary models more often than they mark a slip.
var optInLints = []string{CodeRoundedRealLiteral}

// LintCodes returns the codes of the diagnostics a surface may disable.
func LintCodes() []string { return slices.Clone(lintCodes) }

// IsLintCode reports whether code is the code of a lint.
func IsLintCode(code string) bool { return slices.Contains(lintCodes, code) }

// IsOptInLint reports whether code is a lint reported only once enabled.
func IsOptInLint(code string) bool { return slices.Contains(optInLints, code) }

// WithoutLints returns diags without the lint findings a surface leaves out:
// those whose codes disabled holds, and those of the opt-in lints enabled does
// not hold; diags itself is returned when it has none to drop.
func WithoutLints(diags []diag.Diagnostic, disabled, enabled map[string]bool) []diag.Diagnostic {
	drop := func(d diag.Diagnostic) bool {
		return d.Source == lintSource && (disabled[d.Code] || IsOptInLint(d.Code) && !enabled[d.Code])
	}
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
