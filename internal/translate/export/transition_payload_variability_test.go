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

// A bare `accept Sig`'s payload and a transition's link parameters answer isVariable true on the
// native walk under the library, as the conversion states them and the pilot exports them.
func TestTransitionPayloadAndLinkParametersMayTimeVary(t *testing.T) {
	sf := source.New("vary.sysml", []byte(`package P {
	attribute def Sig;
	state def S {
		state idle;
		state busy;
		transition go first idle accept Sig then busy;
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
	transition, _ := s.Scope.LookupLocal("go")
	var parts []*symbols.Symbol
	for _, rel := range model.OwnedRelationships(transition) {
		member := rel.Implicit.Member
		if member == nil || member.Expr == nil {
			continue
		}
		switch member.Expr.Role {
		case symbols.ExpressionTransitionLinkParameter:
			parts = append(parts, member)
		case symbols.ExpressionTriggerAction:
			for _, owned := range model.OwnedRelationships(member) {
				if payload := owned.Implicit.Member; payload != nil && payload.Expr != nil &&
					payload.Expr.Role == symbols.ExpressionPayload {
					parts = append(parts, payload)
				}
			}
		}
	}
	if len(parts) != 3 {
		t.Fatalf("found %d parts, want two link parameters and one payload", len(parts))
	}
	for i, part := range parts {
		if got, ok := model.ReflectiveFeatureValue(part, "isVariable"); !ok || !got.Bool {
			t.Errorf("part %d isVariable = %v (%t), want true", i, got, ok)
		}
	}
}
