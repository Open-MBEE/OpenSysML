package diag

import "fmt"

// ConformanceMode is how strictly a document is judged against the pinned
// SysML v2 and KerML grammars. It is threaded explicitly through the analysis
// pipeline — no global, no environment read — so every entry point states the
// question it is asking.
type ConformanceMode int

const (
	// ConformanceDefault accepts the OpenSysML notation extensions, reporting each as a
	// warning. It is the zero value: a caller that says nothing gets it.
	ConformanceDefault ConformanceMode = iota
	// ConformanceStrict answers "is this conforming SysML v2?": notation no pinned
	// production admits is an error, so a file using it is rejected.
	ConformanceStrict
)

// String returns the mode's spelling, the same one every entry point accepts.
func (m ConformanceMode) String() string {
	switch m {
	case ConformanceStrict:
		return "strict"
	case ConformanceDefault:
		return "default"
	default:
		return fmt.Sprintf("ConformanceMode(%d)", int(m))
	}
}

// IsStrict reports whether extension notation is an error in this mode.
func (m ConformanceMode) IsStrict() bool { return m == ConformanceStrict }

// ConformanceModeOf maps a boolean surface (a CLI flag, an LSP setting, a request field)
// onto the mode it selects.
func ConformanceModeOf(strict bool) ConformanceMode {
	if strict {
		return ConformanceStrict
	}
	return ConformanceDefault
}

// ParseConformanceMode reads a mode by name, as the CLI and the REPL spell it. The empty
// string is the default mode, so an unset setting is not an error.
func ParseConformanceMode(s string) (ConformanceMode, error) {
	switch s {
	case "", "default":
		return ConformanceDefault, nil
	case "strict":
		return ConformanceStrict, nil
	default:
		return ConformanceDefault, fmt.Errorf("unknown conformance mode %q: want \"default\" or \"strict\"", s)
	}
}
