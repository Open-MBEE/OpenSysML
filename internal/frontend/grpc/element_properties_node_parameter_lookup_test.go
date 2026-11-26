package grpc

import (
	"slices"
	"strings"
	"testing"
)

// A parameter an action's notation states reads the same by qualified name as by id, before and after its
// owner is read; one the element form holds no element for is not found rather than answered empty.
func TestElementPropertiesReadANodeParameterByNameAsById(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    private import ScalarValues::*;
    attribute def Sig { attribute v : Integer; }
    action def Base {
        attribute count : Integer := 0;
        action assignCount assign count := count + 1;
        action whileAct while count < 5 { assign count := count + 1; }
        action ifAct if count > 3 { assign count := 0; } else { assign count := 1; }
        action nested;
        action termAct terminate nested;
    }
    part def Sys {
        port t;
        part target;
        action msg {
            action s send new Sig(1) via t to target;
            then action acc1 accept r : Sig via t;
        }
    }
}`)
	for _, name := range []string{
		"P::Base::ifAct::ifTest", "P::Base::ifAct::thenClause", "P::Base::ifAct::elseClause", "P::Base::whileAct::whileTest",
		"P::Sys::msg::s::payload", "P::Sys::msg::s::sender", "P::Sys::msg::s::receiver", "P::Sys::msg::acc1::receiver",
	} {
		owner := name[:strings.LastIndex(name, "::")]
		named := r.read(name, "qualifiedName", "owner")
		if named.Error != "" || named.Element == nil {
			t.Errorf("%s: %q, want the parameter", name, named.Error)
			continue
		}
		if got := named.Properties[0].Values; len(got) != 1 || got[0].GetStringValue() != name {
			t.Errorf("%s: qualifiedName = %v", name, got)
		}
		if got := elementNames(named.Properties[1].Values); !slices.Equal(got, []string{owner}) {
			t.Errorf("%s: owner = %v, want %s", name, got, owner)
		}
		byID := r.read(named.Element.Id, "qualifiedName")
		if byID.Error != "" || len(byID.Properties[0].Values) != 1 || byID.Properties[0].Values[0].GetStringValue() != name {
			t.Errorf("%s: read by its id %s answers %v %q", name, named.Element.Id, byID.Properties, byID.Error)
		}
	}
	want := []string{"P::Base::ifAct::ifTest", "P::Base::ifAct::thenClause", "P::Base::ifAct::elseClause"}
	for _, property := range []string{"parameter", "input"} {
		if got := elementNames(r.property("P::Base::ifAct", property).Values); !slices.Equal(got, want) {
			t.Errorf("ifAct: %s = %v after its parameters were read by name, want %v", property, got, want)
		}
	}
	for _, name := range []string{"P::Base::whileAct::body", "P::Base::termAct::terminatedOccurrence", "P::Base::assignCount::target"} {
		if resp := r.read(name, "qualifiedName"); !strings.Contains(resp.Error, "element not found") {
			t.Errorf("%s: answered %v %q, want element not found", name, resp.Properties, resp.Error)
		}
	}
}
