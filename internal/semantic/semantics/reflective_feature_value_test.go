package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A feature's value part declares a FeatureValue the feature owns, whose
// value is the expression element (KerML 8.3.3.3.5: Feature::valuation
// subsets ownedMembership).
func TestValuedFeatureOwnsItsFeatureValue(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			part def R {
				attribute mass = 3 { doc /* the mass */ }
				attribute count default := 1 + 2;
				attribute plain;
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	r := member(t, m, pkg, "R")
	mass := member(t, m, r, "mass")
	count := member(t, m, r, "count")
	plain := member(t, m, r, "plain")

	if got := m.ValueExpression(plain); got != nil {
		t.Errorf("value expression of plain = %v, want none", got)
	}
	expr := m.ValueExpression(mass)
	if expr == nil || expr.Kind != symbols.SymbolExpression {
		t.Fatalf("value expression of mass = %v, want an expression element", expr)
	}
	if m.ValueExpression(mass) != expr {
		t.Errorf("value expression of mass is minted twice")
	}
	if got := m.MetaclassOf(expr); got == nil || got.Name != "LiteralInteger" {
		t.Errorf("expression metaclass = %v, want LiteralInteger", got)
	}

	memberships := m.OwnedMemberships(mass)
	if len(memberships) != 2 {
		t.Fatalf("owned memberships of mass = %d, want its FeatureValue then the comment's", len(memberships))
	}
	fv := memberships[0]
	if got := m.MetaclassOf(fv); got == nil || got.Name != "FeatureValue" {
		t.Errorf("membership metaclass = %v, want FeatureValue", got)
	}
	for feature, want := range map[string]*symbols.Symbol{
		"featureWithValue": mass, "owningRelatedElement": mass, "membershipOwningNamespace": mass, "owner": mass,
		"value": expr, "memberElement": expr, "ownedMemberElement": expr, "ownedRelatedElement": expr,
	} {
		if got, ok := m.ReflectiveElements(fv, feature); !ok || len(got) != 1 || got[0] != want {
			t.Errorf("%s of the FeatureValue = %v (ok %v), want %s", feature, got, ok, want.Name)
		}
	}
	for feature, want := range map[string]bool{"isDefault": false, "isInitial": false} {
		if got, ok := m.ReflectiveFeatureValue(fv, feature); !ok || got.Bool != want {
			t.Errorf("%s of mass's FeatureValue = %v (ok %v), want %v", feature, got, ok, want)
		}
	}
	countValue := m.OwnedMemberships(count)
	if len(countValue) != 1 {
		t.Fatalf("owned memberships of count = %d, want its FeatureValue only", len(countValue))
	}
	for feature, want := range map[string]bool{"isDefault": true, "isInitial": true} {
		if got, ok := m.ReflectiveFeatureValue(countValue[0], feature); !ok || got.Bool != want {
			t.Errorf("%s of count's FeatureValue = %v (ok %v), want %v", feature, got, ok, want)
		}
	}
	if got := m.MetaclassOf(countValue[0].Implicit.Member); got == nil || got.Name != "OperatorExpression" {
		t.Errorf("count's expression metaclass = %v, want OperatorExpression", got)
	}

	rels := m.OwnedRelationships(mass)
	if len(rels) != 2 || rels[0] != fv || rels[1] != memberships[1] {
		names := make([]string, 0, len(rels))
		for _, rel := range rels {
			names = append(names, m.MetaclassOf(rel).Name)
		}
		t.Errorf("owned relationships of mass = %v, want the FeatureValue then the comment's membership", names)
	}
	if got, ok := m.ReflectiveElements(mass, "ownedElement"); !ok || len(got) != 2 || got[0] != expr {
		t.Errorf("ownedElement of mass = %v, want the expression then the comment", got)
	}
	if got, ok := m.ReflectiveElements(mass, "ownedMember"); !ok || len(got) != 1 || got[0] == expr {
		t.Errorf("ownedMember of mass = %v, want the comment only (a FeatureValue is no OwningMembership of a member)", got)
	}

	for feature, want := range map[string]*symbols.Symbol{
		"owner": mass, "owningNamespace": mass, "owningMembership": fv, "owningRelationship": fv,
	} {
		if got, ok := m.ReflectiveElements(expr, feature); !ok || len(got) != 1 || got[0] != want {
			t.Errorf("%s of the expression = %v (ok %v), want %s", feature, got, ok, want.Name)
		}
	}
	for _, feature := range []string{"name", "declaredName", "shortName", "qualifiedName"} {
		if got, ok := m.ReflectiveFeatureValue(expr, feature); !ok || got.Kind != symbols.FilterValueEmpty {
			t.Errorf("%s of the expression = %v (ok %v), want empty", feature, got, ok)
		}
	}
	if got, ok := m.ReflectiveFeatureValue(expr, "value"); !ok || got.Kind != symbols.FilterValueInt || got.Int != 3 {
		t.Errorf("value of the literal = %v (ok %v), want 3", got, ok)
	}
	if got, ok := m.ReflectiveFeatureValue(countValue[0].Implicit.Member, "operator"); !ok || got.Str != "+" {
		t.Errorf("operator of count's expression = %v (ok %v), want +", got, ok)
	}
}

func TestEveryValuePartOwnsAFeatureValue(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			requirement def Q {
				subject s = 3;
				assume constraint a default true;
				require constraint r := false;
				attribute items : ScalarValues::Real[*];
				attribute label = "a\nb";
				attribute hashed = items#(1);
				attribute bracketed = items[1];
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	q := member(t, m, pkg, "Q")
	for name, want := range map[string]struct {
		metaclass          string
		isDefault, initial bool
	}{
		"s":         {"LiteralInteger", false, false},
		"a":         {"LiteralBoolean", true, false},
		"r":         {"LiteralBoolean", false, true},
		"label":     {"LiteralString", false, false},
		"hashed":    {"IndexExpression", false, false},
		"bracketed": {"OperatorExpression", false, false},
	} {
		sym := member(t, m, q, name)
		expr := m.ValueExpression(sym)
		if expr == nil {
			t.Errorf("%s: no value expression", name)
			continue
		}
		if got := m.MetaclassOf(expr); got == nil || got.Name != want.metaclass {
			t.Errorf("%s: expression metaclass = %v, want %s", name, got, want.metaclass)
		}
		memberships := m.OwnedMemberships(sym)
		if len(memberships) == 0 || m.MetaclassOf(memberships[0]) == nil || m.MetaclassOf(memberships[0]).Name != "FeatureValue" {
			t.Errorf("%s: first owned membership = %v, want its FeatureValue", name, memberships)
			continue
		}
		fv := memberships[0]
		if got, ok := m.ReflectiveElements(fv, "value"); !ok || len(got) != 1 || got[0] != expr {
			t.Errorf("%s: value of the FeatureValue = %v (ok %v), want the expression", name, got, ok)
		}
		if got, _ := m.ReflectiveFeatureValue(fv, "isDefault"); got.Bool != want.isDefault {
			t.Errorf("%s: isDefault = %v, want %v", name, got.Bool, want.isDefault)
		}
		if got, _ := m.ReflectiveFeatureValue(fv, "isInitial"); got.Bool != want.initial {
			t.Errorf("%s: isInitial = %v, want %v", name, got.Bool, want.initial)
		}
		if rels := m.OwnedRelationships(sym); len(rels) == 0 || rels[0] != fv {
			t.Errorf("%s: ownedRelationship does not lead with the FeatureValue: %v", name, rels)
		}
	}
	label := m.ValueExpression(member(t, m, q, "label"))
	if got, ok := m.ReflectiveFeatureValue(label, "value"); !ok || got.Str != "a\nb" {
		t.Errorf("value of the string literal = %q (ok %v), want the decoded %q", got.Str, ok, "a\nb")
	}
}
