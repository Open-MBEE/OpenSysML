package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestImpliedEndNamesComeFromTheKindsBase: a succession's minted ends are HappensBefore's,
// a binding's SelfLink's; a kind with no base keeps the binary connector's names.
func TestImpliedEndNamesComeFromTheKindsBase(t *testing.T) {
	m, _ := buildModelWithStdlib(t, `package P { part p; }`)
	for _, tc := range []struct {
		kind ast.UsageKind
		want [2]string
	}{
		{ast.UsageSuccession, [2]string{"earlierOccurrence", "laterOccurrence"}},
		{ast.UsageBinding, [2]string{"thisThing", "sameThing"}},
		{ast.UsageConnection, [2]string{"source", "target"}},
	} {
		for i, want := range tc.want {
			if got := m.ImpliedEndName(tc.kind, i); got != want {
				t.Errorf("ImpliedEndName(%v, %d) = %q, want %q", tc.kind, i, got, want)
			}
		}
	}
	if got := m.ImpliedEndName(ast.UsageSuccession, 2); got != "" {
		t.Errorf("a binary succession has no third end, got %q", got)
	}
}
