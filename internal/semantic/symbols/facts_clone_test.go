package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// pathsOf is every member path a fact's references hold, so a test can write
// through each and see whether another value moved.
func pathsOf(f LibraryFacts) [][]int32 {
	var out [][]int32
	add := func(refs ...ElementRef) {
		for _, r := range refs {
			out = append(out, r.Path)
		}
	}
	add(f.Supers...)
	add(f.Redefines...)
	add(f.About...)
	add(f.Ends...)
	add(f.Alias, f.References, f.BaseType)
	for _, rel := range f.Relationships {
		add(rel.Target)
	}
	for _, a := range f.Annotations {
		add(a.Type)
	}
	if f.Annotation != nil {
		add(f.Annotation.Type)
	}
	return out
}

func TestLibraryFactsCloneOwnsEveryPath(t *testing.T) {
	ref := func(fqn string) ElementRef { return ElementRef{FQN: fqn, Path: []int32{1, 2}, Doc: "a.sysml"} }
	facts := LibraryFacts{
		Supers:        []ElementRef{ref("S")},
		Redefines:     []ElementRef{ref("R")},
		About:         []ElementRef{ref("A")},
		Ends:          []ElementRef{ref("E")},
		Alias:         ref("L"),
		References:    ref("F"),
		BaseType:      ref("B"),
		Relationships: []RelationshipFacts{{Kind: ast.RelSubsets, Target: ref("T")}},
		Annotations:   []AnnotationFacts{{TypeFQN: "M", Type: ref("M")}},
		Annotation:    &AnnotationFacts{TypeFQN: "N", Type: ref("N")},
	}
	clone := facts.Clone()
	cloned := pathsOf(clone)
	if want := 10; len(cloned) != want {
		t.Fatalf("the clone holds %d references, want %d", len(cloned), want)
	}
	for _, p := range cloned {
		p[0] = 99
	}
	for i, p := range pathsOf(facts) {
		if p[0] != 1 {
			t.Fatalf("reference %d of the original reads path %v after the clone's changed", i, p)
		}
	}
}

func TestAnnotationFactsCloneOwnsItsValues(t *testing.T) {
	facts := AnnotationFacts{Values: []AnnotationValueFacts{{Feature: "f", Values: []FilterValue{{}, {}}}}}
	clone := facts.Clone()
	if &clone.Values[0] == &facts.Values[0] || &clone.Values[0].Values[0] == &facts.Values[0].Values[0] {
		t.Fatal("the clone shares a value sequence with the original")
	}
}

func TestGatheredRelationshipsCloneOwnsEveryPath(t *testing.T) {
	ref := func(fqn string) ElementRef { return ElementRef{FQN: fqn, Path: []int32{1}, Doc: "a.sysml"} }
	g := &GatheredRelationships{
		Derivations:   []GatheredEnds{{Sources: []ElementRef{ref("s")}, Targets: []ElementRef{ref("t")}}},
		Allocations:   []GatheredEnds{{Sources: []ElementRef{ref("s")}, Targets: []ElementRef{ref("t")}}},
		Conformances:  []GatheredEnds{{Sources: []ElementRef{ref("s")}, Targets: []ElementRef{ref("t")}}},
		Satisfactions: []GatheredSatisfaction{{Requirements: []ElementRef{ref("r")}, Satisfiers: []ElementRef{ref("p")}}},
	}
	clone := g.Clone()
	for _, ends := range [][]GatheredEnds{clone.Derivations, clone.Allocations, clone.Conformances} {
		ends[0].Sources[0].Path[0] = 99
		ends[0].Targets[0].Path[0] = 99
	}
	clone.Satisfactions[0].Requirements[0].Path[0] = 99
	clone.Satisfactions[0].Satisfiers[0].Path[0] = 99
	for _, ends := range [][]GatheredEnds{g.Derivations, g.Allocations, g.Conformances} {
		if ends[0].Sources[0].Path[0] != 1 || ends[0].Targets[0].Path[0] != 1 {
			t.Fatalf("the original's ends read %+v after the clone's changed", ends[0])
		}
	}
	if s := g.Satisfactions[0]; s.Requirements[0].Path[0] != 1 || s.Satisfiers[0].Path[0] != 1 {
		t.Fatalf("the original's satisfaction reads %+v after the clone's changed", s)
	}
}

// A usage the classification has no kind for is a feature by its declaration;
// a recorded one has no declaration and answers from its record.
func TestRecordedUnclassifiedUsageIsFeature(t *testing.T) {
	usage := &Symbol{Kind: SymbolUnknown, Facts: &LibraryFacts{Recorded: true, Node: NodeUsage}}
	if !usage.IsFeature() {
		t.Fatal("a recorded unclassified usage is not a feature")
	}
	def := &Symbol{Kind: SymbolUnknown, Facts: &LibraryFacts{Recorded: true, Node: NodeDefinition}}
	if def.IsFeature() {
		t.Fatal("a recorded unclassified definition is a feature")
	}
}
