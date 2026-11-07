package export_test

import (
	"encoding/json"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

const chainTargetFixture = `package ParameterValues {
	port def PowerPort { out attribute power : ScalarValues::Real; }
	action def Consume { in power : ScalarValues::Real; in gain : ScalarValues::Real; }
	part def Pt { attribute q : ScalarValues::Real; part inner : In; }
	part def In { attribute z : ScalarValues::Real; }
	calc def F { in a : ScalarValues::Real; return r : ScalarValues::Real = a; }
	part def Motor {
		port powerPort : PowerPort;
		part p : Pt;
		perform action consume : Consume {
			in power = powerPort.power;
			in gain = 0.5;
		}
		action boost : Consume {
			in power = powerPort.power * 2.0;
		}
		attribute call = F(p.q);
		attribute nested = p.inner.z;
		calc viaParameter { in pp : Pt; return r : ScalarValues::Real = pp.q; }
	}
}
`

// The API element form states FeatureChainExpression::targetFeature (KerML 1.1
// Beta 2 §8.3.4.8.4) as a reference to the chain's member for every shape a
// chain is written in: a parameter binding, an operator's operand, an
// invocation's argument, a nested chain and a chain from a parameter.
func TestAPIJSONFeatureChainExpressionTargetFeature(t *testing.T) {
	document, err := convert.Convert("chains.sysml", []byte(chainTargetFixture), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("api-json: %v", err)
	}
	byID := map[string]map[string]any{}
	for _, el := range elements {
		if payload, ok := el["payload"].(map[string]any); ok {
			el = payload
		}
		byID[el["@id"].(string)] = el
	}
	refID := func(el map[string]any, property string) string {
		ref, _ := el[property].(map[string]any)
		id, _ := ref["@id"].(string)
		return id
	}
	// ownerOf is the nearest named element the expression is written under.
	ownerOf := func(el map[string]any) string {
		for el != nil {
			if name, _ := el["qualifiedName"].(string); name != "" {
				return name
			}
			el = byID[refID(el, "owner")]
		}
		return ""
	}
	want := map[string]string{
		"ParameterValues::Motor::consume::power":    "ParameterValues::PowerPort::power",
		"ParameterValues::Motor::boost::power":      "ParameterValues::PowerPort::power",
		"ParameterValues::Motor::call":              "ParameterValues::Pt::q",
		"ParameterValues::Motor::nested":            "ParameterValues::In::z",
		"ParameterValues::Motor::viaParameter::r":   "ParameterValues::Pt::q",
		"ParameterValues::Motor::nested::<unnamed>": "",
	}
	got := map[string][]string{}
	for _, el := range byID {
		if el["@type"] != "FeatureChainExpression" {
			continue
		}
		target := byID[refID(el, "targetFeature")]
		name, _ := target["qualifiedName"].(string)
		got[ownerOf(el)] = append(got[ownerOf(el)], name)
	}
	for owner, target := range want {
		if target == "" {
			continue
		}
		found := false
		for _, name := range got[owner] {
			found = found || name == target
		}
		if !found {
			t.Errorf("%s: FeatureChainExpression targetFeature %v, want %s", owner, got[owner], target)
		}
	}
	for owner, names := range got {
		for _, name := range names {
			if name == "" {
				t.Errorf("%s: a FeatureChainExpression states no targetFeature element", owner)
			}
		}
	}
}
