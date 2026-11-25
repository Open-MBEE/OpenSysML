package resolve_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const valuedPayload = `package P {
	part HAPS {
		part payload {
			part userAESA { attribute g = 24; }
			part 'CU-air';
		}
	}
	package Network {
		%s
	}
}`

// A feature valued by another specializes it (KerML 8.3.3.3.4 checkFeatureValuationSpecialization),
// so its body redefines, subsets and reads the members it inherits, and no others.
func TestValuedFeatureInheritsTheMembersOfItsValue(t *testing.T) {
	for _, body := range []string{
		`ref part payload = HAPS.payload { part :>> userAESA; }`,
		`ref part payload = HAPS.payload { part redefines userAESA { attribute h = g; } }`,
		`ref part payload = HAPS.payload { part mine :> userAESA; ref r = 'CU-air'; }`,
		`ref part payload = HAPS.payload; ref part again = payload { part :>> userAESA { attribute :>> g; } }`,
	} {
		if got := resolvedWithModel(t, strings.Replace(valuedPayload, "%s", body, 1)); len(got) != 0 {
			t.Errorf("%s: diagnostics = %q, want none", body, got)
		}
	}
	got := resolvedWithModel(t, strings.Replace(valuedPayload, "%s",
		`ref part payload = HAPS.payload { part :>> nope; }`, 1))
	if len(got) != 1 || !strings.HasPrefix(got[0], "unresolved reference: nope") {
		t.Errorf("diagnostics = %q, want nope unresolved", got)
	}
}

// A `default` value, a declared specialization or a direction leaves the feature's generals as
// declared, and a binding connector constrains values without specializing anything.
func TestValuedFeatureOutsideTheConstraintInheritsNothing(t *testing.T) {
	for _, body := range []string{
		`ref part payload default HAPS.payload { part :>> userAESA; }`,
		`part def Eq; ref part payload : Eq = HAPS.payload { part :>> userAESA; }`,
		`part other; ref part payload :> other = HAPS.payload { part :>> userAESA; }`,
		`action a { in ref part payload = HAPS.payload { part :>> userAESA; } }`,
		`ref part payload { part :>> userAESA; } bind payload = HAPS.payload;`,
	} {
		got := resolvedWithModel(t, strings.Replace(valuedPayload, "%s", body, 1))
		if len(got) != 1 || !strings.HasPrefix(got[0], "unresolved reference: userAESA") {
			t.Errorf("%s: diagnostics = %q, want userAESA unresolved", body, got)
		}
	}
}

// The value's result is a direct supertype of the valued feature; the feature's declared types
// stay those of its declaration, which the value is validated against (KerML 8.4.4.11).
func TestFeatureValueGeneralIsADirectSupertype(t *testing.T) {
	const name = "app.sysml"
	src := strings.Replace(valuedPayload, "%s", `part def Eq;
		part typedSource : Eq;
		ref part viaTyped = typedSource;
		ref part payload = HAPS.payload;
		ref part byDefault default HAPS.payload;
		ref part typed : Eq = HAPS.payload;
		action a { in ref part directed = HAPS.payload; }`, 1)
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(name, root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	r.ResolveDocument(name, root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", r.Diagnostics)
	}
	lookup := func(fqn string) *symbols.Symbol {
		t.Helper()
		found := idx.LookupQualified(fqn)
		if len(found) != 1 {
			t.Fatalf("%s: %d symbols", fqn, len(found))
		}
		return found[0]
	}
	value := lookup("P::HAPS::payload")
	payload := lookup("P::Network::payload")

	if got := model.FeatureValueGeneral(payload); got != value {
		t.Errorf("FeatureValueGeneral(payload) = %v, want P::HAPS::payload", got)
	}
	if !slices.Contains(model.DirectSupertypes(payload), value) {
		t.Errorf("DirectSupertypes(payload) lacks P::HAPS::payload")
	}
	if !slices.Contains(model.AllSupertypes(payload), value) || !model.Conforms(payload, value) {
		t.Errorf("payload does not specialize P::HAPS::payload")
	}
	member, ok := model.LookupMember(payload, "CU-air")
	if !ok || member != lookup("P::HAPS::payload::CU-air") {
		t.Errorf("LookupMember(payload, CU-air) = %v, %v", member, ok)
	}
	viaTyped, eq := lookup("P::Network::viaTyped"), lookup("P::Network::Eq")
	if !model.Conforms(viaTyped, eq) {
		t.Errorf("viaTyped does not conform to the type of its value")
	}
	if slices.Contains(model.FeatureTypeSet(viaTyped), eq) || slices.Contains(model.DeclaredTypes(viaTyped), eq) {
		t.Errorf("the value's type is among viaTyped's declared types")
	}
	for _, fqn := range []string{"P::Network::byDefault", "P::Network::typed", "P::Network::a::directed"} {
		sym := lookup(fqn)
		if got := model.FeatureValueGeneral(sym); got != nil {
			t.Errorf("FeatureValueGeneral(%s) = %v, want none", fqn, got)
		}
		if slices.Contains(model.DirectSupertypes(sym), value) {
			t.Errorf("DirectSupertypes(%s) holds P::HAPS::payload", fqn)
		}
	}
}
