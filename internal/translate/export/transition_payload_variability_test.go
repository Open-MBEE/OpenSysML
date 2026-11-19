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

// A trigger's payload — `accept Sig`, `accept after`, `accept at`, `accept when` — and a transition's
// link parameters answer isVariable true on every reader under the library, as the conversion states them.
func TestTransitionPayloadAndLinkParametersMayTimeVary(t *testing.T) {
	sf := source.New("vary.sysml", []byte(`package P {
	attribute def Sig;
	attribute x : ScalarValues::Boolean;
	state def S {
		state idle;
		state busy;
		transition go first idle accept Sig then busy;
		transition t1 first idle accept after 1 [SI::s] then busy;
		transition t2 first idle accept at 1 [SI::s] then busy;
		transition t3 first idle accept when x then busy;
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
	var parts []*symbols.Symbol
	owners := map[*symbols.Symbol]*symbols.Symbol{}
	for _, name := range []string{"go", "t1", "t2", "t3"} {
		transition, _ := s.Scope.LookupLocal(name)
		for _, rel := range model.OwnedRelationships(transition) {
			member := rel.Implicit.Member
			if member == nil || member.Expr == nil {
				continue
			}
			switch member.Expr.Role {
			case symbols.ExpressionTransitionLinkParameter:
				parts = append(parts, member)
				owners[member] = transition
			case symbols.ExpressionTriggerAction:
				for _, owned := range model.OwnedRelationships(member) {
					if payload := owned.Implicit.Member; payload != nil && payload.Expr != nil &&
						payload.Expr.Role == symbols.ExpressionPayload {
						parts = append(parts, payload)
						owners[payload] = member
					}
				}
			}
		}
	}
	if len(parts) != 12 {
		t.Fatalf("found %d parts, want two link parameters and one payload per transition", len(parts))
	}
	for i, part := range parts {
		if got, ok := model.ReflectiveFeatureValue(part, "isVariable"); !ok || !got.Bool {
			t.Errorf("part %d (%T) isVariable = %v (%t), want true", i, part.Decl, got, ok)
		}
		if !model.FeatureIsVariable(part) || !model.UsageMayTimeVary(part) ||
			!model.UsageMayTimeVaryForOwningType(part, owners[part]) {
			t.Errorf("part %d (%T): the readers disagree on its variability", i, part.Decl)
		}
	}
}
