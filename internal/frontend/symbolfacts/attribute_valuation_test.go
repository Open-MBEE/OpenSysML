package symbolfacts

import (
	"testing"
)

// A redefined attribute in a nested part usage, valued by a sibling's attribute.
const occurrenceConfigModel = `
package OccurrenceConfig {
    part def A { attribute prop_a : ScalarValues::Real; }
    part def B { attribute prop_b : ScalarValues::Real; }
    part system {
        part usage_a : A { attribute :>> prop_a = 100.0; }
        part usage_b : B { attribute :>> prop_b = usage_a.prop_a; }
        part usage_c : B { attribute :>> prop_b = usage_a.prop_a * 2.0; }
        part usage_d : B;
    }
}
`

// Value is the constant an attribute's expression folds to on its own; a value
// written as a reference to another part's attribute has none, but its valuation
// says what is written and what it evaluates to through the features it reads,
// so a client can tell it from an attribute with no value at all.
func TestAttributeValuationTellsAWrittenValueFromNone(t *testing.T) {
	idx, sc := modelWithLibrary(t, occurrenceConfigModel)
	for _, c := range []struct {
		usage     string
		constant  bool
		kind      string
		evaluated float64
	}{
		{"usage_a", true, "literal", 100},
		{"usage_b", false, "featureChain", 100},
		{"usage_c", false, "operator", 200},
	} {
		sym, _ := MemberChain(idx, sc.Semantics, "OccurrenceConfig::system::"+c.usage)
		if sym == nil {
			t.Fatalf("%s not found", c.usage)
		}
		info := Of(sym, sc)
		var attribute *Attribute
		for i := range info.Attributes {
			if info.Attributes[i].Name == "prop_a" || info.Attributes[i].Name == "prop_b" {
				attribute = info.Attributes[i]
			}
		}
		if attribute == nil {
			t.Fatalf("%s: no prop attribute in %+v", c.usage, info.Attributes)
		}
		if (attribute.Value != nil) != c.constant {
			t.Errorf("%s: constant value %+v, want present=%v", c.usage, attribute.Value, c.constant)
		}
		v := attribute.Valuation
		if v == nil || v.Kind != c.kind {
			t.Fatalf("%s: valuation %+v, want kind %s", c.usage, v, c.kind)
		}
		if v.Evaluated == nil || v.Evaluated.Const == nil || v.Evaluated.Const.AsReal() != c.evaluated {
			t.Errorf("%s: evaluated %+v, want %v", c.usage, v.Evaluated, c.evaluated)
		}
		if c.kind != "literal" && (len(v.References) != 1 || v.References[0].TargetId != "OccurrenceConfig::system::usage_a::prop_a") {
			t.Errorf("%s: references %+v, want usage_a's prop_a", c.usage, v.References)
		}
	}
	sym, _ := MemberChain(idx, sc.Semantics, "OccurrenceConfig::system::usage_d")
	info := Of(sym, sc)
	for _, attribute := range info.Attributes {
		if attribute.Name == "prop_b" && (attribute.Value != nil || attribute.Valuation != nil) {
			t.Errorf("usage_d: prop_b has value %+v valuation %+v, want neither", attribute.Value, attribute.Valuation)
		}
	}
}
