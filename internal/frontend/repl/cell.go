package repl

import (
	"slices"
	"strings"
)

// RenderResult produces the lines the prompt prints for a submission at the
// given verbosity.
func RenderResult(r Result, v Verbosity) []string { return renderResult(r, v) }

// Failed reports whether a submission was refused or had an error at the given
// verbosity: what the prompt prints for it reports a failure, not a declaration.
// Rejected reports whether the submission was refused or has an error of its
// own, whatever the verbosity: errors elsewhere in the model, which Failed
// counts at debug verbosity, do not reject a submission that joined it.
func (r Result) Rejected() bool {
	return r.Refused != nil || hasError(scopedDiagnostics(r, VerbosityNormal))
}

func (r Result) Failed(v Verbosity) bool {
	if r.Refused != nil {
		return true
	}
	if v >= VerbosityDebug {
		return hasError(r.Diagnostics)
	}
	return hasError(scopedDiagnostics(r, v))
}

// KnownMeta reports whether name is a `%` command this build serves, so a
// caller can tell a command that printed guidance from one that was unknown.
func KnownMeta(name string) bool {
	i := slices.IndexFunc(metaCommandTable, func(c metaCommand) bool { return c.name == name })
	return i >= 0 && metaCommandTable[i].served()
}

// MetaArgs reads a meta command line as the prompt does: the command, then its
// arguments, a quoted one kept whole.
func MetaArgs(line string) []string { return parseArgs(strings.TrimSpace(line)) }
