package modelsync

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const model = `package Vehicles {
	part def Wheel { attribute radius : ScalarValues::Real; }
	part def Car { part wheels : Wheel[4]; }
}
package Other { part def Boat; }
`

func graphOf(t *testing.T, notation string) *rdf.Graph {
	t.Helper()
	graph, err := convert.SysMLToRDF("session.sysml", []byte(notation))
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestRootedCutsOneElementAndWhatItOwns(t *testing.T) {
	graph := graphOf(t, model)
	cut, err := Rooted(graph, "Vehicles")
	if err != nil {
		t.Fatal(err)
	}
	roots := Roots(cut)
	if len(roots) != 1 {
		t.Fatalf("roots of the cut: %v, want just Vehicles", roots)
	}
	for _, subject := range cut.Subjects() {
		if name, _ := cut.Lexical(subject, rdf.SysML+"qualifiedName"); strings.HasPrefix(name, "Other") {
			t.Errorf("%s is outside Vehicles but in the cut", name)
		}
	}
	found := false
	for _, subject := range cut.Subjects() {
		if name, _ := cut.Lexical(subject, rdf.SysML+"qualifiedName"); name == "Vehicles::Car::wheels" {
			found = true
		}
	}
	if !found {
		t.Error("Vehicles::Car::wheels is owned by Vehicles but missing from the cut")
	}
	if _, err := Rooted(graph, "Nowhere"); err == nil {
		t.Error("an unknown name was cut")
	}
}

func TestRootedCutReadsBackAsNotation(t *testing.T) {
	graph := graphOf(t, model)
	for _, derived := range []bool{true, false} {
		cut, err := Rooted(graph, "Vehicles")
		if err != nil {
			t.Fatal(err)
		}
		if !derived {
			cut = WithoutDerived(cut)
		}
		data, err := export.WriteAPIJSON(cut)
		if err != nil {
			t.Fatalf("derived=%v: write: %v", derived, err)
		}
		back, err := export.ReadAPIJSON(data)
		if err != nil {
			t.Fatalf("derived=%v: read: %v", derived, err)
		}
		notation, err := Notation(Scoped(back, reposync.Scope{ProjectID: "p1", Branch: "main"}), nil)
		if err != nil {
			t.Fatalf("derived=%v: notation: %v", derived, err)
		}
		text := string(notation)
		for _, want := range []string{"package Vehicles", "part def Wheel", "part wheels : Wheel[4]", "projectId = \"p1\"", "branch = \"main\""} {
			if !strings.Contains(text, want) {
				t.Errorf("derived=%v: notation lacks %q:\n%s", derived, want, text)
			}
		}
		if strings.Contains(text, "Boat") {
			t.Errorf("derived=%v: notation holds Boat, outside Vehicles:\n%s", derived, text)
		}
	}
}

const chained = `package Chains {
	part def A { part b : B; }
	part def B { attribute x : ScalarValues::Real; }
	part a : A { attribute y = a.b.x; }
	variation part def Choice { variant part one : A; }
}
`

func TestWithoutDerivedDropsDerivedPropertiesTheReaderRecomputes(t *testing.T) {
	full := graphOf(t, chained)
	cut := WithoutDerived(full)
	for _, triple := range full.Triples() {
		if cut.Has(triple) {
			continue
		}
		name := rdf.LocalName(triple.Predicate.Value)
		if readerDerived[name] || !derived(full, triple) {
			t.Errorf("%s was dropped from %s", name, triple.Subject)
		}
	}
	for _, name := range []string{"declaredName", "qualifiedName", "owner"} {
		found := false
		for _, triple := range cut.Triples() {
			if triple.Predicate.Value == rdf.SysML+name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no %s is left", name)
		}
	}
}

// TestWithoutDerivedReadsBackEveryExample pins readerDerived against the
// notation reader: every example publishes without derived properties to the
// notation it publishes with them.
func TestWithoutDerivedReadsBackEveryExample(t *testing.T) {
	files, err := filepath.Glob("../../../../examples/*.sysml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	roundTrip := func(graph *rdf.Graph) ([]byte, error) {
		data, err := export.WriteAPIJSON(graph)
		if err != nil {
			return nil, err
		}
		back, err := export.ReadAPIJSON(data)
		if err != nil {
			return nil, err
		}
		return Notation(back, nil)
	}
	dropped := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		graph, err := convert.SysMLToRDF(file, data)
		if err != nil {
			continue
		}
		want, err := roundTrip(graph)
		if err != nil {
			continue
		}
		cut := WithoutDerived(graph)
		dropped += graph.Len() - cut.Len()
		got, err := roundTrip(cut)
		if err != nil {
			t.Errorf("%s: without derived properties: %v", filepath.Base(file), err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: the notation differs without derived properties", filepath.Base(file))
		}
	}
	if dropped == 0 {
		t.Error("no example has a derived property to leave out")
	}
}
