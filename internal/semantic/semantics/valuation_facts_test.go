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
