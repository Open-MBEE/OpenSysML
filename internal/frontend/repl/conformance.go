package repl

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// ConformanceMode reports the strictness the session judges notation at.
func (s *Session) ConformanceMode() diag.ConformanceMode {
	defer s.reading()()
	return s.ws.ConformanceMode()
}

// SetConformanceMode switches what the session asks of its model: whether
// notation no SysML v2 production admits is a warning or an error. It takes
// effect at once — the buffer is re-analyzed on the next request. A workspace
// holding a document as its interface record keeps its mode and says so.
func (s *Session) SetConformanceMode(mode diag.ConformanceMode) error {
	defer s.enter()()
	return s.ws.SetConformanceMode(mode)
}

// doStrict shows or sets the conformance mode, reporting what the buffer looks
// like under it: a mode change is asked in order to see its answer.
func (s *Session) doStrict(args []string) []string {
	if len(args) == 0 {
		return []string{fmt.Sprintf("strict: %s", onOff(s.ws.ConformanceMode().IsStrict()))}
	}
	var mode diag.ConformanceMode
	switch args[0] {
	case "on":
		mode = diag.ConformanceStrict
	case "off":
		mode = diag.ConformanceDefault
	default:
		return []string{fmt.Sprintf("error: unknown strict setting %q (want on or off)", args[0])}
	}
	if err := s.ws.SetConformanceMode(mode); err != nil {
		return []string{fmt.Sprintf("error: %v", err)}
	}
	out := []string{fmt.Sprintf("strict: %s", onOff(mode.IsStrict()))}
	return append(out, s.diagnosticLines()...)
}

// DisabledLints reports the codes of the lints the session leaves out of its
// diagnostics.
func (s *Session) DisabledLints() []string {
	defer s.reading()()
	return s.ws.DisabledLints()
}

// SetDisabledLints replaces the lints the session leaves out of its
// diagnostics; a code naming no lint is an error and changes nothing.
func (s *Session) SetDisabledLints(codes []string) error {
	defer s.enter()()
	return s.ws.SetDisabledLints(codes)
}

// EnabledLints reports the codes of the opt-in lints the session keeps in its
// diagnostics.
func (s *Session) EnabledLints() []string {
	defer s.reading()()
	return s.ws.EnabledLints()
}

// SetEnabledLints replaces the opt-in lints the session keeps in its
// diagnostics; a code naming no lint is an error and changes nothing.
func (s *Session) SetEnabledLints(codes []string) error {
	defer s.enter()()
	return s.ws.SetEnabledLints(codes)
}

// lintOn reports whether a lint is reported under the disabled and enabled sets.
func lintOn(code string, disabled, enabled []string) bool {
	if slices.Contains(disabled, code) {
		return false
	}
	return !passes.IsOptInLint(code) || slices.Contains(enabled, code)
}

// doLint lists the lints, each on or off, or switches one, reporting what the
// buffer looks like with it switched.
func (s *Session) doLint(args []string) []string {
	list := func() []string {
		disabled, enabled := s.ws.DisabledLints(), s.ws.EnabledLints()
		out := make([]string, 0, len(passes.LintCodes()))
		for _, code := range passes.LintCodes() {
			out = append(out, fmt.Sprintf("lint %s: %s", code, onOff(lintOn(code, disabled, enabled))))
		}
		return out
	}
	switch len(args) {
	case 0:
		return list()
	case 2:
	default:
		return []string{"error: usage: %lint [<code> on|off]"}
	}
	code := args[0]
	if err := passes.CheckLintCodes([]string{code}); err != nil {
		return []string{fmt.Sprintf("error: %v", err)}
	}
	without := func(codes []string) []string {
		return slices.DeleteFunc(slices.Clone(codes), func(c string) bool { return c == code })
	}
	disabled, enabled := without(s.ws.DisabledLints()), without(s.ws.EnabledLints())
	switch args[1] {
	case "on":
		if passes.IsOptInLint(code) {
			enabled = append(enabled, code)
		}
	case "off":
		if !passes.IsOptInLint(code) {
			disabled = append(disabled, code)
		}
	default:
		return []string{fmt.Sprintf("error: unknown lint setting %q (want on or off)", args[1])}
	}
	if err := s.ws.SetDisabledLints(disabled); err != nil {
		return []string{fmt.Sprintf("error: %v", err)}
	}
	if err := s.ws.SetEnabledLints(enabled); err != nil {
		return []string{fmt.Sprintf("error: %v", err)}
	}
	return append(list(), s.diagnosticLines()...)
}
