package view

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func repeatedMemberModel(members string) string {
	return `package P {
		private import Views::*;
		part def Leaf { ` + members + ` }
		part def Outer { part repeated : Leaf; }
		part def Carrier { part outer : Outer; }
		part a : Carrier; part b : Carrier; part c : Carrier;
		view v { expose a; expose b; expose c; render asInterconnectionDiagram; }
	}`
}

func TestRepeatedExpansionFoldIgnoresUndrawnActions(t *testing.T) {
	for _, members := range []string{"", "action ignored;"} {
		r, idx := loadSources(t, []string{"members.sysml"}, [][]byte{[]byte(repeatedMemberModel(members))})
		v := lookup(t, idx, "P::v")
		exposed, err := r.model.ExposedElements(v)
		if err != nil {
			t.Fatal(err)
		}
		measured := &Rendering{}
		r.prepareExpansionFold(v, exposed, measured, KindInterconnection)
		if total := measured.expansionPlans[v].total; total != 9 {
			t.Fatalf("members=%q: measured %d nodes, want 9", members, total)
		}
		r.expansionFoldThreshold = -1
		baseline, err := r.Render(v)
		if err != nil {
			t.Fatal(err)
		}
		r.expansionFoldThreshold = 1
		got, err := r.Render(v)
		if err != nil {
			t.Fatal(err)
		}
		query, err := r.DrawnIn(v)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Drawn(), query) {
			t.Fatal("undrawn actions changed drawn-set parity")
		}
		if got.foldedOccurrences != 0 || got.ExpansionFoldNotice != "" {
			t.Fatalf("undrawn actions produced %d folds: %s", got.foldedOccurrences, got.ExpansionFoldNotice)
		}
		for _, node := range everyNode(got.Roots) {
			if strings.Contains(node.Detail, "repeated expansion folded") {
				t.Fatalf("undrawn action folded at %s", node.Name)
			}
		}
		for _, form := range got.Kind.SupportedForms() {
			actual, err := got.Write(form)
			if err != nil {
				t.Fatal(err)
			}
			want, err := baseline.Write(form)
			if err != nil {
				t.Fatal(err)
			}
			if actual != want {
				t.Fatalf("members=%q: undrawn actions changed %s output", members, form)
			}
		}
	}
}

func TestRepeatedExpansionFoldPartsBesideUndrawnActions(t *testing.T) {
	r, idx := loadSources(t, []string{"parts.sysml"}, [][]byte{[]byte(repeatedMemberModel("part child; action ignored;"))})
	r.expansionFoldThreshold = 1
	v := lookup(t, idx, "P::v")
	got, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	query, err := r.DrawnIn(v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Drawn(), query) {
		t.Fatal("drawn parts changed drawn-set parity")
	}
	if got.foldedOccurrences != 2 {
		t.Fatalf("got %d folds; want 2 omitted child occurrences", got.foldedOccurrences)
	}
	var noted, children int
	for _, node := range everyNode(got.Roots) {
		if node.Name == "child" {
			children++
		}
		if strings.Contains(node.Detail, "repeated expansion folded") {
			noted++
			if node.Name != "repeated" || len(node.Children) != 0 || node.Detail != "features of P::Leaf shown at P::a::outer::repeated; repeated expansion folded" {
				t.Fatalf("wrong drawn-member fold: %#v", node)
			}
		}
	}
	if noted != 2 || children != 1 {
		t.Fatalf("fold notes=%d drawn children=%d; want 2,1", noted, children)
	}
}

func TestRepeatedExpansionFoldIgnoresMixedNonstructuralMembers(t *testing.T) {
	src := repeatedMemberModel("attribute ignored;")
	r, idx := loadSources(t, []string{"mixed.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 1
	v := lookup(t, idx, "P::v")
	exposed, err := r.model.ExposedElements(v)
	if err != nil {
		t.Fatal(err)
	}
	out := &Rendering{Kind: KindInterconnection}
	f := r.prepareExpansionFold(v, exposed, out, KindInterconnection)
	f.walk.mixed = true
	f.limit = 10000
	f.sizes = map[expansionSizeKey]int{}
	if total := f.size(lookup(t, idx, "P::Outer::repeated"), nil, expansion{}, 0, true); total != 1 {
		t.Fatalf("mixed size includes rejected attribute: %d; want 1", total)
	}
	w := &featureWalk{r: r, view: v, ids: &nodeIDs{}, nodes: map[*symbols.Symbol][]*Node{},
		pins: map[*Node]map[*symbols.Symbol]string{}, parent: map[*Node]*Node{}, pruned: map[*symbols.Symbol]bool{},
		out: out, mixed: true, fold: f}
	for _, sym := range exposed {
		out.Roots = append(out.Roots, w.featureNode(sym, nil, expansion{}, 0, true))
	}
	if out.foldedOccurrences != 0 {
		t.Fatalf("mixed structural walk counted %d undrawn attribute folds", out.foldedOccurrences)
	}
	for _, node := range everyNode(out.Roots) {
		if node.Name == "ignored" || strings.Contains(node.Detail, "repeated expansion folded") {
			t.Fatalf("mixed walk folded or drew rejected member: %#v", node)
		}
	}
	src = strings.Replace(src, "asInterconnectionDiagram", "asMixedDiagram", 1)
	src = strings.Replace(src, "Views::*", "OpenSysMLRenderings::*", 1)
	r, idx = loadSources(t, []string{"mixed-view.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 1
	v = lookup(t, idx, "P::v")
	got, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindMixed {
		t.Fatalf("rendered %s; want mixed", got.Kind)
	}
	query, err := r.DrawnIn(v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Drawn(), query) {
		t.Fatal("mixed rendering changed drawn-set parity")
	}
}

func TestRepeatedExpansionFoldKeepsTreeParameters(t *testing.T) {
	src := repeatedMemberModel("in x;")
	src = strings.Replace(src, "part def Leaf", "constraint def Leaf", 1)
	src = strings.Replace(src, "part repeated : Leaf", "constraint repeated : Leaf", 1)
	src = strings.Replace(src, "asInterconnectionDiagram", "asTreeDiagram", 1)
	r, idx := loadSources(t, []string{"parameters.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 1
	v := lookup(t, idx, "P::v")
	got, err := r.Render(v)
	if err != nil {
		t.Fatal(err)
	}
	query, err := r.DrawnIn(v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Drawn(), query) || got.foldedOccurrences != 0 || got.ExpansionFoldNotice != "" {
		t.Fatal("tree parameters changed drawn-set parity or produced false folds")
	}
	var pinned int
	for _, node := range everyNode(got.Roots) {
		if node.Name == "repeated" {
			if len(node.Ports) != 1 || node.Ports[0].Name != "x" {
				t.Fatalf("parameter was not pinned: %#v", node.Ports)
			}
			pinned++
		}
	}
	if pinned != 3 {
		t.Fatalf("parameter pins=%d; want 3", pinned)
	}
}

func TestRepeatedExpansionFoldFallbackNamesOnlyDrawnDefinitions(t *testing.T) {
	src := repeatedMemberModel("")
	src = strings.Replace(src, "part def Leaf {  }", `
		part def Drawn { part child; }
		part def Ignored { action ignored; }
		part def Leaf :> Drawn, Ignored;`, 1)
	r, idx := loadSources(t, []string{"definitions.sysml"}, [][]byte{[]byte(src)})
	r.expansionFoldThreshold = 1
	v := lookup(t, idx, "P::v")
	exposed, err := r.model.ExposedElements(v)
	if err != nil {
		t.Fatal(err)
	}
	out := &Rendering{}
	f := r.prepareExpansionFold(v, exposed, out, KindInterconnection)
	leaf := lookup(t, idx, "P::Leaf")
	if len(r.drawnTypes(leaf)) != 0 {
		t.Fatal("definition has explicit typing; fallback was not exercised")
	}
	node := &Node{}
	f.note(out, leaf, node, expansionRepresentative{name: "earlier"})
	if node.Detail != "features of P::Drawn shown at earlier; repeated expansion folded" || len(out.foldedDefinitions) != 1 {
		t.Fatalf("undrawn definition included in fallback: %s", node.Detail)
	}
}
