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

// A graph stating an about target by its name alone (the collapsed form of a graph
// that never resolved it) still agrees with an Annotation naming that element by IRI.
func TestAnnotationMayAnnotateAnElementStatedByName(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    part def A;\n    comment about A /* note */\n    metadata def M;\n    metadata m : M about A;\n}\n")
	byName := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	rewritten := 0
	byName.RewriteTriples(func(tr *rdf.Triple) bool {
		if !strings.HasSuffix(tr.Predicate.Value, pAnnotatedElement) || strings.Contains(tr.Subject.Value, "_an") {
			return true
		}
		switch {
		case tr.Object.Value == rdf.Element+"P__A":
			*tr = rdf.Triple{Subject: tr.Subject, Predicate: tr.Predicate, Object: rdf.String("A")}
			rewritten++
		case strings.Contains(tr.Object.Value, `{"@id":"P__A"}`):
			tr.Object.Value = strings.Replace(tr.Object.Value, `{"@id":"P__A"}`, `"A"`, 1)
			rewritten++
		}
		return true
	})
	if rewritten != 2 {
		// One statement per owner: a single target carries no JSON mirror.
		t.Fatalf("rewrote %d collapsed statements of A to its name, want 2", rewritten)
	}
	out, err := ToSysML(byName)
	if err != nil {
		t.Fatalf("a graph stating the about target by name reads: %v", err)
	}
	for _, want := range []string{"comment about A", "metadata m : M about A;"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the graph does not read back with %q:\n%s", want, out)
		}
	}
}

// An Annotation matched to the owner's literal through a namesake's declared name
// does not respell the clause: the owner's `about A` stays `A`, not the namesake.
func TestAnnotationMatchedByNameKeepsTheOwnersSpelling(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    comment about A /* note */\n}\npackage Q {\n    part def A;\n}\n")
	namesake := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	rewritten := 0
	namesake.RewriteTriples(func(tr *rdf.Triple) bool {
		// Without source text the clause is printed from the graph, as another tool's graph is.
		if tr.Predicate.Value == rdf.OpenSysML+xSourceText || tr.Predicate.Value == rdf.OpenSysML+xSourceTail {
			return false
		}
		if strings.HasSuffix(tr.Subject.Value, "_an0") && tr.Predicate.Value == rdf.SysML+pAnnotatedElement && tr.Object.Value == "A" {
			*tr = rdf.Triple{Subject: tr.Subject, Predicate: tr.Predicate, Object: rdf.IRI(rdf.Element + "Q__A")}
			rewritten++
		}
		return true
	})
	if rewritten != 1 {
		t.Fatalf("rewrote %d Annotation targets to the namesake, want 1", rewritten)
	}
	out, err := ToSysML(namesake)
	if err != nil {
		t.Fatalf("the graph reads: %v", err)
	}
	if !strings.Contains(string(out), "comment about A ") || strings.Contains(string(out), "Q::A") {
		t.Errorf("the owner's `about A` should read back as written, not as the namesake:\n%s", out)
	}
}

// Two written names matched to namesakes each keep their own: a target whose short name is
// another target's name does not take that name's place.
func TestAnnotationsMatchedByNameKeepDistinctSpellings(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    comment about S, B /* note */\n}\npackage Q {\n    part def <S> B;\n}\npackage R {\n    part def S;\n}\n")
	namesakes := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	rewritten := 0
	namesakes.RewriteTriples(func(tr *rdf.Triple) bool {
		if tr.Predicate.Value == rdf.OpenSysML+xSourceText || tr.Predicate.Value == rdf.OpenSysML+xSourceTail {
			return false
		}
		if tr.Predicate.Value != rdf.SysML+pAnnotatedElement {
			return true
		}
		switch {
		case strings.HasSuffix(tr.Subject.Value, "_an0") && tr.Object.Value == "S":
			*tr = rdf.Triple{Subject: tr.Subject, Predicate: tr.Predicate, Object: rdf.IRI(rdf.Element + "R__S")}
			rewritten++
		case strings.HasSuffix(tr.Subject.Value, "_an1") && tr.Object.Value == "B":
			*tr = rdf.Triple{Subject: tr.Subject, Predicate: tr.Predicate, Object: rdf.IRI(rdf.Element + "Q__B")}
			rewritten++
		}
		return true
	})
	if rewritten != 2 {
		t.Fatalf("rewrote %d Annotation targets to the namesakes, want 2", rewritten)
	}
	out, err := ToSysML(namesakes)
	if err != nil {
		t.Fatalf("the graph reads: %v", err)
	}
	if !strings.Contains(string(out), "comment about S, B ") {
		t.Errorf("the owner's `about S, B` should read back as written:\n%s", out)
	}
}

// Two Annotations of the one namesake both keep the one written name: `about A, A`.
func TestRepeatedAnnotationsMatchedByNameKeepTheSpelling(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    comment about A, A /* note */\n}\npackage Q {\n    part def A;\n}\n")
	namesake := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	rewritten := 0
	namesake.RewriteTriples(func(tr *rdf.Triple) bool {
		if tr.Predicate.Value == rdf.OpenSysML+xSourceText || tr.Predicate.Value == rdf.OpenSysML+xSourceTail {
			return false
		}
		if strings.Contains(tr.Subject.Value, "_an") && tr.Predicate.Value == rdf.SysML+pAnnotatedElement && tr.Object.Value == "A" {
			*tr = rdf.Triple{Subject: tr.Subject, Predicate: tr.Predicate, Object: rdf.IRI(rdf.Element + "Q__A")}
			rewritten++
		}
		return true
	})
	if rewritten != 2 {
		t.Fatalf("rewrote %d Annotation targets to the namesake, want 2", rewritten)
	}
	out, err := ToSysML(namesake)
	if err != nil {
		t.Fatalf("the graph reads: %v", err)
	}
	if !strings.Contains(string(out), "comment about A, A ") {
		t.Errorf("the owner's `about A, A` should read back as written:\n%s", out)
	}
}

// Two written names of one namesake (`A` and its short name `S`) each stay as written.
func TestAnnotationsOfOneNamesakeKeepBothWrittenNames(t *testing.T) {
	full := graphOfNotation(t, "package P {\n    comment about A, S /* note */\n}\npackage Q {\n    part def <S> A;\n}\n")
	namesake := rdf.NewGraphOf(append([]rdf.Triple(nil), full.Triples()...), full.Prefixes)
	rewritten := 0
	namesake.RewriteTriples(func(tr *rdf.Triple) bool {
		if tr.Predicate.Value == rdf.OpenSysML+xSourceText || tr.Predicate.Value == rdf.OpenSysML+xSourceTail {
			return false
		}
		if strings.Contains(tr.Subject.Value, "_an") && tr.Predicate.Value == rdf.SysML+pAnnotatedElement && !tr.Object.IsIRI() {
			*tr = rdf.Triple{Subject: tr.Subject, Predicate: tr.Predicate, Object: rdf.IRI(rdf.Element + "Q__A")}
			rewritten++
		}
		return true
	})
	if rewritten != 2 {
		t.Fatalf("rewrote %d Annotation targets to the namesake, want 2", rewritten)
	}
	out, err := ToSysML(namesake)
	if err != nil {
		t.Fatalf("the graph reads: %v", err)
	}
	if !strings.Contains(string(out), "comment about A, S ") {
		t.Errorf("the owner's `about A, S` should read back as written:\n%s", out)
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
