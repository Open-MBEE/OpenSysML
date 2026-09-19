package passes

import (
	"testing"
)

// relationshipEndCase is one keyword-first relationship whose ends are judged:
// wants lists the type diagnostics expected, in order; none means clean.
type relationshipEndCase struct {
	name  string
	src   string
	wants []string
}

func checkRelationshipEndCases(t *testing.T, cases []relationshipEndCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := diagsIn(t, "a.kerml", "package P { "+c.src+" }", "type")
			if len(diags) != len(c.wants) {
				t.Fatalf("%s: expected %d type diagnostics %q, got %v", c.src, len(c.wants), c.wants, diags)
			}
			for i, want := range c.wants {
				if diags[i].Message != want {
					t.Errorf("%s: diagnostic %d: got %q, want %q", c.src, i, diags[i].Message, want)
				}
			}
		})
	}
}

// A Specialization relates two Types (KerML 1.0 §8.3.3.1): a package is not one,
// a feature is. The pinned validator rejects the package ends and accepts the rest.
func TestRelationshipMemberSubtypeEnds(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"package source", "package Q; class B; subtype Q :> B;",
			[]string{"subtype source must be a type, found package"}},
		{"package target", "package Q; class B; specialization subtype B :> Q;",
			[]string{"subtype target must be a type, found package"}},
		{"both ends packages", "package Q; package R; subtype Q :> R;",
			[]string{"subtype source must be a type, found package", "subtype target must be a type, found package"}},
		{"types", "class A; class B; specialization Gen subtype A specializes B;", nil},
		{"features are types", "feature f; class B; subtype f :> B; subtype B :> f;", nil},
	})
}

// A Subclassification relates two Classifiers (§8.3.3.2): neither a package nor
// a feature is one, while every classifier kind, a data type among them, is.
func TestRelationshipMemberSubclassifierEnds(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"package source", "package Q; class B; subclassifier Q :> B;",
			[]string{"subclassifier source must be a classifier, found package"}},
		{"package target", "package Q; class B; subclassifier B :> Q;",
			[]string{"subclassifier target must be a classifier, found package"}},
		{"feature source", "feature f; class B; subclassifier f :> B;",
			[]string{"subclassifier source must be a classifier, found attributeUsage"}},
		{"feature target", "feature f; class B; subclassifier B :> f;",
			[]string{"subclassifier target must be a classifier, found attributeUsage"}},
		{"classifiers", "class A; class B; specialization subclassifier A specializes B;", nil},
		{"data type", "datatype D; class B; subclassifier D :> B;", nil},
	})
}

// A FeatureTyping types a Feature by a Type (§8.3.4.4): the typed end must be a
// feature, the type may be any type, a feature included.
func TestRelationshipMemberTypingEnds(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"package source", "package Q; class B; typing Q : B;",
			[]string{"typing source must be a feature, found package"}},
		{"class source", "class C; class B; typing C typed by B;",
			[]string{"typing source must be a feature, found kermlType"}},
		{"package target", "package Q; feature f; specialization typing f : Q;",
			[]string{"typing target must be a type, found package"}},
		{"feature by type", "feature f; class B; typing f : B;", nil},
		{"feature by feature", "feature f; feature g; typing f typed by g;", nil},
	})
}

// A Subsetting and a Redefinition relate two Features (§8.3.4.5, §8.3.4.6):
// neither a package nor a class stands at either end.
func TestRelationshipMemberSubsetAndRedefinitionEnds(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"subset package source", "package Q; feature f; subset Q :> f;",
			[]string{"subset source must be a feature, found package"}},
		{"subset package target", "package Q; feature f; subset f subsets Q;",
			[]string{"subset target must be a feature, found package"}},
		{"subset class source", "class C; feature f; subset C :> f;",
			[]string{"subset source must be a feature, found kermlType"}},
		{"subset class target", "class C; feature f; specialization subset f :> C;",
			[]string{"subset target must be a feature, found kermlType"}},
		{"subset features", "feature f; feature g; subset f :> g;", nil},
		{"redefinition package source", "package Q; class A { feature f; } redefinition Q :>> A::f;",
			[]string{"redefinition source must be a feature, found package"}},
		{"redefinition package target", "package Q; class A { feature f; } redefinition A::f redefines Q;",
			[]string{"redefinition target must be a feature, found package"}},
		{"redefinition class source", "class C; class A { feature f; } redefinition C :>> A::f;",
			[]string{"redefinition source must be a feature, found kermlType"}},
		{"redefinition class target", "class C; class A { feature f; } redefinition A::f :>> C;",
			[]string{"redefinition target must be a feature, found kermlType"}},
		{"redefinition features", "class A { feature f; } class B :> A { feature g; } redefinition B::g :>> A::f;", nil},
	})
}

// A Conjugation and a Disjoining relate two Types (§8.3.3.4, §8.3.3.3): a
// feature is a type, a package is not.
func TestRelationshipMemberConjugateAndDisjointEnds(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"conjugate package source", "package Q; class B; conjugate Q ~ B;",
			[]string{"conjugate source must be a type, found package"}},
		{"conjugate package target", "package Q; class B; conjugation conjugate B conjugates Q;",
			[]string{"conjugate target must be a type, found package"}},
		{"conjugate types", "class A; class B; conjugate A ~ B;", nil},
		{"conjugate feature", "feature f; class B; conjugate f ~ B;", nil},
		{"disjoint package source", "package Q; class B; disjoint Q from B;",
			[]string{"disjoint source must be a type, found package"}},
		{"disjoint package target", "package Q; class B; disjoining disjoint B from Q;",
			[]string{"disjoint target must be a type, found package"}},
		{"disjoint types", "class A; class B; disjoint A from B;", nil},
		{"disjoint features", "feature f; feature g; disjoint f from g;", nil},
	})
}

// A FeatureInverting relates two Features (§8.3.4.8) and a TypeFeaturing a
// Feature and a Type (§8.3.4.3): a class is a type but not a feature.
func TestRelationshipMemberInverseAndFeaturingEnds(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"inverse package source", "package Q; feature f; inverse Q of f;",
			[]string{"inverse source must be a feature, found package"}},
		{"inverse package target", "package Q; feature f; inverting inverse f of Q;",
			[]string{"inverse target must be a feature, found package"}},
		{"inverse class source", "class C; feature f; inverse C of f;",
			[]string{"inverse source must be a feature, found kermlType"}},
		{"inverse class target", "class C; feature f; inverse f of C;",
			[]string{"inverse target must be a feature, found kermlType"}},
		{"inverse features", "feature f; feature g; inverse f of g;", nil},
		{"featuring package source", "package Q; class B; featuring Q by B;",
			[]string{"featuring source must be a feature, found package"}},
		{"featuring class source", "class C; class B; featuring C by B;",
			[]string{"featuring source must be a feature, found kermlType"}},
		{"featuring package target", "package Q; feature f; featuring f by Q;",
			[]string{"featuring target must be a type, found package"}},
		{"featuring feature by type", "feature f; class B; featuring f by B;", nil},
		{"featuring feature by feature", "feature f; feature g; featuring f by g;", nil},
	})
}

// A named multiplicity is a Feature and so a Type: it may stand at any Type or
// Feature end, keyword-first or in a declaration clause, but not as a Classifier.
func TestRelationshipMemberNamedMultiplicityIsAType(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"keyword-first type ends", "multiplicity M [1..2]; class B; feature f; " +
			"subtype M :> B; disjoint M from B; conjugate M ~ B; typing f : M; featuring f by M;", nil},
		{"keyword-first feature ends", "multiplicity M [1..2]; feature f; subset M :> f; inverse f of M;", nil},
		{"declaration typing", "multiplicity M [1..2]; feature g : M;", nil},
		{"chain through a multiplicity", "multiplicity M [1..2] { feature x; } feature f; " +
			"subset M.x :> f; feature g :> M.x; feature h : M.x;", nil},
		{"chain through a class", "class C { feature x; } feature f; subset C.x :> f;",
			[]string{"feature chain segment must be a feature, found kermlType"}},
		{"not a classifier", "multiplicity M [1..2]; class B; subclassifier M :> B;",
			[]string{"subclassifier source must be a classifier, found multiplicity"}},
	})
}

// An end that resolves through an alias is judged by what the alias names.
func TestRelationshipMemberEndThroughAlias(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"alias of a type", "class A; class B; alias AB for B; disjoint A from AB;", nil},
		{"alias of a package", "package Q; class A; alias AQ for Q; disjoint A from AQ;",
			[]string{"disjoint target must be a type, found package"}},
	})
}

// An end that names nothing is the name-resolution tier's finding alone: the
// type tier says nothing about it.
func TestRelationshipMemberUnresolvedEndIsSilentAtTypeTier(t *testing.T) {
	for _, src := range []string{
		"class B; disjoint Nope from B;",
		"class B; subtype B :> Nope;",
		"class B; subclassifier Nope :> B;",
		"class B; typing Nope : B;",
		"feature f; subset f :> Nope;",
		"feature f; redefinition Nope :>> f;",
		"class B; conjugate Nope ~ B;",
		"feature f; inverse f of Nope;",
		"feature f; featuring f by Nope;",
	} {
		src := "package P { " + src + " }"
		if diags := diagsIn(t, "a.kerml", src, "name-resolution"); len(diags) != 1 {
			t.Fatalf("%s: expected one name-resolution diagnostic, got %v", src, diags)
		}
		if diags := diagsIn(t, "a.kerml", src, "type"); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// The check is scoped to where the member is written: a relationship member in
// a type body judges its ends in that body's scope.
func TestRelationshipMemberEndsInsideATypeBody(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"body member wrong kind", "package Q; class A { feature f; subset f :> Q; }",
			[]string{"subset target must be a feature, found package"}},
		{"body member clean", "class A { feature f; feature g; subset f :> g; disjoint f from g; }", nil},
	})
}

// The keyword-first forms are KerML's alone: a SysML document has no member to
// judge, and the declaration clauses keep their own rules and wording.
func TestRelationshipMemberDeclarationClauseWordingUnchanged(t *testing.T) {
	checkRelationshipEndCases(t, []relationshipEndCase{
		{"specializes a package", "package Q; class C specializes Q;",
			[]string{"a KerML type may specialize only a type, found package"}},
		{"subsets a package", "package Q; feature f subsets Q;",
			[]string{"subsets target must be a usage or definition, found package"}},
		{"typed by a package", "package Q; feature f : Q;",
			[]string{"type must be a type, found package"}},
	})
}
