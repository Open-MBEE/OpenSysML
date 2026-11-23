package passes

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestReportedSetSeesDiagnosticsAppendedDirectly(t *testing.T) {
	var set reportedSet
	diags := []diag.Diagnostic{{Span: source.Span{Offset: 1}, Message: "direct"}}
	if set.add(diags, 1, "direct") {
		t.Fatal("a diagnostic appended directly was reported again")
	}
	if !set.add(diags, 2, "direct") || !set.add(diags, 1, "other") {
		t.Fatal("a different offset or message was taken for a repeat")
	}
	if set.add(diags, 2, "direct") {
		t.Fatal("a recorded pair was reported again")
	}
}

// BenchmarkAppendUnique reports distinct diagnostics: a scan of the list per
// report grows with the square of their number.
func BenchmarkAppendUnique(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("diagnostics=%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tc := &typeChecker{}
				for offset := range count {
					tc.appendUnique(diag.Diagnostic{Span: source.Span{Offset: offset}, Message: "m"})
				}
			}
		})
	}
}
