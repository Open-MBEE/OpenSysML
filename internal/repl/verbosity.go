package repl

import "fmt"

// Verbosity selects how much of a submission's analysis reaches the terminal.
//
// The REPL analyses the whole accumulated buffer on every submission, which is
// what the runtime needs but not what a user typing one declaration wants to
// read. Verbosity scopes the report back down to what was just typed.
type Verbosity int

const (
	// VerbosityQuiet reports only errors in the submission just made.
	VerbosityQuiet Verbosity = iota
	// VerbosityNormal adds warnings in that submission. The default.
	VerbosityNormal
	// VerbosityDebug reports every diagnostic over the whole buffer, at
	// buffer-absolute positions and with the pass that produced it.
	VerbosityDebug
)

var verbosityNames = map[Verbosity]string{
	VerbosityQuiet:  "quiet",
	VerbosityNormal: "normal",
	VerbosityDebug:  "debug",
}

func (v Verbosity) String() string {
	if name, ok := verbosityNames[v]; ok {
		return name
	}
	return fmt.Sprintf("Verbosity(%d)", int(v))
}

// ParseVerbosity maps a flag or %verbosity argument to a level.
func ParseVerbosity(s string) (Verbosity, error) {
	for v, name := range verbosityNames {
		if s == name {
			return v, nil
		}
	}
	return VerbosityNormal, fmt.Errorf("unknown verbosity %q (want quiet, normal or debug)", s)
}

// Verbosity reports the session's current output level.
func (s *Session) Verbosity() Verbosity {
	defer s.reading()()
	return s.verbosity
}

// SetVerbosity sets the session's output level.
func (s *Session) SetVerbosity(v Verbosity) {
	defer s.enter()()
	s.verbosity = v
}
