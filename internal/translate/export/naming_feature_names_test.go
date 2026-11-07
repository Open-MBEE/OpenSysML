package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func parseFixture(t *testing.T, src string) *rdf.Graph {
	t.Helper()
	file := source.New("n.sysml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDF(file, root)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// A `satisfy … by lander` subject declares no name: KerML Feature::effectiveName takes the
// redefined subject parameter's, `subj` (§8.3.3.3.4 namingFeature), as does its memberName.
func TestSubjectByReferenceIsNamedByTheSubjectItRedefines(t *testing.T) {
	graph := parseFixture(t, `package P {
    part def Base;
    requirement def R { subject subj : Base; }
    part lander : Base;
    satisfy requirement r : R by lander;
}
`)
	var subjects []rdf.Term
	for _, subject := range graph.Subjects() {
		m, ok := graph.Object(subject, rdf.RDFType)
		if _, owns := graph.Object(subject, rdf.SysML+"ownedMemberElement"); ok && owns && m.Value == rdf.SysML+"SubjectMembership" {
			subjects = append(subjects, subject)
		}
	}
	if len(subjects) != 2 {
		var members []string
		for _, membership := range subjects {
			usage, _ := graph.Object(membership, rdf.SysML+"ownedMemberElement")
			members = append(members, membership.Value+" -> "+usage.Value)
		}
		t.Fatalf("the model writes %d SubjectMemberships, want R's and r's: %v", len(subjects), members)
	}
	for _, membership := range subjects {
		name, ok := graph.Object(membership, rdf.SysML+pMemberName)
		if !ok || name.Value != "subj" {
			t.Errorf("%s memberName = %v (present %t), want subj", membership.Value, name, ok)
		}
		usage, ok := graph.Object(membership, rdf.SysML+"ownedMemberElement")
		if !ok {
			t.Fatalf("%s owns no member", membership.Value)
		}
		declared, declares := graph.Object(usage, rdf.SysML+pDeclaredName)
		name, named := graph.Object(usage, rdf.SysML+pName)
		if declares && declared.Value == "subj" {
			continue
		}
		if declares || !named || name.Value != "subj" {
			t.Errorf("%s name = %v (present %t), declaredName = %v (present %t); want the derived subj", usage.Value, name, named, declared, declares)
		}
	}
	for _, subject := range graph.Subjects() {
		if m, _ := graph.Object(subject, rdf.RDFType); m.Value != rdf.SysML+"SatisfyRequirementUsage" {
			continue
		}
		if _, ok := graph.Object(subject, rdf.SysML+pDeclaredName); !ok {
			t.Fatalf("%s declares no name", subject.Value)
		}
	}
}

// A `do` statement declares no name (its action specializes TransitionAction::effect), yet
// a chain addresses it as `effect`, the role the symbol table resolves; the reader keeps that.
func TestTransitionEffectAddressedByAChainRoundTrips(t *testing.T) {
	graph := parseFixture(t, `package P {
    attribute def Pub;
    port def Pt { out item p : Pub; }
    part def Server {
        port port1 : Pt;
        state serverBehavior {
            entry; then waiting;
            state waiting; state done;
            transition delivering first waiting accept Pub do send new Pub() via port1 then done;
        }
    }
    part s : Server {
        occurrence src;
        event serverBehavior.delivering.effect[1] :>> src;
    }
}
`)
	structural := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.OpenSysML+xSourceText {
			structural.AddTriple(triple)
		}
	}
	back, err := ToSysML(structural)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(back), "serverBehavior.delivering.effect") {
		t.Fatalf("notation lost the chain to the effect:\n%s", back)
	}
}
