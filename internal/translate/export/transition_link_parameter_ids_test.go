package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A transition's synthesized link parameters spell the ids the conversion mints for them,
// `<transition>_linkparam` and `_triggerparam`, as its succession spells `_succession`.
func TestSynthesizedElementIDOfTransitionLinkParameters(t *testing.T) {
	sf := source.New("links.sysml", []byte(`package P {
	attribute def Sig;
	state def S {
		state idle;
		state busy;
		transition wake first idle accept Sig then busy;
	}
}
`))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	index := libs.NewModelIndex()
	index.AddDocument(sf.Name(), root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.ResolveDocument(sf.Name(), root)
	pkg, _ := index.DocumentRoot(sf.Name()).LookupLocal("P")
	s, _ := pkg.Scope.LookupLocal("S")
	wake, ok := s.Scope.LookupLocal("wake")
	if !ok {
		t.Fatal("no transition wake")
	}
	ownerID := func(sym *symbols.Symbol) string {
		if sym == wake {
			return "P__S__wake"
		}
		return ""
	}
	var got []string
	for _, rel := range model.OwnedRelationships(wake) {
		member := rel.Implicit.Member
		if member == nil || member.Expr == nil || member.Expr.Role != symbols.ExpressionTransitionLinkParameter {
			continue
		}
		id, ok := SynthesizedElementID(member, ownerID)
		if !ok {
			t.Errorf("link parameter %d has no synthesized id", member.Expr.Index)
		}
		got = append(got, id)
	}
	if want := []string{"P__S__wake_linkparam", "P__S__wake_triggerparam"}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("link parameter ids = %q, want %q", got, want)
	}
}
