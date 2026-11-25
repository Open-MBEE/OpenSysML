package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// An ownedAnnotatingRelationship the candidate derives and the graph omits is not a
// disagreement (a publish without derived properties states none); one naming another
// Annotation is.
func TestOmittedAnnotatingRelationshipIsNotADisagreement(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    part def W;\n\n    comment about W /* note */\n    @M about W;\n    metadata def M;\n}\n")
	stripped := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	dropped := 0
	stripped.RewriteTriples(func(t *rdf.Triple) bool {
		if rdf.LocalName(t.Predicate.Value) != pOwnedAnnotatingRelationship {
			return true
		}
		dropped++
		return false
	})
	if dropped != 2 {
		t.Fatalf("dropped %d ownedAnnotatingRelationship statements, want 2 (the comment's and the metadata usage's)", dropped)
	}
	if got := disagreeingTriples(stripped, full); len(got) != 0 {
		t.Errorf("%d omitted statements counted as disagreements: %v", len(got), got)
	}

	moved := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	changed := 0
	moved.RewriteTriples(func(t *rdf.Triple) bool {
		if t.Predicate.Value == rdf.SysML+pOwnedAnnotatingRelationship && t.Subject.Value == rdf.Element+"P___401" {
			t.Object = rdf.IRI(rdf.Element + "P___402_an0")
			changed++
		}
		return true
	})
	if changed != 1 {
		t.Fatalf("moved %d statements, want 1", changed)
	}
	if len(disagreeingTriples(moved, full)) == 0 {
		t.Error("an ownedAnnotatingRelationship naming another element's Annotation is not a disagreement")
	}
}
