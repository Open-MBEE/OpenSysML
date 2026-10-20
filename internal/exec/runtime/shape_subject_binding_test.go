package runtime

import (
	"testing"
)

const nestedSubjectShapeSrc = `
	package test {
		requirement def R1 { subject s : ScalarValues::Integer; require constraint { s > 0 } }
		requirement def RP { subject s : PA; require constraint { s.v > 0 } }
		part def PA { attribute v : ScalarValues::Integer = 5; }
		part def P1 {
			attribute sub : ScalarValues::Integer = 3;
			part pa : PA;
			requirement rq : R1 { subject s :>> s = sub; }
			requirement rqBare : R1 { subject = sub; }
			requirement rqPlain : R1;
			satisfy requirement sat : RP by pa;
		}
	}
`

// TestNestedRequirementUsageShapeBindsItsSubject: a usage binding its subject (a named member,
// a bare `subject = x;`, a satisfy's `by`) types the object it holds; one binding nothing does not.
func TestNestedRequirementUsageShapeBindsItsSubject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, nestedSubjectShapeSrc)
	p1 := oneSymbol(t, idx, "test::P1")

	holders := map[string]*EffectiveFeature{}
	for _, feat := range ctx.FeaturesOf(p1) {
		f := feat
		holders[f.Name] = &f
	}
	for _, name := range []string{"rq", "rqBare", "sat"} {
		holder := holders[name]
		if holder == nil {
			t.Fatalf("P1 carries no %s", name)
		}
		if typ := ctx.CompositeTypeOf(holder); typ != holder.Symbol {
			t.Fatalf("%s holds an object of %s, want the usage itself", name, SymbolText(typ))
		}
		var subject *EffectiveFeature
		for _, feat := range ctx.FeaturesOf(holder.Symbol) {
			if feat.Name == "s" {
				f := feat
				subject = &f
			}
		}
		if subject == nil {
			t.Fatalf("%s carries no subject s", name)
		}
		if subject.DefaultValue == nil || !ctx.valueBinds(subject) {
			t.Errorf("%s.s binds no value", name)
		}
	}
	if typ := ctx.CompositeTypeOf(holders["rqPlain"]); typ == nil || typ.Name != "R1" {
		t.Errorf("rqPlain holds an object of %s, want its definition R1", SymbolText(typ))
	}
}
