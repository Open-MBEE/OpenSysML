package libs

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The general a feature value implies (KerML 8.3.3.3.4) reads from the recorded interface as from
// the parse: a supertype the feature inherits members through, not one of its declared types.
func TestRecordedFeatureValueGeneral(t *testing.T) {
	const name = "valued.sysml"
	const text = `package P {
		part HAPS { part payload { part 'CU-air'; } }
		part def Eq;
		part typedSource : Eq;
		ref part viaTyped = typedSource;
		ref part payload = HAPS.payload;
		ref part byDefault default HAPS.payload;
	}`
	idx := NewModelIndex()
	root := parser.New(source.New(name, []byte(text))).ParseFile()
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	r.ResolveDocument(name, root)

	rec, err := WriteInterface(name, source.KindSysML, idx, r, model, nil, nil)
	if err != nil {
		t.Fatalf("WriteInterface: %v", err)
	}
	recorded, err := symbols.BuildRecorded(rec.Scope, name)
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	ridx := NewModelIndex()
	ridx.AddRecordedDocument(name, source.KindSysML, recorded, nil)
	rr := resolve.New(ridx)
	rmodel := semantics.NewModel(rr)
	rr.SetModel(rmodel)

	for _, c := range []struct {
		label string
		idx   *symbols.Index
		model *semantics.Model
	}{{"parsed", idx, model}, {"recorded", ridx, rmodel}} {
		lookup := func(fqn string) *symbols.Symbol {
			t.Helper()
			found := c.idx.LookupQualified(fqn)
			if len(found) != 1 {
				t.Fatalf("%s %s: %d symbols", c.label, fqn, len(found))
			}
			return found[0]
		}
		value, payload := lookup("P::HAPS::payload"), lookup("P::payload")
		if got := c.model.FeatureValueGeneral(payload); got != value {
			t.Errorf("%s: FeatureValueGeneral(payload) = %v, want P::HAPS::payload", c.label, got)
		}
		if !slices.Contains(c.model.DirectSupertypes(payload), value) {
			t.Errorf("%s: DirectSupertypes(payload) lacks P::HAPS::payload", c.label)
		}
		if _, ok := c.model.LookupMember(payload, "CU-air"); !ok {
			t.Errorf("%s: payload does not inherit CU-air", c.label)
		}
		if slices.Contains(c.model.FeatureTypeSet(lookup("P::viaTyped")), lookup("P::Eq")) {
			t.Errorf("%s: the value's type is among viaTyped's declared types", c.label)
		}
		if got := c.model.FeatureValueGeneral(lookup("P::byDefault")); got != nil {
			t.Errorf("%s: FeatureValueGeneral(byDefault) = %v, want none", c.label, got)
		}
	}
}
