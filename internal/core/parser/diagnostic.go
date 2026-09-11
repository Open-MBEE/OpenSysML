package parser

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Diagnostic is a parser-emitted syntax error. It is unified with the
// pass/validation diagnostic model in Plan 4; kept local here to avoid a
// premature cross-package dependency.
type Diagnostic struct {
	Span    source.Span
	Message string
	// Code classifies a warning for the consumers that report it; errors carry
	// the "syntax" code their reporters give them.
	Code string
	// Fixes are the unambiguous edits resolving the diagnostic, offered by an
	// editor as quick fixes.
	Fixes []quickfix.Fix
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
	msgImportVisibility     = "import without a visibility indicator: SysML v2 requires public, private or protected before 'import'"
)

// Warning codes.
const (
	codeReservedKeywordName = "reserved-keyword-name"
	// codeImportVisibility marks an `import` written without ImportPrefix's indicator.
	codeImportVisibility = "import-visibility"
	// codeEnumerationBodyMember marks a member EnumerationBody does not admit.
	codeEnumerationBodyMember = "enumeration-body-member"
)
