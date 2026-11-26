package grpc

import (
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
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
        action forAct for i in (1, 2, 3) { assign count := i; }
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
	if resp := r.read("P::Base::assignCount::count", "qualifiedName"); !strings.Contains(resp.Error, "element not found") {
		t.Errorf("P::Base::assignCount::count: answered %v %q, want element not found", resp.Properties, resp.Error)
	}
}

// A parameter the element form holds no element for reads by name as the feature of its action's kind it
// redefines, the one the name stated before the parameter was held; a bare terminate states none and inherits it.
func TestElementPropertiesReadAnUnwrittenNodeParameterAsTheOneItRedefines(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    private import ScalarValues::*;
    action def Base {
        attribute count : Integer := 0;
        action nested;
        action assignCount assign count := count + 1;
        action whileAct while count < 5 { assign count := count + 1; }
        action forAct for i in (1, 2, 3) { assign count := i; }
        action termAct terminate nested;
        action bare terminate;
    }
}`)
	for _, tc := range []struct{ name, owner, parameter string }{
		{"P::Base::termAct::terminatedOccurrence", "Actions::terminateActions", "terminatedOccurrence"},
		{"P::Base::bare::terminatedOccurrence", "Actions::terminateActions", "terminatedOccurrence"},
		// A nested assignment's target redefines that of the subaction feature (SysML v2 Table 33).
		{"P::Base::assignCount::target", "Actions::Action::assignments", "target"},
		{"P::Base::assignCount::replacementValues", "Actions::AssignmentAction", "replacementValues"},
		{"P::Base::whileAct::body", "Actions::WhileLoopAction", "body"},
		{"P::Base::forAct::seq", "Actions::ForLoopAction", "seq"},
		{"P::Base::forAct::body", "Actions::ForLoopAction", "body"},
	} {
		resp := r.read(tc.name, "qualifiedName", "name", "owner", "owningType", "featuringType")
		if resp.Error != "" || len(resp.Properties) != 5 {
			t.Errorf("%s: answered %v %q", tc.name, resp.Properties, resp.Error)
			continue
		}
		got := []string{resp.Properties[0].Values[0].GetStringValue(), resp.Properties[1].Values[0].GetStringValue()}
		if want := []string{tc.owner + "::" + tc.parameter, tc.parameter}; !slices.Equal(got, want) {
			t.Errorf("%s: qualifiedName and name = %v, want %v", tc.name, got, want)
		}
		for _, property := range resp.Properties[2:] {
			if got := elementNames(property.Values); !slices.Equal(got, []string{tc.owner}) {
				t.Errorf("%s: %s = %v, want %s", tc.name, property.Name, got, tc.owner)
			}
		}
		byID := r.read(resp.Element.Id, "qualifiedName")
		if byID.Error != "" || byID.Properties[0].Values[0].GetStringValue() != tc.owner+"::"+tc.parameter {
			t.Errorf("%s: read by its id %s answers %v %q", tc.name, resp.Element.Id, byID.Properties, byID.Error)
		}
	}
}

// The variable a `for` loop declares is a parameter the loop owns (SysML 8.3.17.10), read under the loop's name.
func TestElementPropertiesReadAForLoopVariableByName(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    action def Base {
        attribute count = 0;
        action forAct for i in (1, 2, 3) { assign count := i; }
    }
}`)
	named := r.read("P::Base::forAct::i", "qualifiedName", "name", "owner", "owningType", "featuringType")
	if named.Error != "" || len(named.Properties) != 5 {
		t.Fatalf("P::Base::forAct::i: answered %v %q", named.Properties, named.Error)
	}
	if got := named.Properties[0].Values; len(got) != 1 || got[0].GetStringValue() != "P::Base::forAct::i" {
		t.Errorf("qualifiedName = %v", got)
	}
	if got := named.Properties[1].Values; len(got) != 1 || got[0].GetStringValue() != "i" {
		t.Errorf("name = %v", got)
	}
	for _, property := range named.Properties[2:] {
		if got := elementNames(property.Values); !slices.Equal(got, []string{"P::Base::forAct"}) {
			t.Errorf("%s = %v, want P::Base::forAct", property.Name, got)
		}
	}
	byID := r.read(named.Element.Id, "qualifiedName")
	if byID.Error != "" || byID.Properties[0].Values[0].GetStringValue() != "P::Base::forAct::i" {
		t.Errorf("read by its id %s answers %v %q", named.Element.Id, byID.Properties, byID.Error)
	}
}

// A parameter in a document the model excludes answers that document's error, whether or not the form holds it.
func TestElementPropertiesReadANodeParameterOfAnExcludedDocument(t *testing.T) {
	r := parseDocumentsForElementProperties(t, &pb.SourceDocument{Name: "c.sysml", Source: &pb.SourceDocument_Content{
		Content: "package C { action def D { attribute n = 0; action w while n < 1 { } } part def Broken { ; }",
	}})
	for _, name := range []string{"C::D::w", "C::D::w::whileTest", "C::D::w::body"} {
		if resp := r.read(name, "qualifiedName"); !strings.Contains(resp.Error, "syntax error") {
			t.Errorf("%s: answered %v %q, want its document's syntax error", name, resp.Properties, resp.Error)
		}
	}
}

// A loop's variable reads by the name its id read answers: quoted, by its short name, and through an inheriting definition.
func TestElementPropertiesReadALoopVariableByEveryNameItAnswers(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    action def Base {
        action quoted for 'my var' in (1, 2) { }
        action short for <sv> in (1, 2) { }
        action plain for i in (1, 2) { }
        action escaped for 'a\tb' in (1, 2) { action atb; }
        action step for s in (1, 2) { action inner for wrong in (1, 2) { } }
        action 'step::inner' for right in (1, 2) { }
        action 'lone::loop' for only in (1, 2) { }
    }
    action def Derived :> Base;
}`)
	for name, declared := range map[string]string{
		"P::Base::quoted::'my var'": "P::Base::quoted::'my var'",
		"P::Base::short::sv":        "P::Base::short::sv",
		"P::Derived::plain::i":      "P::Base::plain::i",
		"P::Base::escaped::'a\\tb'": "P::Base::escaped::'a\\tb'",
	} {
		resp := r.read(name, "qualifiedName")
		if resp.Error != "" || resp.Element == nil {
			t.Errorf("%s: %q, want the loop's variable", name, resp.Error)
			continue
		}
		if got := resp.Element.Metaclass; got != "ReferenceUsage" {
			t.Errorf("%s: answered a %s, want the variable", name, got)
		}
		if want := r.read(declared, "qualifiedName"); want.Element == nil || resp.Element.Id != want.Element.Id {
			t.Errorf("%s: answered %v, want the element of %s", name, resp.Element, declared)
		}
	}
	// A loop whose name holds `::` is reached by its quoted segment, declared or inherited, whether or not the path also nests.
	for name, variable := range map[string]string{
		"P::Base::'step::inner'::right":    "right",
		"P::Derived::'step::inner'::right": "right",
		"P::Base::'lone::loop'::only":      "only",
		"P::Derived::'lone::loop'::only":   "only",
	} {
		resp := r.read(name, "name")
		if resp.Error != "" || resp.Element == nil || resp.Element.Metaclass != "ReferenceUsage" {
			t.Errorf("%s: answered %v %q, want the loop's variable", name, resp.Element, resp.Error)
		} else if got := resp.Properties[0].Values[0].GetStringValue(); got != variable {
			t.Errorf("%s: answered the variable %s, want %s", name, got, variable)
		}
	}
	if resp := r.read("P::Derived::'step::inner'::wrong", "qualifiedName"); !strings.Contains(resp.Error, "element not found") {
		t.Errorf("a loop named 'step::inner' answers another loop's variable: %v %q", resp.Element, resp.Error)
	}
}

// A loop reached through a type whose quoted name differs from another type's only by its quoting is that type's loop.
func TestElementPropertiesReadALoopVariableThroughAQuotedAncestor(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    action def Spaced { action 'a::b' for v in (1, 2) { } }
    action def Bare { action 'a::b' for v in (1, 2) { } }
    action def 'Base ' :> Spaced;
    action def Base :> Bare;
}`)
	for name, declared := range map[string]string{
		"P::'Base '::'a::b'::v": "P::Spaced::'a::b'::v",
		"P::Base::'a::b'::v":    "P::Bare::'a::b'::v",
	} {
		resp, want := r.read(name, "qualifiedName"), r.read(declared, "qualifiedName")
		if want.Error != "" || want.Element == nil {
			t.Fatalf("%s: %q, want the declared variable", declared, want.Error)
		}
		if resp.Error != "" || resp.Element == nil || resp.Element.Id != want.Element.Id {
			t.Errorf("%s: answered %v %q, want %s", name, resp.Element, resp.Error, want.Element.Id)
		}
	}
}
