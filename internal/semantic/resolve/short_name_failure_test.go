package resolve

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A redefinition no general offers a target for has a settled empty short name,
// memoized so lookups through its scope do not re-run the failed lookup.
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

// The settled empty short name is dropped when another document makes the
// naming feature resolve, and recorded again when it stops resolving.
func TestUnresolvedNamingFeatureFollowsTheDocumentItReads(t *testing.T) {
	const unresolved, resolved = "package B { }", "package B { part def T :> A::P; }"
	w := newTrackedIndex(t, map[string]string{
		"a.sysml": "package A { private import B::*; part def P { attribute <m> mass; } part x : T { attribute :>> mass; attribute w :> m; } }",
		"b.sysml": unresolved,
	})
	naming := func() (*symbols.Symbol, string) {
		var feature *symbols.Symbol
		var short string
		w.r.InDocument("a.sysml", func() {
			w.r.ResolveDocument("a.sysml", w.roots["a.sysml"])
			x := scopeOf(t, scopeOf(t, w.idx.DocumentRoot("a.sysml"), "A"), "x")
			redefinition := w.r.derivedNamedMembers(x)[0]
			feature, short = w.r.namingFeatureOf(redefinition), w.r.derivedShortName(redefinition)
		})
		return feature, short
	}
	m := w.idx.LookupQualified("A::P::mass")[0]
	for i, step := range []struct {
		src       string
		feature   *symbols.Symbol
		shortName string
	}{{unresolved, nil, ""}, {resolved, m, "m"}, {unresolved, nil, ""}} {
		if i > 0 {
			w.put("b.sysml", step.src)
			m = w.idx.LookupQualified("A::P::mass")[0]
			if step.feature != nil {
				step.feature = m
			}
		}
		if feature, short := naming(); feature != step.feature || short != step.shortName {
			t.Errorf("step %d: naming feature %v, short name %q; want %v, %q", i, feature, short, step.feature, step.shortName)
		}
	}
}
