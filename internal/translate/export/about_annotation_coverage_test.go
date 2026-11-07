package export

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A comment whose collapsed annotatedElement names two elements but which owns one
// Annotation is a conflicting graph: writing it would mint the Annotation the elements
// deny. A graph stating no Annotation elements at all (from before them) still reads.
func TestAboutAnnotationsMustCoverEveryAnnotatedElement(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    part def A;\n    part def B;\n    comment about A, B /* note */\n    metadata def M;\n    metadata m : M about A, B;\n}\n")
	for _, missing := range []string{"P___402_an1", "P__m_an1"} {
		t.Run(missing, func(t *testing.T) {
			partial := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
			dropped := 0
			partial.RewriteTriples(func(tr *rdf.Triple) bool {
				// The element's statements, and every reference to it, the JSON mirror included.
				if tr.Subject.Value == rdf.Element+missing || strings.Contains(tr.Object.Value, missing) {
					dropped++
					return false
				}
				return true
			})
			if dropped == 0 {
				t.Fatalf("the graph states nothing about %s", missing)
			}
			out, err := ToSysML(partial)
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v; notation:\n%s", err, out)
			}
			if !strings.Contains(err.Error(), rdf.Element+"P__B") {
				t.Errorf("error %q should name the uncovered <%sP__B>", err, rdf.Element)
			}
		})
	}

	legacy := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	legacy.RewriteTriples(func(tr *rdf.Triple) bool {
		return !strings.Contains(tr.Subject.Value, "_an") && !strings.Contains(tr.Object.Value, "_an")
	})
	out, err := ToSysML(legacy)
	if err != nil {
		t.Fatalf("a graph stating no Annotation elements reads: %v", err)
	}
	for _, want := range []string{"comment about A, B", "metadata m : M about A, B;"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the legacy graph does not read back with %q:\n%s", want, out)
		}
	}
}

func TestAnnotationMustAnnotateAStatedElement(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    part def A;\n    part def B;\n    comment about A, B /* note */\n    metadata def M;\n    metadata m : M about A, B;\n}\n")
	for _, owner := range []string{"P___402", "P__m"} {
		t.Run(owner, func(t *testing.T) {
			conflicting := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
			dropped := 0
			conflicting.RewriteTriples(func(tr *rdf.Triple) bool {
				// The owner's collapsed statement of B alone, its JSON mirror included; the _an1 Annotation of B stays.
				if tr.Subject.Value != rdf.Element+owner || !strings.HasSuffix(tr.Predicate.Value, pAnnotatedElement) {
					return true
				}
				if tr.Object.Value == rdf.Element+"P__B" {
					dropped++
					return false
				}
				if strings.Contains(tr.Object.Value, `{"@id":"P__B"}`) {
					tr.Object.Value = strings.Replace(tr.Object.Value, `,{"@id":"P__B"}`, "", 1)
					dropped++
				}
				return true
			})
			if dropped != 2 {
				t.Fatalf("rewrote %d collapsed statements of <%sP__B> by %s, want 2", dropped, rdf.Element, owner)
			}
			out, err := ToSysML(conflicting)
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v; notation:\n%s", err, out)
			}
			if !strings.Contains(err.Error(), rdf.Element+"P__B") {
				t.Errorf("error %q should name the denied <%sP__B>", err, rdf.Element)
			}
		})
	}
}

// `about x, x` owns two Annotations of the one element, as the notation adds one per
// name (KerML 8.2.3.2 AnnotatingElement: `about` names), each with its own id, and
// reads back as written.
func TestRepeatedAboutNamesOwnDistinctAnnotations(t *testing.T) {
	graph := graphOfNotation(t, "package P {\n    part x;\n    comment about x, x /* note */\n}\n")
	comment := rdf.IRI(rdf.Element + "P___401")
	owned := graph.Objects(comment, rdf.SysML+pOwnedAnnotatingRelationship)
	if len(owned) != 2 || owned[0].Value != rdf.Element+"P___401_an0" || owned[1].Value != rdf.Element+"P___401_an1" {
		t.Fatalf("the comment owns %v, want _an0 and _an1", owned)
	}
	for _, annotation := range owned {
		if got := graph.Objects(annotation, rdf.SysML+pAnnotatedElement); len(got) != 1 || got[0].Value != rdf.Element+"P__x" {
			t.Errorf("%s annotates %v, want x", annotation.Value, got)
		}
	}
	if got := graph.Objects(comment, rdf.SysML+pAnnotatedElement); len(got) != 1 {
		t.Errorf("the collapsed annotatedElement states %v, want x once", got)
	}
	out, err := ToSysML(graph)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "comment about x, x") {
		t.Errorf("the notation does not come back with both names:\n%s", out)
	}
}
