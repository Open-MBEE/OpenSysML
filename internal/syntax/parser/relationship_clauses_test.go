package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// relationshipDiags parses src as the file kind name selects and returns the
// parser's diagnostic messages.
func relationshipDiags(t *testing.T, name, src string) []string {
	t.Helper()
	p := New(source.New(name, []byte(src)))
	p.ParseFile()
	msgs := make([]string, 0, len(p.Diagnostics))
	for _, d := range p.Diagnostics {
		msgs = append(msgs, d.Message)
	}
	return msgs
}

func wantRelationshipError(t *testing.T, name, src, want string) {
	t.Helper()
	msgs := relationshipDiags(t, name, src)
	if len(msgs) == 0 {
		t.Fatalf("%s: %q: expected a diagnostic containing %q, got none", name, src, want)
	}
	for _, m := range msgs {
		if strings.Contains(m, want) {
			return
		}
	}
	t.Errorf("%s: %q: expected a diagnostic containing %q, got %v", name, src, want, msgs)
}

func wantRelationshipClean(t *testing.T, name, src string) {
	t.Helper()
	if msgs := relationshipDiags(t, name, src); len(msgs) != 0 {
		t.Errorf("%s: %q: expected no diagnostics, got %v", name, src, msgs)
	}
}

// A classifier declaration admits one specialization list: a second `:>` or
// `specializes` clause is an error (SysML.xtext DefinitionDeclaration,
// KerML.xtext ClassifierDeclaration). Issue #615.
func TestClassifierSecondSpecializationList(t *testing.T) {
	want := "a classifier declares one specialization list"
	for _, tc := range []struct {
		name, src string
	}{
		{"a.sysml", "part def X :> A :> B;"},
		{"a.sysml", "part def X specializes A specializes B;"},
		{"a.sysml", "part def X :> A specializes B;"},
		{"a.kerml", "class X :> A :> B;"},
		{"a.kerml", "type X :> A :> B;"},
		{"a.kerml", "class X specializes A specializes B;"},
	} {
		wantRelationshipError(t, tc.name, tc.src, want)
	}
}

// The specialization list precedes the type-relationship clauses
// (KerML.xtext:460 ClassifierDeclaration).
func TestClassifierSpecializationAfterTypeRelationship(t *testing.T) {
	wantRelationshipError(t, "a.kerml", "class X disjoint from A :> B;", "must precede")
	wantRelationshipError(t, "a.kerml", "class X unions A specializes B;", "must precede")
	wantRelationshipError(t, "a.kerml", "type X disjoint from A conjugates B;", "must precede")
}

// Clauses a FeatureDeclaration states relate features only; a classifier
// declaration admits none of them.
func TestClassifierFeatureClausesRejected(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"a.sysml", "part def X : A;", "`:` relates features"},
		{"a.sysml", "part def X subsets A;", "`subsets` relates features"},
		{"a.sysml", "part def X redefines A;", "`:>>` relates features"},
		{"a.sysml", "part def X :>> A;", "`:>>` relates features"},
		{"a.sysml", "part def X ::> A;", "`::>` relates features"},
		{"a.sysml", "part def X => A;", "`=>` relates features"},
		{"a.kerml", "class X : A;", "a classifier declaration admits no `:` clause"},
		{"a.kerml", "class X :>> a;", "`:>>` relates features: a classifier declaration admits no `:>>` clause"},
		{"a.kerml", "class X chains a.b;", "`chains` relates features"},
		{"a.kerml", "class X inverse of a;", "`inverse of` relates features"},
		{"a.kerml", "class X featured by a;", "`featured by` relates features"},
	} {
		wantRelationshipError(t, tc.name, tc.src, tc.want)
	}
}

// A KerML classifier's OwnedMultiplicity sits ahead of its specialization part
// (KerML.xtext:460), so a `[` after the specialization list is misplaced.
func TestClassifierMultiplicityAfterSpecialization(t *testing.T) {
	wantRelationshipError(t, "a.kerml", "class X :> A [2];", "multiplicity precedes the specialization list")
}

// Specializations precede a feature declaration's relationship clauses
// (KerML.xtext FeatureDeclaration: FeatureSpecializationPart?
// FeatureRelationshipPart*).
func TestFeatureSpecializationAfterRelationshipClause(t *testing.T) {
	wantRelationshipError(t, "a.kerml", "feature x disjoint from y : A;", "is a specialization")
	wantRelationshipError(t, "a.kerml", "feature x chains a.b :> y;", "is a specialization")
	wantRelationshipError(t, "a.kerml", "feature x inverse of a ::> b;", "is a specialization")
}

// A MultiplicityPart admits `ordered` and `nonunique` once each, in either
// order (KerML.xtext:685).
func TestFeatureMultiplicityDuplicateModifier(t *testing.T) {
	wantRelationshipError(t, "a.kerml", "feature x : A ordered nonunique ordered;", "duplicate `ordered`")
	wantRelationshipError(t, "a.kerml", "feature x : A nonunique nonunique;", "duplicate `nonunique`")
	wantRelationshipError(t, "a.sysml", "part x : A [2] ordered ordered;", "duplicate `ordered`")
}

// Declaration tails that the grammars admit stay diagnostic-free.
func TestRelationshipClauseShapesAccepted(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		// One specialization list states many targets.
		{"a.sysml", "part def X :> A, B;"},
		// Specialization before the type-relationship clauses, and several of
		// those in a row.
		{"a.kerml", "class C specializes A disjoint from B;"},
		{"a.kerml", "class X disjoint from A, B unions C, D intersects D, E differences E, A;"},
		// A feature repeats specializations freely.
		{"a.sysml", "part x :> a :> b;"},
		{"a.sysml", "part x : A : B;"},
		{"a.sysml", "part x : A [2] :> b;"},
		{"a.kerml", "feature x [1] : A;"},
		// A classifier's multiplicity precedes its specialization list.
		{"a.kerml", "class X [2] :> A;"},
		// `ordered`/`nonunique` once each, in either order.
		{"a.kerml", "feature x : A ordered nonunique;"},
		{"a.kerml", "feature x : A nonunique ordered;"},
	} {
		wantRelationshipClean(t, tc.name, tc.src)
	}
}
