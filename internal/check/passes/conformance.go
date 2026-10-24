package passes

import "github.com/Open-MBEE/OpenSysML/internal/syntax/diag"

// strictSeverity is the severity of a finding strict conformance makes an
// error: extension notation, and a namespace whose memberships are not
// distinguishable (KerML 8.3.2.4.5). By default it is a warning.
func strictSeverity(mode diag.ConformanceMode) diag.Severity {
	if mode.IsStrict() {
		return diag.SeverityError
	}
	return diag.SeverityWarning
}
