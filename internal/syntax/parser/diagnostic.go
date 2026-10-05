package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Diagnostic is a parser-emitted syntax error. It is unified with the
// pass/validation diagnostic model in Plan 4; kept local here to avoid a
// premature cross-package dependency.
type Diagnostic struct {
	Span    source.Span
	Message string
	// Code classifies the diagnostic for the consumers that report it: every
	// warning carries one, and an error one only when it is a specific
	// finding; ErrorCode gives an error's reporting code.
	Code string
	// Fixes are the unambiguous edits resolving the diagnostic, offered by an
	// editor as quick fixes.
	Fixes []diag.Fix
}

// CodeSyntax is the code an error is reported under when its reporter gave it
// none: the general ill-formed-parse finding.
const CodeSyntax = "syntax"

// ErrorCode is the code an error is reported under: its own when it has one,
// else CodeSyntax.
func (d Diagnostic) ErrorCode() string {
	if d.Code != "" {
		return d.Code
	}
	return CodeSyntax
}

// Recurring diagnostic messages, spelled once so every site that reports the
// same syntax error words it the same way.
const (
	msgExpectedActionBrace  = "expected '}' after action expression"
	msgExpectedBodyClose    = "expected '}' to close body"
	msgExpectedBodyMember   = "expected a body member"
	msgExpectedCloseParen   = "expected ')'"
	msgExpectedShortName    = "expected short name after '<'"
	msgExpectedCloseAngle   = "expected '>'"
	msgExpectedLocaleString = "expected locale string"
	msgBindingEndExpression = "a binding end names a feature, not an expression; " +
		"declare a feature with the expression as its value and bind to that"
	msgConnectorEndExpression = "a connector end names a feature, not an expression; " +
		"name the feature itself and put any multiplicity before it"
	msgFlowEndExpression    = "a flow end names a feature, not an expression; name the feature itself"
	msgImportVisibility     = "import without a visibility indicator: SysML v2 requires public, private or protected before 'import'"
	msgDeferNotationRemoved = "the OpenSysML `defer <event>;` extension was removed: SysML v2 has no deferral notation; model a deferred signal with an ordered buffer (`item deferred : Sig[*] ordered;`), a do action whose accept loop keeps each occurrence while the state is active, and an exit action that sends each kept occurrence to self (docs/reference/sysml-v1-migration.md, Deferred signals)"
)

// Error codes: a specific ill-formed-parse finding a consumer reports by its
// own code rather than CodeSyntax.
const (
	// codeDeferNotationRemoved marks the removed `defer <event>;` state body member.
	codeDeferNotationRemoved = "defer-notation-removed"
)

// Warning codes.
const (
	codeReservedKeywordName = "reserved-keyword-name"
	// codeImportVisibility marks an `import` written without ImportPrefix's indicator.
	codeImportVisibility = "import-visibility"
	// codeEnumerationBodyMember marks a member EnumerationBody does not admit.
	codeEnumerationBodyMember = "enumeration-body-member"
	// codeNonstandardNotation marks notation the parser reads but the grammar
	// does not admit, such as a /* */ comment where no member may start.
	codeNonstandardNotation = "nonstandard-notation"
)
