package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// ConformanceMode reports the strictness the session judges notation at.
func (s *Session) ConformanceMode() diag.ConformanceMode {
	defer s.reading()()
	return s.ws.ConformanceMode()
}

// SetConformanceMode switches what the session asks of its model: whether
// notation no SysML v2 production admits is a warning or an error. It takes
// effect at once — the buffer is re-analyzed on the next request.
func (s *Session) SetConformanceMode(mode diag.ConformanceMode) {
	defer s.enter()()
	s.ws.SetConformanceMode(mode)
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
	s.ws.SetConformanceMode(mode)
	out := []string{fmt.Sprintf("strict: %s", onOff(mode.IsStrict()))}
	return append(out, s.diagnosticLines()...)
}
