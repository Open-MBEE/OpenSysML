package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// A reference subsetting carries the referenced feature's type along, so a
// referencing usage inherits from it as well as from its own kind's library
// definition.
func TestW10BReferenceSubsettingContributesABase(t *testing.T) {
	src := `package Test {
	part def ABlock { part x; }
	part def B {
		action a: ABlock;
	}
	ref b : B;
	perform b.a;
}`
	var at []source0
	for _, d := range w8cLibraryDiagnostics(t, "t.sysml", src) {
		if d.Message == "Duplicate of inherited member name 'self' from Action, Part" {
			at = append(at, source0{d.Span.Offset, d.Severity})
		}
	}
	// The chain `b.a` is a feature of its own, specialized as the perform is,
	// so it carries the conflict too.
	if len(at) != 3 {
		t.Fatalf("want the warning on `action a`, `perform b.a` and its chain, got %v", at)
	}
	for _, a := range at {
		if a.severity != diag.SeverityWarning {
			t.Errorf("want a warning, got %v", a.severity)
		}
	}
	for i, want := range []string{"perform b.a;", "b.a;"} {
		if got, off := at[i+1].offset, offsetOfW10B(src, want); got != off {
			t.Errorf("want warning %d at offset %d, got %d", i+1, off, got)
		}
	}
}

type source0 struct {
	offset   int
	severity diag.Severity
}

func offsetOfW10B(src, want string) int {
	for i := 0; i+len(want) <= len(src); i++ {
		if src[i:i+len(want)] == want {
			return i
		}
	}
	return -1
}
