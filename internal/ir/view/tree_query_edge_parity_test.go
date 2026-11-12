package view

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestTreeDrawnQueryKeepsAssociationOccurrences(t *testing.T) {
	const model = `package P {
		part def Clock { action stepping { action update { %s } } }
		part def World { part clock : Clock; }
		part def Observer;
		connection def ClockToWorld { end clock : Clock; end world : World; %s }
		view v { %s }
	}`
	for _, annotation := range []string{
		`rep language "Javascript" /* time = time + step */`,
		`comment note /* update the clock */`,
		`doc /* update the clock */`,
	} {
		for _, tc := range []struct {
			name, end, expose string
			linked            bool
		}{
			{"association first", "", "expose ClockToWorld; expose World; expose Clock;", false},
			{"nary association first", "end observer : Observer;", "expose ClockToWorld; expose World; expose Clock; expose Observer;", false},
			{"definition first", "", "expose World; expose Clock; expose ClockToWorld;", true},
			{"association stays a box", "", "expose ClockToWorld; expose World;", true},
		} {
			t.Run(annotation+"/"+tc.name, func(t *testing.T) {
				src := fmt.Sprintf(model, annotation, tc.end, tc.expose)
				r, idx := loadSources(t, []string{"association-query.sysml"}, [][]byte{[]byte(src)})
				v := lookup(t, idx, "P::v")
				full, err := r.Render(v)
				if err != nil {
					t.Fatal(err)
				}
				query, err := r.DrawnIn(v)
				if err != nil {
					t.Fatal(err)
				}
				note := r.annotatingMembers(lookup(t, idx, "P::Clock::stepping::update"))[0]
				if full.Drawn().Edge(note) != tc.linked || full.Drawn().Steers(note.Origin(), EdgeAnchor) != tc.linked {
					t.Fatalf("full rendering annotation link: got %v, want %v", full.Drawn().Edge(note), tc.linked)
				}
				if !reflect.DeepEqual(full.Drawn(), query) {
					t.Fatalf("drawn sets differ: full annotation edge=%v, query annotation edge=%v", full.Drawn().Edge(note), query.Edge(note))
				}
			})
		}
	}
}

func TestTreeDrawnQueryKeepsExposedRelationshipOccurrences(t *testing.T) {
	for _, rel := range []string{
		`connection link connect (a, b, c) { part copy : Clock; }`,
		`dependency link from a, b to c { part copy : Clock; }`,
	} {
		t.Run(rel, func(t *testing.T) {
			src := `package P {
				part def Clock { action update { doc /* update the clock */ } }
				part a : Clock; part b : Clock; part c : Clock;
				` + rel + `
				view v { expose link; expose a; expose b; expose c; expose Clock; }
			}`
			r, idx := loadSources(t, []string{"relationship-query.sysml"}, [][]byte{[]byte(src)})
			v := lookup(t, idx, "P::v")
			full, err := r.Render(v)
			if err != nil {
				t.Fatal(err)
			}
			query, err := r.DrawnIn(v)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(full.Drawn(), query) {
				t.Fatal("drawn sets differ after exposed relationship lowering")
			}
			relationship := lookup(t, idx, "P::link")
			node := full.Roots[0]
			if node.Kind != connectionDotKind && node.Kind != dependencyDotKind {
				t.Fatalf("relationship stayed a box: %s", node.Kind)
			}
			if len(node.Children) != 0 {
				t.Fatal("lowered relationship kept its children")
			}
			clock := lookup(t, idx, "P::Clock")
			update := lookup(t, idx, "P::Clock::update")
			q := r.treeQuery(v, &Rendering{query: true}, []*symbols.Symbol{relationship, clock})
			q.ancestors[relationship] = true
			inside := q.key(update, clock, expansion{relationship: true}, false)
			delete(q.ancestors, relationship)
			outside := q.key(update, clock, expansion{}, false)
			if inside == outside {
				t.Fatal("query shares descendants across a lowered relationship")
			}
		})
	}
}

func TestTreeDrawnQueryKeepsReferenceAssociationOccurrences(t *testing.T) {
	for _, usage := range []string{
		`ref copy : ClockToWorld;`,
		`connection copy : ClockToWorld;`,
		`part copy : ClockToWorld;`,
	} {
		t.Run(usage, func(t *testing.T) {
			src := `package P {
				part def Clock { action stepping { action update { doc /* update the clock */ } } }
				part def World { part clock : Clock; }
				connection def ClockToWorld { end clock : Clock; end world : World; }
				` + usage + `
				view v { expose ClockToWorld; expose copy; expose World; expose Clock; }
			}`
			r, idx := loadSources(t, []string{"reference-association-query.sysml"}, [][]byte{[]byte(src)})
			v := lookup(t, idx, "P::v")
			full, err := r.Render(v)
			if err != nil {
				t.Fatal(err)
			}
			query, err := r.DrawnIn(v)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(full.Drawn(), query) {
				t.Fatalf("drawn sets differ: full edges=%d, query edges=%d", len(full.Drawn().edges), len(query.edges))
			}
		})
	}
}

func TestTreeDrawnQueryMissingRewriteTokenDisablesReuse(t *testing.T) {
	r, idx := loadSources(t, []string{"missing-rewrite-token.sysml"}, [][]byte{[]byte(`package P {
		part def Clock { action update; }
		connection def Known;
		connection def First;
		connection def Second;
	}`)})
	clock := lookup(t, idx, "P::Clock")
	update := lookup(t, idx, "P::Clock::update")
	known := lookup(t, idx, "P::Known")
	for _, name := range []string{"First", "Second"} {
		t.Run(name, func(t *testing.T) {
			q := &treeQuery{
				r: r, treeQueryGraph: &treeQueryGraph{ids: map[*symbols.Symbol]int{known: 0}},
				ancestors: expansion{known: true}, rewritten: map[*symbols.Symbol]bool{},
				nodes: map[treeQueryKey]treeQueryNode{},
			}
			indexed := q.key(update, clock, expansion{}, false)
			cached := &Node{}
			q.remember(indexed, 0, cached)
			if q.node(indexed, 0) != cached {
				t.Fatal("indexed rewrite token zero did not reuse its completed node")
			}
			q.ancestors = expansion{lookup(t, idx, "P::"+name): true}
			missing := q.key(update, clock, expansion{}, false)
			if q.node(missing, 0) != nil {
				t.Fatal("unindexed rewrite ancestor reused token zero's node")
			}
			q.remember(missing, 0, &Node{})
			if len(q.nodes) != 1 {
				t.Fatal("unindexed rewrite ancestor stored a reusable node")
			}
			poison := &Node{}
			q.nodes[treeQueryKey{}] = treeQueryNode{node: poison, depth: 5}
			if q.node(missing, 5) != nil {
				t.Fatal("uncacheable key looked up an existing entry")
			}
			q.remember(missing, 0, &Node{})
			if got := q.nodes[treeQueryKey{}]; got.node != poison || got.depth != 5 {
				t.Fatal("uncacheable key replaced an existing entry")
			}
			if q.node(indexed, 0) != cached {
				t.Fatal("uncacheable occurrence changed an indexed context")
			}
		})
	}
}
