package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const valuationSrc = `
	package P {
		port def PowerPort { out attribute power : ScalarValues::Real; }
		action def Consume { in power : ScalarValues::Real; in gain : ScalarValues::Real; }
		part def Pt { attribute q : ScalarValues::Real; part inner : In; }
		part def In { attribute z : ScalarValues::Real; }
		calc def F { in a : ScalarValues::Real; return r : ScalarValues::Real = a; }
		part def Motor {
			port powerPort : PowerPort;
			part p : Pt;
			attribute limit : ScalarValues::Real default 3.0;
			attribute start : ScalarValues::Real := p.q;
			perform action consume : Consume {
				in power = powerPort.power;
				in gain = 0.5;
			}
			action boost : Consume {
				in power = powerPort.power * 2.0;
			}
			attribute call = F(p.q) + p.inner.z;
			attribute viaCall = F(p.q).r;
			attribute none : ScalarValues::Real;
		}
	}
`

// KerML 1.1 Beta 2 §8.3.3.3.5 Feature::valuation: the value expression a feature
// declares, read as a client sees it — the notation, the kind, the FeatureValue's
// flags and the features it reads, resolved through the reflective tables.
func TestValuationOfReadsTheBoundExpression(t *testing.T) {
	m, root := buildModelWithStdlibUnwalked(t, valuationSrc)
	m.SetSourceText(source.TextOf(map[string]*source.SourceFile{"t.sysml": source.New("t.sysml", []byte(valuationSrc))}, nil))
	pkg, _ := root.LookupLocal("P")
	motor := member(t, m, pkg, "Motor")
	names := func(syms []*symbols.Symbol) string {
		var out []string
		for _, s := range syms {
			out = append(out, s.Name)
		}
		return strings.Join(out, ".")
	}
	for _, c := range []struct {
		feature               *symbols.Symbol
		text, kind, metaclass string
		isDefault, isInitial  bool
		refs                  []string
	}{
		{member(t, m, member(t, m, motor, "consume"), "power"), "powerPort.power", "featureChain", "FeatureChainExpression", false, false, []string{"powerPort.power"}},
		{member(t, m, member(t, m, motor, "consume"), "gain"), "0.5", "literal", "LiteralRational", false, false, nil},
		{member(t, m, member(t, m, motor, "boost"), "power"), "powerPort.power * 2.0", "operator", "OperatorExpression", false, false, []string{"powerPort.power"}},
		{member(t, m, motor, "limit"), "3.0", "literal", "LiteralRational", true, false, nil},
		{member(t, m, motor, "start"), "p.q", "featureChain", "FeatureChainExpression", false, true, []string{"p.q"}},
		{member(t, m, motor, "call"), "F(p.q) + p.inner.z", "operator", "OperatorExpression", false, false, []string{"p.q", "p.inner.z"}},
		// A chain on a call reads what the call reads; its own target, a member
		// of the call's result, is not resolved reflectively yet.
		{member(t, m, motor, "viaCall"), "F(p.q).r", "featureChain", "FeatureChainExpression", false, false, []string{"p.q"}},
	} {
		facts, ok := m.ValuationOf(c.feature)
		if !ok {
			t.Errorf("%s: no valuation", c.feature.Name)
			continue
		}
		if facts.Text != c.text || facts.Kind != c.kind || facts.Metaclass != c.metaclass || facts.IsDefault != c.isDefault || facts.IsInitial != c.isInitial {
			t.Errorf("%s: %+v", c.feature.Name, *facts)
		}
		var got []string
		for _, ref := range facts.References {
			got = append(got, names(ref.Path))
			if ref.Target != ref.Path[len(ref.Path)-1] {
				t.Errorf("%s: reference %q ends at %s", c.feature.Name, ref.Text, ref.Target.Name)
			}
		}
		if strings.Join(got, ";") != strings.Join(c.refs, ";") {
			t.Errorf("%s: references %v, want %v", c.feature.Name, got, c.refs)
		}
	}
	if _, ok := m.ValuationOf(member(t, m, motor, "none")); ok {
		t.Error("a feature declaring no value has a valuation")
	}
}

// The evaluated value reads through the features the expression names to the
// values they are bound to (EvalIn); one that reaches a valueless feature has none.
func TestValuationOfEvaluatesThroughTheFeaturesItReads(t *testing.T) {
	const src = `
	package OccurrenceConfig {
		part def A { attribute prop_a : ScalarValues::Real; }
		part def B { attribute prop_b : ScalarValues::Real; }
		part system {
			part usage_a : A { attribute :>> prop_a = 100.0; }
			part usage_b : B { attribute :>> prop_b = usage_a.prop_a; }
			part usage_c : B { attribute :>> prop_b = usage_a.prop_a * 2.0; }
			part usage_e : B { attribute :>> prop_b = usage_d.prop_b; }
			part usage_d : B;
		}
	}
`
	m, root := buildModelWithStdlibUnwalked(t, src)
	pkg, _ := root.LookupLocal("OccurrenceConfig")
	system := member(t, m, pkg, "system")
	for _, c := range []struct {
		usage, attribute string
		evaluated        float64
		determined       bool
	}{
		{"usage_a", "prop_a", 100, true},
		{"usage_b", "prop_b", 100, true},
		{"usage_c", "prop_b", 200, true},
		{"usage_e", "prop_b", 0, false},
	} {
		facts, ok := m.ValuationOf(member(t, m, member(t, m, system, c.usage), c.attribute))
		if !ok {
			t.Fatalf("%s: no valuation", c.usage)
		}
		if (facts.Evaluated != nil) != c.determined {
			t.Errorf("%s: evaluated %+v, want determined=%v", c.usage, facts.Evaluated, c.determined)
		} else if c.determined && facts.Evaluated.AsReal() != c.evaluated {
			t.Errorf("%s: evaluated %v, want %v", c.usage, facts.Evaluated.AsReal(), c.evaluated)
		}
	}
}
