package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestElementPropertiesReadActionNodeParameters pins that each part of an assignment, while, for
// or bare loop statement is a parameter the node owns through a ParameterMembership.
func TestElementPropertiesReadActionNodeParameters(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    private import ScalarValues::*;
    action def A {
        attribute count : Integer;
        action assignCount assign count := 1;
        action whileAct while count < 3 { action step; } until count > 5;
        action forAct for i : Integer in (1, 2, 3) { action each; }
        action loopAct loop { action again; } until count > 9;
        action ifAct if count > 0 { action whenPositive; }
    }
}`)
	described := func(id, property string) []string {
		var out []string
		for _, value := range r.property(id, property).GetValues() {
			out = append(out, value.GetElement().GetId()+" "+value.GetElement().GetMetaclass())
		}
		return out
	}
	expect := func(id, property string, want ...string) {
		t.Helper()
		if got := described(id, property); !slices.Equal(got, want) {
			t.Errorf("%s: %s =\n  %v\nwant\n  %v", id, property, got, want)
		}
	}
	const (
		assign = "P__A__assignCount"
		while  = "P__A__whileAct"
		forID  = "P__A__forAct"
		loop   = "P__A__loopAct"
		ifID   = "P__A__ifAct"
	)
	for _, tc := range []struct {
		node          string
		relationships []string
		parameters    []string
	}{
		{
			assign,
			[]string{
				assign + "_ptargetParameter_om ParameterMembership",
				assign + "_ptarget_om OwningMembership",
				assign + "_pvalueParameter_om ParameterMembership",
			},
			[]string{assign + "_ptargetParameter ReferenceUsage", assign + "_pvalueParameter ReferenceUsage"},
		},
		{
			while,
			[]string{
				while + "_pwhileCondition_om ParameterMembership",
				while + "_loopBody_om ParameterMembership",
				while + "_puntilCondition_om ParameterMembership",
			},
			[]string{
				while + "_pwhileCondition OperatorExpression",
				while + "_loopBody ActionUsage",
				while + "_puntilCondition OperatorExpression",
			},
		},
		{
			forID,
			[]string{
				forID + "_ploopVariable_om FeatureMembership",
				forID + "_pcollectionParameter_om ParameterMembership",
				forID + "_loopBody_om ParameterMembership",
			},
			[]string{forID + "_pcollectionParameter ReferenceUsage", forID + "_loopBody ActionUsage"},
		},
		{
			loop,
			[]string{
				loop + "_pemptyParameter_om ParameterMembership",
				loop + "_loopBody_om ParameterMembership",
				loop + "_puntilCondition_om ParameterMembership",
			},
			[]string{
				loop + "_pemptyParameter ReferenceUsage",
				loop + "_loopBody ActionUsage",
				loop + "_puntilCondition OperatorExpression",
			},
		},
		{
			ifID,
			[]string{ifID + "_pcondition_om ParameterMembership", ifID + "___400___400_om ParameterMembership"},
			[]string{ifID + "_pcondition OperatorExpression", ifID + "___400___400 ActionUsage"},
		},
	} {
		expect(tc.node, "ownedRelationship", tc.relationships...)
		expect(tc.node, "parameter", tc.parameters...)
		expect(tc.node, "input", tc.parameters...)
		for _, parameter := range tc.parameters {
			id, _, _ := strings.Cut(parameter, " ")
			if got := propertyElementIDs(r.property(id, "owner").Values); !slices.Equal(got, []string{tc.node}) {
				t.Errorf("%s: owner = %v, want %s", id, got, tc.node)
			}
			if !strings.HasSuffix(parameter, " ReferenceUsage") {
				continue
			}
			if got := r.property(id, "direction").GetValues(); len(got) != 1 || got[0].GetStringValue() != "in" {
				t.Errorf("%s: direction = %v, want in", id, got)
			}
		}
	}
	expect(forID, "ownedFeature",
		forID+"_ploopVariable ReferenceUsage", forID+"_pcollectionParameter ReferenceUsage", forID+"_loopBody ActionUsage")

	// A value's parameter binds it; the target's nests the feature assigned, which is no parameter.
	expect(assign+"_pvalueParameter", "ownedRelationship", assign+"_pvalueParameter_pvalue_om FeatureValue")
	expect(forID+"_pcollectionParameter", "ownedRelationship", forID+"_pcollectionParameter_pvalue_om FeatureValue")
	expect(assign+"_ptargetParameter", "ownedRelationship", assign+"_ptargetParameter_ptargetFeature_om FeatureMembership")
	expect(assign+"_ptargetParameter", "parameter")
	expect(assign+"_ptargetParameter_ptargetFeature", "ownedRelationship",
		assign+"_ptargetParameter_ptargetFeature_paccessedFeature_om FeatureMembership")
	expect(loop+"_pemptyParameter", "ownedRelationship")

	// A loop's body owns what the braces declare, and is no parameter's owner itself.
	expect(while+"_loopBody", "ownedFeature", while+"___400__step ActionUsage")
	expect(forID+"_loopBody", "ownedFeature", forID+"___400__each ActionUsage")
	expect(loop+"_loopBody", "ownedFeature", loop+"___400__again ActionUsage")
	expect(while+"___400__step", "owner", while+"_loopBody ActionUsage")
	expect(while+"_loopBody", "parameter")
}

// TestElementPropertiesReadLoopBodyParameter pins that a loop body written `action <name>? { … }`
// is the loop's body parameter itself, with no second action wrapping it.
func TestElementPropertiesReadLoopBodyParameter(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    private import ScalarValues::*;
    action def A {
        attribute count : Integer;
        action charge while count < 3 action charging { action step; }
        action spin loop action { action again; } until count > 9;
        action each for i : Integer in (1, 2, 3) action visit { action look; }
    }
}`)
	described := func(id, property string) []string {
		var out []string
		for _, value := range r.property(id, property).GetValues() {
			out = append(out, value.GetElement().GetId()+" "+value.GetElement().GetMetaclass())
		}
		return out
	}
	expect := func(id, property string, want ...string) {
		t.Helper()
		if got := described(id, property); !slices.Equal(got, want) {
			t.Errorf("%s: %s =\n  %v\nwant\n  %v", id, property, got, want)
		}
	}
	const (
		charge = "P__A__charge"
		spin   = "P__A__spin"
		each   = "P__A__each"
	)
	// The written body keeps the id its declaration has, under the statement's scope.
	bodyOf := func(loop string, at int) string {
		t.Helper()
		parameters := described(loop, "parameter")
		if at >= len(parameters) || !strings.HasSuffix(parameters[at], " ActionUsage") || strings.Contains(parameters[at], "_loopBody") {
			t.Fatalf("%s: parameter = %v, want the written body action at %d", loop, parameters, at)
		}
		return strings.TrimSuffix(parameters[at], " ActionUsage")
	}
	charging, visit := bodyOf(charge, 1), bodyOf(each, 1)
	expect(charge, "ownedRelationship",
		charge+"_pwhileCondition_om ParameterMembership", charging+"_om ParameterMembership")
	expect(charge, "parameter", charge+"_pwhileCondition OperatorExpression", charging+" ActionUsage")
	expect(charge, "input", charge+"_pwhileCondition OperatorExpression", charging+" ActionUsage")
	expect(charging, "owner", charge+" WhileLoopActionUsage")
	expect(charging, "ownedMember", charging+"__step ActionUsage")
	if got := r.property(charging, "name").GetValues(); len(got) != 1 || got[0].GetStringValue() != "charging" {
		t.Errorf("charging: name = %v", got)
	}
	expect(each, "parameter", each+"_pcollectionParameter ReferenceUsage", visit+" ActionUsage")
	expect(visit, "owner", each+" ForLoopActionUsage")
	nameless := bodyOf(spin, 1)
	// A body is a referential input of the loop it varies over, written as an action or as a block.
	r2 := parseForElementProperties(t, `package P {
    action def A {
        action plain loop { action again; }
    }
}`)
	for _, body := range []struct {
		properties *elementPropertiesReader
		id         string
	}{{r, charging}, {r, nameless}, {r, visit}, {r2, "P__A__plain_loopBody"}} {
		if got := body.properties.property(body.id, "direction").GetValues(); len(got) != 1 || got[0].GetStringValue() != "in" {
			t.Errorf("%s: direction = %v, want in", body.id, got)
		}
		for property, want := range map[string]bool{"isComposite": false, "isReference": true, "isVariable": true, "mayTimeVary": true} {
			if got := body.properties.property(body.id, property).GetValues(); len(got) != 1 || got[0].GetBoolValue() != want {
				t.Errorf("%s: %s = %v, want %v", body.id, property, got, want)
			}
		}
	}
	if got := described(spin, "parameter"); len(got) != 3 || !strings.HasSuffix(got[1], " ActionUsage") || strings.Contains(got[1], "_loopBody") {
		t.Errorf("spin: parameter = %v, want the empty parameter, the written body action and the until condition", got)
	}
}

// TestElementPropertiesReadLoopBodyImport pins that an import in a loop's `{ … }` body is owned
// by the body action alone, as the conversion writes it, and resolves names in that body.
func TestElementPropertiesReadLoopBodyImport(t *testing.T) {
	r := parseForElementProperties(t, `package Q { part def T; }
package P {
    action def A {
        action named loop { private import Q::*; part t : T; }
        loop { private import Q::*; part u : T; }
        for i : ScalarValues::Integer in (1, 2) { private import Q::*; part v : T; }
    }
}`)
	ids := func(element, property string) []string {
		var out []string
		for _, value := range r.property(element, property).GetValues() {
			out = append(out, value.GetElement().GetId())
		}
		return out
	}
	for _, loop := range []string{"P__A__named", "P__A___401", "P__A___402"} {
		if got := ids(loop, "ownedImport"); len(got) != 0 {
			t.Errorf("%s: ownedImport = %v, want none: the body owns it", loop, got)
		}
		body := loop + "_loopBody"
		imports := ids(body, "ownedImport")
		if len(imports) != 1 {
			t.Fatalf("%s: ownedImport = %v, want the one import the body writes", body, imports)
		}
		if got := ids(imports[0], "importOwningNamespace"); !slices.Equal(got, []string{body}) {
			t.Errorf("%s: importOwningNamespace = %v, want %s", imports[0], got, body)
		}
		if got := ids(body, "ownedRelationship"); len(got) != 2 || got[0] != imports[0] {
			t.Errorf("%s: ownedRelationship = %v, want the import, then the part's membership", body, got)
		}
	}
}

// TestElementPropertiesReadLoopVariableTypeOutsideBody pins that a for loop's variable is typed
// where the loop is written: an import in the body does not rebind the name its type is written by.
func TestElementPropertiesReadLoopVariableTypeOutsideBody(t *testing.T) {
	r := parseForElementProperties(t, `package Q { part def T { attribute q; } }
package R { part def T { attribute r; } }
package P {
    private import R::*;
    part ts : T[*];
    action def A {
        for i : T in ts { private import Q::*; part u : T; attribute a = i.r; }
    }
}`)
	for element, want := range map[string]string{"P__A___400_ploopVariable": "R__T", "P__A___400__u": "Q__T"} {
		var got []string
		for _, value := range r.property(element, "type").GetValues() {
			got = append(got, value.GetElement().GetId())
		}
		if !slices.Equal(got, []string{want}) {
			t.Errorf("%s: type = %v, want %s", element, got, want)
		}
	}
}

// TestElementPropertiesReadLoopBodyImportShadowsOuterName pins KerML 8.2.3.5.4: a name a loop
// body imports is found before the same name in an enclosing namespace, as in a plain action body.
func TestElementPropertiesReadLoopBodyImportShadowsOuterName(t *testing.T) {
	r := parseForElementProperties(t, `package Q { part def T; }
package P {
    part def T;
    part def Holder { part items : T[*]; }
    action def A {
        in h : Holder;
        action plain { private import Q::*; part v : T; }
        action f1 for i : T in h.items { private import Q::*; part v : T; }
        action f2 for i : T in h.items { private import Q::T; part v : T; }
        action lp loop { private import Q::*; part v : T; }
    }
}`)
	for element, want := range map[string]string{
		"P__A__plain__v":         "Q__T",
		"P__A__f1___400__v":      "Q__T",
		"P__A__f2___400__v":      "Q__T",
		"P__A__lp___400__v":      "Q__T",
		"P__A__f1_ploopVariable": "P__T",
		"P__A__f2_ploopVariable": "P__T",
	} {
		var got []string
		for _, value := range r.property(element, "type").GetValues() {
			got = append(got, value.GetElement().GetId())
		}
		if !slices.Equal(got, []string{want}) {
			t.Errorf("%s: type = %v, want %s", element, got, want)
		}
	}
}

// TestParseLoopGuardDoesNotSeeBodyImport pins KerML 8.2.3.5.4 for a loop's `until` guard: it is the
// loop's own expression, so it reads the body's declarations but not a name only the body imports.
func TestParseLoopGuardDoesNotSeeBodyImport(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{
		Content: `package Q { attribute flag : ScalarValues::Integer; }
package P {
    action def A {
        attribute flag : ScalarValues::Boolean;
        action l loop { private import Q::*; } until flag;
        loop { private import Q::*; } until flag;
        attribute xs : ScalarValues::Integer[*];
        loop { private import Q::*; } until xs->ControlFunctions::exists { in x : ScalarValues::Integer; flag };
        loop { private import Q::*; attribute done : ScalarValues::Boolean; } until done;
    }
}`}})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range parsed.GetDiagnostics() {
		t.Errorf("unexpected diagnostic: %s", d.GetMessage())
	}
}

// TestParseLoopGuardBodyExpressionDoesNotSeeBodyImport: a body expression in the condition is
// part of the condition, so a name only the loop's body imports is unresolved there.
func TestParseLoopGuardBodyExpressionDoesNotSeeBodyImport(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{
		Content: `package Q { attribute only : ScalarValues::Integer; }
package P {
    action def A {
        attribute xs : ScalarValues::Integer[*];
        loop { private import Q::*; } until xs->ControlFunctions::exists { in x : ScalarValues::Integer; x > only };
    }
}`}})
	if err != nil {
		t.Fatal(err)
	}
	var unresolved int
	for _, d := range parsed.GetDiagnostics() {
		if strings.Contains(d.GetMessage(), "unresolved reference: only") {
			unresolved++
		}
	}
	if unresolved != 1 {
		t.Errorf("unresolved `only` reported %d times, want 1: %v", unresolved, parsed.GetDiagnostics())
	}
}
