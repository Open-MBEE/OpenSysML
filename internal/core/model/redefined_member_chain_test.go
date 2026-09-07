package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// bindingAt resolves the reference whose text ends at the first occurrence of
// anchor, reporting the symbol its last segment reaches.
func bindingAt(t *testing.T, ws *Workspace, uri, anchor string) *symbols.Symbol {
	t.Helper()
	doc := ws.Document(uri)
	if doc == nil || doc.Scope == nil {
		t.Fatal("the document has no scope")
	}
	offset := strings.Index(string(doc.Content), anchor)
	if offset < 0 {
		t.Fatalf("anchor %q not in source", anchor)
	}
	offset += len(anchor) - 1
	for _, ref := range resolve.References(doc.AST, doc.Scope) {
		sp := ref.QN.Span()
		if offset < sp.Offset || offset >= sp.End() {
			continue
		}
		sym, ok := ws.ResolveReferenceInDoc(uri, ref)
		if !ok || sym == nil {
			t.Fatalf("%q did not resolve", anchor)
		}
		return sym
	}
	t.Fatalf("no reference written at %q", anchor)
	return nil
}

// ownerPath spells sym as the owners it is declared under, by effective name.
func ownerPath(sym *symbols.Symbol) string {
	path := sym.Name
	for s := sym.OwnerScope; s != nil; s = s.Parent() {
		if o := s.Owner(); o != nil {
			path = o.Name + "::" + path
		}
	}
	return path
}

const redefiningUsageModel = `package Repro {
	item def Edge;
	item def Vertex;
	item def Surface;
	item def Polygon {
		item edges : Edge [3..*] {
			item vertices : Vertex [2];
		}
	}
	item def Disc :> Surface {
		item edges : Edge [1];
	}
	item def Other {
		item faces : Polygon [*];
	}
	item def Polyhedron {
		item faces : Polygon [2..*] {
			ref :>> REDEFINED, Other::faces::edges;
		}
		item edges = faces.edges;
		item ff : Polygon [1] :> faces { item :>> Polygon::edges, faces::edges [3..4]; }
		item base : Disc [1] :> faces {
			ref :>> Disc::edges, Other::faces::edges;
		}
		item baseEdges = base.edges;
		item cf : Surface [1] :> faces;
		item cfEdges = cf.edges;
		item firstVertices = edges#(1).vertices;
		item wallEdges = base#(1).edges;
	}
}`

// An anonymous member is named by what it redefines (KerML 7.3.4.5), so both
// `faces.edges` and `faces::edges` reach it, as do valued, indexed and subsetting chains.
func TestChainThroughRedefiningUsageBindsToTheRedefiningMember(t *testing.T) {
	for _, spelling := range []string{"Polygon::edges", "edges"} {
		t.Run(spelling, func(t *testing.T) {
			ws := NewWorkspace()
			uri := "file:///repro.sysml"
			ws.Open(uri, []byte(strings.Replace(redefiningUsageModel, "REDEFINED", spelling, 1)), 1)
			if got := errorsIn(t, ws, uri); got != "" {
				t.Fatalf("unexpected error(s): %s", got)
			}
			for anchor, want := range map[string]string{
				"= faces.edges":                             "Repro::Polyhedron::faces::edges",
				"Polygon::edges, faces::edges":              "Repro::Polyhedron::faces::edges",
				"= base.edges":                              "Repro::Polyhedron::base::edges",
				"= cf.edges":                                "Repro::Polyhedron::faces::edges",
				"= edges#(1).vertices":                      "Repro::Polygon::edges::vertices",
				"= base#(1).edges":                          "Repro::Polyhedron::base::edges",
				":>> " + spelling:                           "Repro::Polygon::edges",
				":>> " + spelling + ", Other::faces::edges": "Repro::Polygon::edges",
				":>> Disc::edges":                           "Repro::Disc::edges",
			} {
				if got := ownerPath(bindingAt(t, ws, uri, anchor)); got != want {
					t.Errorf("%s binds to %s, want %s", anchor, got, want)
				}
			}
			chain := bindingAt(t, ws, uri, "= faces.edges")
			if bindingAt(t, ws, uri, "Polygon::edges, faces::edges") != chain {
				t.Errorf("faces::edges binds to a different symbol than faces.edges")
			}
			if !chain.EffectiveName() || chain.Naming != symbols.NamedByRedefinition {
				t.Errorf("the redefining member is named %q by %v, want by its redefinition", chain.Name, chain.Naming)
			}
			if bindingAt(t, ws, uri, ":>> "+spelling) == chain {
				t.Errorf("the chain reached the masked Polygon::edges")
			}
		})
	}
}

// A member neither the usage nor anything it generalizes declares stays
// unresolved, through either spelling.
func TestChainThroughRedefiningUsageReportsAbsentMembers(t *testing.T) {
	src := strings.Replace(redefiningUsageModel, "REDEFINED", "Polygon::edges", 1)
	src = strings.Replace(src, "= faces.edges;", "= faces.nothere;", 1)
	src = strings.Replace(src, "Polygon::edges, faces::edges", "Polygon::edges, faces::nothere", 1)
	src = strings.Replace(src, "= edges#(1).vertices;", "= base#(1).nothere;", 1)
	ws := NewWorkspace()
	uri := "file:///repro.sysml"
	ws.Open(uri, []byte(src), 1)
	got := errorsIn(t, ws, uri)
	for _, want := range []string{
		"unresolved member: nothere",
		"unresolved reference: faces::nothere",
		"unresolved member: nothere",
	} {
		i := strings.Index(got, want)
		if i < 0 {
			t.Fatalf("missing %q in: %s", want, got)
		}
		got = got[i+len(want):]
	}
	if strings.Contains(got, "unresolved") {
		t.Fatalf("unexpected further unresolved name(s): %s", got)
	}
}
