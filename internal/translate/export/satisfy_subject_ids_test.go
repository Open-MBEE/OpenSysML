package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A `by` clause and its target share one span, so a reader over another parse must still tell
// the subject parameter (and its SubjectMembership) from its value (and its FeatureValue).
func TestSatisfySubjectAndValueIDsAreDistinctAcrossParses(t *testing.T) {
	const text = `package P {
	part def Sys { part sub : Sys; }
	requirement def R { subject s : Sys; }
	part sys : Sys {
		satisfy requirement r : R by sys;
		satisfy requirement r3 : R by sys.sub;
	}
}
`
	first := source.New("satisfy-ids.sysml", []byte(text))
	reader := NewElementIDReader([]ElementIDDocument{{File: first, Root: parser.New(first).ParseFile()}})
	other := parser.New(source.New("satisfy-ids.sysml", []byte(text))).ParseFile()
	var clauses []*ast.Relationship
	ast.Inspect(other, func(n ast.Node) bool {
		if u, ok := n.(*ast.Usage); ok && u.Kind == ast.UsageSatisfy {
			for _, rel := range u.Relationships {
				if rel != nil && rel.Kind == ast.RelSubject {
					clauses = append(clauses, rel)
				}
			}
		}
		return true
	})
	if len(clauses) != 2 {
		t.Fatalf("%d by clauses, want 2", len(clauses))
	}
	for i, want := range []string{"P__sys__r_subject", "P__sys__r3_subject"} {
		rel := clauses[i]
		if rel.Span() != rel.Target.Span() {
			t.Fatalf("%s: the clause and its target no longer share a span; this test guards that case", want)
		}
		if got, err := reader.Read(first.Name(), rel, ""); err != nil || got != want {
			t.Errorf("subject %d = %q (%v), want %s", i, got, err, want)
		}
		if got, err := reader.Read(first.Name(), rel.Target, ""); err != nil || got != want+"_pvalue" {
			t.Errorf("value %d = %q (%v), want %s_pvalue", i, got, err, want)
		}
		if got, err := reader.ReadOwningMembership(first.Name(), rel, ""); err != nil || got != want+"_om" {
			t.Errorf("subject membership %d = %q (%v), want %s_om", i, got, err, want)
		}
		if got, err := reader.ReadOwningMembership(first.Name(), rel.Target, ""); err != nil || got != want+"_pvalue_om" {
			t.Errorf("value membership %d = %q (%v), want %s_pvalue_om", i, got, err, want)
		}
	}
}
