package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A conformance check against a named type compares the candidate's and its
// memoized supertypes' owner chains to the name: no qualified name is built.
func TestConformsByNameDoesNotBuildNames(t *testing.T) {
	idx := symbols.NewIndex()
	addTestDoc(t, idx, "d.sysml", `package T {
		part def Base;
		part def Mid :> Base;
		part def Leaf :> Mid;
		part def Other;
	}`)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	leaf := idx.LookupQualified("T::Leaf")[0]

	cases := []struct {
		fqn  string
		want bool
	}{{"T::Base", true}, {"T::Mid", true}, {"T::Leaf", true}, {"T::Other", false}, {"T::Absent", false}, {"Base", false}}
	for _, c := range cases {
		if got := m.conformsByName(leaf, c.fqn); got != c.want {
			t.Fatalf("conformsByName(Leaf, %s) = %v, want %v", c.fqn, got, c.want)
		}
		if allocs := testing.AllocsPerRun(20, func() { m.conformsByName(leaf, c.fqn) }); allocs != 0 {
			t.Errorf("a repeated conformsByName(Leaf, %s) allocates %.0f objects, want 0", c.fqn, allocs)
		}
	}
}
