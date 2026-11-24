package grpc

import (
	"slices"
	"strings"
	"testing"
)

// TestElementPropertiesReadActionNodeParameters pins what an assignment, a while loop, a for loop
// and a bare loop own for the parts of their statements (SysML.xtext AssignmentNodeDeclaration,
// WhileLoopNode, ForLoopNode): each part is a parameter the node owns through a
// ParameterMembership, as an if action's already are.
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

// TestElementPropertiesReadLoopBodyParameter pins a loop body written as the action it is
// (SysML.xtext ActionBodyParameter, `action <name>? { … }`): that action is the loop's body
// parameter itself, named or not, and no second action wraps it.
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
	if got := r.property(charging, "direction").GetValues(); len(got) != 1 || got[0].GetStringValue() != "in" {
		t.Errorf("charging: direction = %v, want in", got)
	}
	expect(each, "parameter", each+"_pcollectionParameter ReferenceUsage", visit+" ActionUsage")
	expect(visit, "owner", each+" ForLoopActionUsage")
	if got := described(spin, "parameter"); len(got) != 3 || !strings.HasSuffix(got[1], " ActionUsage") || strings.Contains(got[1], "_loopBody") {
		t.Errorf("spin: parameter = %v, want the empty parameter, the written body action and the until condition", got)
	}
}
