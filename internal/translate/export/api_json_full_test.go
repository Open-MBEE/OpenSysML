package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestFullAPIJSONMapsEvaluatedMembershipsToGraphSubjects(t *testing.T) {
	const model = `package P {
		part def Base { part size; }
		part def Derived specializes Base { part mass; }
	}`
	file := source.New("m.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var pkg *symbols.Symbol
	for _, sym := range res.Index().LookupQualified("P") {
		pkg = sym
		break
	}
	if pkg == nil {
		t.Fatal("resolver omitted package P")
	}
	evaluator := metamodel.New(res, encoders[0].ids.model)
	value, ok := evaluator.Property(metamodel.ElementOf(pkg), "Namespace", "membership")
	if !ok {
		t.Fatal("evaluator did not compute Namespace::membership")
	}
	subjects := buildSemanticSubjects(graph, res, encoders, encoders[0].ids.model)
	for _, member := range value.Values {
		if member.Kind != metamodel.MembershipValue {
			t.Fatalf("membership contains %#v, want a membership", member)
		}
		if term := subjects.byMembership[member.Membership]; term.Value == "" {
			fqn := symbols.FQNOf(member.Membership.Member)
			sameFQN := false
			for mapped := range subjects.byMembership {
				if mapped.Member != nil && symbols.FQNOf(mapped.Member) == fqn {
					sameFQN = true
					break
				}
			}
			t.Fatalf("membership %q has no graph identity (mapped member with same name: %t)", fqn, sameFQN)
		}
	}
}
