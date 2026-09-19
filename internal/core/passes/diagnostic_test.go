package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func TestSeverityString(t *testing.T) {
	cases := map[diag.Severity]string{
		diag.SeverityError:   "error",
		diag.SeverityWarning: "warning",
		diag.SeverityInfo:    "info",
		diag.SeverityHint:    "hint",
		diag.Severity(999):   "unknown",
	}
	for sev, want := range cases {
		if got := sev.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", int(sev), got, want)
		}
	}
}

func TestDiagnosticFields(t *testing.T) {
	d := diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     source.Span{Offset: 3, Len: 5},
		Message:  "boom",
		Code:     "unresolved",
		Source:   "name-resolution",
	}
	if d.Severity != diag.SeverityError || d.Span.Offset != 3 || d.Span.Len != 5 {
		t.Fatalf("unexpected diag: %+v", d)
	}
	if d.Message != "boom" || d.Code != "unresolved" || d.Source != "name-resolution" {
		t.Fatalf("unexpected diag: %+v", d)
	}
}
