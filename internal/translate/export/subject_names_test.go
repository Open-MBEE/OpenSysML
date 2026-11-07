package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A nameless subject (`subject = x;`, a satisfaction's `by x`) redefines the definition's subject
// and takes its name (SysML v2 7.19.3, KerML 1.1 §8.3.3.3): name and memberName state it, declaredName not.
func TestNamelessSubjectIsNamedByTheDefinitionsSubject(t *testing.T) {
	graph := normativeGraph(t, `package P {
	part def Sys;
	part sys : Sys;
	requirement def R { subject s : Sys; }
	requirement r : R { subject = sys; }
	satisfy requirement r2 : R by sys;
}
`)
	for _, owner := range []string{"P__r", "P__r2"} {
		var subjects []rdf.Term
		for _, membership := range objects(graph, "urn:sysmlv2:element:"+owner, pOwnedMembership) {
			if meta(graph, membership) != mSubjectMembership {
				continue
			}
			if got := graph.Objects(membership, rdf.SysML+pMemberName); len(got) != 1 || got[0].Value != "s" {
				t.Errorf("%s: SubjectMembership memberName = %v, want \"s\"", owner, got)
			}
			subjects = append(subjects, graph.Objects(membership, rdf.SysML+pMemberElement)...)
		}
		if len(subjects) != 1 {
			t.Fatalf("%s: %d subject parameters, want 1", owner, len(subjects))
		}
		if got := graph.Objects(subjects[0], rdf.SysML+pName); len(got) != 1 || got[0].Value != "s" {
			t.Errorf("%s: subject name = %v, want \"s\"", owner, got)
		}
		if got := graph.Objects(subjects[0], rdf.SysML+pDeclaredName); len(got) != 0 {
			t.Errorf("%s: subject declaredName = %v, want none", owner, got)
		}
	}
}
