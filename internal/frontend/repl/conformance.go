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

// doLint lists the lints, each on or off, or switches one, reporting what the
// buffer looks like with it switched.
func (s *Session) doLint(args []string) []string {
	disabled := s.ws.DisabledLints()
	list := func() []string {
		cur := s.ws.DisabledLints()
		out := make([]string, 0, len(passes.LintCodes()))
		for _, code := range passes.LintCodes() {
			out = append(out, fmt.Sprintf("lint %s: %s", code, onOff(!slices.Contains(cur, code))))
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
	next := slices.DeleteFunc(slices.Clone(disabled), func(c string) bool { return c == code })
	switch args[1] {
	case "on":
	case "off":
		next = append(next, code)
	default:
		return []string{fmt.Sprintf("error: unknown lint setting %q (want on or off)", args[1])}
	}
	if err := s.ws.SetDisabledLints(next); err != nil {
		return []string{fmt.Sprintf("error: %v", err)}
	}
	return append(list(), s.diagnosticLines()...)
}
