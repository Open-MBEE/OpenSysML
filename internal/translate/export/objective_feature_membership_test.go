package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// An objective is the RequirementUsage its ObjectiveMembership — a FeatureMembership (SysML 8.3.19) —
// owns: composite, undirected, no parameter end; a `require constraint` body varies (isVariable, mayTimeVary).
func TestObjectiveIsAnUndirectedCompositeAndTheRequireBodyVaries(t *testing.T) {
	graph := normativeGraph(t, `package O {
		requirement def Light { attribute mass : ScalarValues::Real; require constraint { mass < 10.0 } }
		requirement massRequirement : Light;
		verification def Weigh {
			objective { verify massRequirement; }
		}
	}`)
	var objective rdf.Term
	for _, e := range graph.Subjects() {
		if meta(graph, e) == "RequirementUsage" {
			for _, m := range graph.Objects(e, rdf.SysML+pOwningMembership) {
				if meta(graph, m) == "ObjectiveMembership" {
					objective = e
					if param := objects(graph, m.Value, pOwnedMemberParameter); len(param) != 0 {
						t.Errorf("ObjectiveMembership ownedMemberParameter %v, want none", param)
					}
					if end := objects(graph, m.Value, "ownedObjectiveRequirement"); len(end) != 1 || end[0].Value != e.Value {
						t.Errorf("ObjectiveMembership ownedObjectiveRequirement %v, want the objective", end)
					}
				}
			}
		}
	}
	if objective.Value == "" {
		t.Fatal("no RequirementUsage under an ObjectiveMembership")
	}
	if d := objects(graph, objective.Value, pDirection); len(d) != 0 {
		t.Errorf("objective direction %v, want none", d)
	}
	if !graph.BoolValue(objective, rdf.SysML+"isComposite") {
		t.Errorf("objective isComposite false, want true")
	}
	var constraints int
	for _, e := range graph.Subjects() {
		if meta(graph, e) != "ConstraintUsage" {
			continue
		}
		for _, m := range graph.Objects(e, rdf.SysML+pOwningMembership) {
			if meta(graph, m) != "RequirementConstraintMembership" {
				continue
			}
			constraints++
			if !graph.BoolValue(e, rdf.SysML+"isVariable") || !graph.BoolValue(e, rdf.SysML+"mayTimeVary") {
				t.Errorf("require body constraint isVariable/mayTimeVary false, want true (a feature of an Occurrence)")
			}
		}
	}
	if constraints != 1 {
		t.Fatalf("require body constraints = %d, want 1", constraints)
	}
	// The structural graph reads back to the objective as written; a graph stating a direction on the objective
	// reads it back explicitly (`in composite`, a directed usage being referential by default) — no `in` default.
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
	if !strings.Contains(string(back), "objective {") || strings.Contains(string(back), "in objective") {
		t.Errorf("notation of the undirected objective:\n%s", back)
	}
	structural.Add(objective, rdf.IRI(rdf.SysML+pDirection), rdf.String("in"))
	back, err = ToSysML(structural)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(back), "in composite objective {") {
		t.Errorf("notation lost the objective's stated direction:\n%s", back)
	}
}
