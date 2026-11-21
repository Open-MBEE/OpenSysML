package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A constructor owns only its result, whose Feature owns one argument feature per argument,
// each valued by it (KerML 8.3.4.8.3, KerML.xtext ConstructorResultMember → ArgumentMember).
func TestConstructorResultOwnsTheArgumentFeatures(t *testing.T) {
	sf := source.New("ctor.sysml", []byte(`package P {
	attribute def Point { attribute x : ScalarValues::Integer; attribute y : ScalarValues::Integer; }
	attribute p = new Point(x = 1, y = 2);
}
`))
	root := parser.New(sf).ParseFile()
	graph, err := ToRDF(sf, root)
	if err != nil {
		t.Fatal(err)
	}
	var constructor rdf.Term
	for _, subject := range graph.Subjects() {
		if graph.Type(subject) == rdf.SysML+"ConstructorExpression" {
			constructor = subject
		}
	}
	if constructor.Value == "" {
		t.Fatal("no ConstructorExpression written")
	}
	var owned []string
	for _, membership := range graph.Objects(constructor, rdf.SysML+"ownedRelationship") {
		owned = append(owned, rdf.LocalName(graph.Type(membership)))
	}
	if got := strings.Join(owned, ","); got != "Membership,ReturnParameterMembership" {
		t.Errorf("the constructor owns %s, want Membership,ReturnParameterMembership", got)
	}
	result, ok := graph.Object(constructor, rdf.SysML+"result")
	if !ok {
		t.Fatal("the constructor has no result")
	}
	features := graph.Objects(result, rdf.SysML+"ownedFeature")
	arguments := graph.Objects(constructor, rdf.SysML+"argument")
	if len(features) != 2 || len(arguments) != 2 {
		t.Fatalf("the result owns %d features for %d arguments, want 2 and 2", len(features), len(arguments))
	}
	for i, feature := range features {
		if membership, _ := graph.Object(feature, rdf.SysML+"owningRelationship"); rdf.LocalName(graph.Type(membership)) != "FeatureMembership" {
			t.Errorf("argument feature %d is owned through %s, want FeatureMembership", i, rdf.LocalName(graph.Type(membership)))
		}
		if !strings.Contains(feature.Value, "in"+string(rune('0'+i))) {
			t.Errorf("argument feature %d is %s, want the in%d part of the constructor", i, feature.Value, i)
		}
		var valued bool
		for _, membership := range graph.Objects(feature, rdf.SysML+"ownedRelationship") {
			if value, ok := graph.Object(membership, rdf.SysML+"value"); ok && value.Value == arguments[i].Value {
				valued = true
			}
		}
		if !valued {
			t.Errorf("argument feature %d is not valued by argument %d", i, i)
		}
	}
}
