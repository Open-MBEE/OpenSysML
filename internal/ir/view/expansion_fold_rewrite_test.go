package view

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const foldRewriteFeatures = `
	part def Feature { part hidden; }
	part def Leaf { part repeated : Feature; }
`

func TestRepeatedExpansionFoldKeepsAssociationOccurrences(t *testing.T) {
	for _, tc := range []struct{ name, end, expose string }{
		{"association first", "", "expose ClockToWorld; expose World; expose Clock;"},
		{"nary association first", "end observer : Observer;", "expose ClockToWorld; expose World; expose Clock; expose Observer;"},
		{"definition first", "", "expose World; expose Clock; expose ClockToWorld;"},
		{"association stays a box", "", "expose ClockToWorld; expose World;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`package P {
				%s
				part def Clock { part shared : Leaf; action stepping { action update { doc /* update the clock */ } } }
				part def World { part clock : Clock; }
				part def Observer;
				connection def ClockToWorld { end clock : Clock; end world : World; %s }
				part worldA : World; part worldB : World;
				view v { %s expose worldA; expose worldB; }
			}`, foldRewriteFeatures, tc.end, tc.expose)
			checkFoldRewriteParity(t, src)
		})
	}
}

func TestRepeatedExpansionFoldKeepsExposedRelationshipOccurrences(t *testing.T) {
	for _, rel := range []string{
		`connection link connect (a, b, c) { part copy : Clock; }`,
		`dependency link from a, b to c { part copy : Clock; }`,
	} {
		t.Run(rel, func(t *testing.T) {
			src := `package P {
				` + foldRewriteFeatures + `
				part def Clock { part shared : Leaf; action update { doc /* update the clock */ } }
				part a { part copy : Clock; } part b { part copy : Clock; } part c { part copy : Clock; }
				` + rel + `
				view v { expose link; expose a; expose b; expose c; expose Clock; }
			}`
			checkFoldRewriteParity(t, src)
		})
	}
}

func TestRepeatedExpansionFoldKeepsReferenceAssociationOccurrences(t *testing.T) {
	for _, usage := range []string{
		`ref copy : ClockToWorld;`,
		`connection copy : ClockToWorld;`,
		`part copy : ClockToWorld;`,
	} {
		t.Run(usage, func(t *testing.T) {
			src := `package P {
				` + foldRewriteFeatures + `
				part def Clock { part shared : Leaf; action stepping { action update { doc /* update the clock */ } } }
				part def World { part clock : Clock; }
				connection def ClockToWorld { end clock : Clock; end world : World; }
				` + usage + `
				view v { expose ClockToWorld; expose copy; expose World; expose Clock; }
			}`
			checkFoldRewriteParity(t, src)
		})
	}
}

func TestRepeatedExpansionFoldRejectsRewriteRepresentatives(t *testing.T) {
	r, idx := loadSources(t, []string{"fold-key.sysml"}, [][]byte{[]byte(`package P {
		connection def Rewrite;
		part small;
		part large { part child; }
		view v { expose Rewrite; expose small; expose large; }
	}`)})
	v := lookup(t, idx, "P::v")
	rewrite := lookup(t, idx, "P::Rewrite")
	small := lookup(t, idx, "P::small")
	large := lookup(t, idx, "P::large")
	q := r.treeQuery(v, &Rendering{query: true}, []*symbols.Symbol{rewrite, small, large})
	f := &expansionFold{q: q, kind: KindTree, limit: 3,
		sizes: map[expansionSizeKey]int{}, representatives: map[treeQueryKey]expansionRepresentative{}}
	key, _, _ := f.representative(small, nil, expansion{}, 0, true)
	f.remember(key, 0, "P::small")
	if _, _, ok := f.representative(small, nil, expansion{}, 0, true); !ok {
		t.Fatal("safe occurrence was not remembered")
	}
	if key, _, ok := f.representative(rewrite, nil, expansion{}, 0, true); ok || key.sym != nil {
		t.Fatal("a rewritten box can become a representative")
	}
	q.ancestors[rewrite] = true
	if key := q.key(small, nil, expansion{}, true); key.sym == nil || key.rewrites == "" {
		t.Fatal("fold query did not retain the indexed rewrite signature")
	}
	if key, _, ok := f.representative(small, nil, expansion{}, 0, true); ok || key.sym != nil {
		t.Fatal("a rewritten box's descendant reused a representative")
	}
	delete(q.ids, rewrite)
	key, _, ok := f.representative(small, nil, expansion{}, 0, true)
	if ok || key.sym != nil {
		t.Fatal("unindexed rewrite ancestor permits folding")
	}
	f.remember(key, 0, "not drawn")
	if _, ok := f.representatives[treeQueryKey{}]; ok {
		t.Fatal("an empty key stored a representative")
	}
	if got := f.size(small, nil, expansion{}, 0, true); got != 1 {
		t.Fatalf("small count=%d, want 1", got)
	}
	if got := f.size(large, nil, expansion{}, 0, true); got != 2 {
		t.Fatalf("large count=%d, want 2: empty key reused small's size", got)
	}
}

func checkFoldRewriteParity(t *testing.T, src string) {
	t.Helper()
	r, idx := loadSources(t, []string{"fold-rewrite.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 3
	v := lookup(t, idx, "P::v")
	full, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	query, err := r.DrawnIn(v)
	if err != nil {
		t.Fatal(err)
	}
	if full.foldedOccurrences == 0 {
		t.Fatal("forced threshold did not fold any occurrence")
	}
	if !reflect.DeepEqual(full.Drawn(), query) {
		t.Error("folded rendering differs from DrawnIn after relationship lowering")
	}
	checkDrawnFoldRepresentatives(t, full)
}

func checkDrawnFoldRepresentatives(t *testing.T, rendering *Rendering) {
	t.Helper()
	drawn := map[string]*Node{}
	var index func([]*Node, string)
	index = func(nodes []*Node, parent string) {
		for _, node := range nodes {
			name := node.Name
			if parent != "" {
				name = parent + "::" + name
			}
			drawn[name] = node
			index(node.Children, name)
		}
	}
	index(rendering.Roots, "")
	for path, node := range drawn {
		before, _, folded := strings.Cut(node.Detail, "; repeated expansion folded")
		if !folded {
			continue
		}
		_, name, ok := strings.Cut(before, " shown at ")
		if !ok || drawn[name] == nil {
			t.Errorf("fold at %s names a representative that is not drawn: %q", path, node.Detail)
		} else if strings.Contains(drawn[name].Detail, "; repeated expansion folded") {
			t.Errorf("fold at %s names a folded representative: %q", path, node.Detail)
		}
	}
}
