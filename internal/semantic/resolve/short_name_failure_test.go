package resolve

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A redefinition whose target no general offers has no naming feature, and that
// settled answer is memoized like any other: every lookup passing through the
// scope asks each member for its derived short name, so re-deriving it there
// re-ran the failed lookup once per member per lookup.
func TestUnresolvedNamingFeatureIsDerivedOnce(t *testing.T) {
	const n = 16
	var src strings.Builder
	src.WriteString("package P {\n\tattribute v;\n\tpart x : Missing {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&src, "\t\tattribute :>> a%d = v;\n", i)
	}
	src.WriteString("\t}\n}\n")
	root := parsedRoot(t, "d.sysml", src.String())
	idx := symbols.NewIndexFromDoc("d.sysml", root)
	r := New(idx)
	r.ResolveDocument("d.sysml", root)
	x := scopeOf(t, scopeOf(t, idx.DocumentRoot("d.sysml"), "P"), "x")
	if got := len(r.derivedNamedMembers(x)); got != n {
		t.Fatalf("x has %d members named after a naming feature, want %d", got, n)
	}
	if short := r.shortNamedMembers(x, "v", nil); len(short) != 0 {
		t.Fatalf("v is the short name of %v, want none", short)
	}
	if allocs := testing.AllocsPerRun(20, func() { r.shortNamedMembers(x, "v", nil) }); allocs != 0 {
		t.Errorf("asking x's %d members for their short names allocates %.0f objects, want 0", n, allocs)
	}
}
