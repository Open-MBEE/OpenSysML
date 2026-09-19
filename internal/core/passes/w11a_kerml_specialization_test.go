package passes

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// w11aSysMLTypeMessages are the type-tier messages of src analyzed as SysML.
func w11aSysMLTypeMessages(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, d := range typeDiags(t, src) {
		out = append(out, d.Message)
	}
	sort.Strings(out)
	return out
}

// `:>` on a classifier is a subclassification like `specializes`, so the family
// rules see both spellings (KerML 8.3.3.1; pilot validateClassSpecialization,
// validateDataTypeSpecialization).
func TestW11ASpecializationFamiliesSubclassificationSpelling(t *testing.T) {
	src := `package Test {
	datatype D; class C; struct S; assoc AS; assoc struct ASS; interaction IN; behavior B;
	class C1 :> AS;
	class C2 :> ASS;
	class C3 :> S, D;
	struct S1 :> D;
	metaclass MC :> D;
	assoc struct AS1 :> D;
	interaction IN1 :> D;
	behavior B1 :> AS;
	function F1 :> D;
	predicate P1 :> D;
	datatype D1 :> C;
	datatype D2 :> ASS;
	datatype D3 :> IN;
	datatype D4 :> B;
	class C4 :> C { class Inner :> D; }
	abstract class C5 :> D;
	class C6 specializes D, AS;
}`
	w11aWantMessages(t, src, false,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeClassOrAssoc, msgW11ASpecializeClassOrAssoc,
		msgW11ASpecializeClassOrAssoc, msgW11ASpecializeClassOrAssoc)
}

// A class that is also an association may specialize an association but still
// not a data type; a structure that specializes an interaction breaks the class
// rule and the structure rule alike, one message each.
func TestW11ASpecializationFamiliesAssociationClasses(t *testing.T) {
	src := `package Test {
	datatype D; struct S; assoc AS; interaction IN;
	assoc struct AS1 :> AS;
	interaction IN1 :> AS;
	assoc struct AS2 :> D;
	struct S1 :> IN;
	assoc AS3 :> S;
}`
	w11aWantMessages(t, src, false,
		msgW11ASpecializeDataTypeOrAssoc, msgW11ASpecializeDataTypeOrAssoc,
		msgW11ASpecializeBehavior)
}

// A plain classifier or type, a feature subsetting another, and a data type
// specializing a data type are outside the family rules.
func TestW11ASpecializationFamiliesSilentShapes(t *testing.T) {
	src := `package Test {
	datatype D; class C;
	classifier CL :> D;
	type T :> D;
	datatype D1 :> D;
	class C1 :> C;
	feature f : D;
	feature g :> f;
	class C2 { feature h :> f; }
}`
	name := "<t>.kerml"
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument(name, root)
	for _, d := range Analyze(name, root, nil, idx) {
		if d.Code == "specialization-kind" {
			t.Errorf("unexpected %q", d.Message)
		}
	}
}

// The SysML definitions map onto the KerML families and are reported in the
// pilot's SysML wording (pilot validateClassSpecialization,
// validateDataTypeSpecialization on a SysML document).
func TestW11ASpecializationFamiliesSysMLDefinitions(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{"attribute def A; item def I :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; part def P :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; occurrence def O :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; individual def X :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"enum def E; part def P :> E;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; port def PT :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; metadata def M :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; view def V :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; action def AD :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; state def SD :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; calc def CA :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; constraint def CO :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; bool def BD :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"part def PD; bool def BD :> PD;", []string{msgW11ASpecializeStructure}},
		{"attribute def A; requirement def R :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; use case def UC :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; connection def CD :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; interface def IF :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; allocation def AL :> A;", []string{msgW11ASpecializeAttributeDef}},
		{"connection def CD; part def P :> CD;", []string{msgW11ASpecializeAttributeDef}},
		{"interface def IF; part def P :> IF;", []string{msgW11ASpecializeAttributeDef}},
		{"flow def FD; item def I :> FD;", []string{msgW11ASpecializeAttributeDef, msgW11ASpecializeBehavior}},
		{"item def I; attribute def A :> I;", []string{msgW11ASpecializeItemDef}},
		{"connection def CD; attribute def A :> CD;", []string{msgW11ASpecializeItemDef}},
		{"action def AD; attribute def A :> AD;", []string{msgW11ASpecializeItemDef}},
		{"item def I; enum def E :> I;", []string{msgW11ASpecializeItemDef}},
		{"attribute def A; part def P1 :> A; part def P2 :> P1;", []string{msgW11ASpecializeAttributeDef}},
		{"attribute def A; item def I specializes A;", []string{msgW11ASpecializeAttributeDef}},
		{"connection def CD; attribute def A; part def P :> CD, A;", []string{msgW11ASpecializeAttributeDef, msgW11ASpecializeAttributeDef}},
		{"part def PD; action def AD :> PD;", []string{msgW11ASpecializeStructure}},
		{"action def AD; part def PD :> AD;", []string{msgW11ASpecializeBehavior}},
		{"part def PD; connection def CD :> PD;", nil},
		{"part def PD; flow def FD :> PD;", []string{msgW11ASpecializeStructure}},
		{"connection def CD; flow def FD :> CD;", []string{msgW11ASpecializeStructure}},
		{"action def AD; flow def FD :> AD;", nil},
		{"interface def IF; connection def CD :> IF;", nil},
		{"attribute def A; enum def E :> A;", nil},
		{"part def PD; individual def X :> PD;", nil},
		{"attribute def A; part def P { attribute a : A; attribute b :> a; }", nil},
	}
	for _, tt := range cases {
		got := w11aSysMLTypeMessages(t, tt.src)
		sort.Strings(tt.want)
		if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
			t.Errorf("%s: got %v, want %v", tt.src, got, tt.want)
		}
	}
}

// The same rule reports a definition's library supertype of the wrong family.
func TestW11ASpecializationFamiliesLibrarySupertypes(t *testing.T) {
	src := `package Test {
	part def P1 :> ScalarValues::Integer;
	attribute def A1 :> Occurrences::Occurrence;
	attribute def A2 :> Links::Link;
	part def P2 :> Base::Anything;
	attribute def A3 :> Base::Anything;
	part def P3 :> Objects::Object;
	connection def CD :> Links::Link;
}`
	got := w11aSysMLLibraryTypeMessages(t, src)
	want := []string{msgW11ASpecializeAttributeDef, msgW11ASpecializeItemDef, msgW11ASpecializeItemDef}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// w11aSysMLLibraryTypeMessages are the specialization-kind messages of src
// analyzed as SysML against the standard library.
func w11aSysMLLibraryTypeMessages(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, d := range w9cLibraryDiags(t, src, false) {
		if d.Code == "specialization-kind" {
			out = append(out, d.Message)
		}
	}
	sort.Strings(out)
	return out
}
