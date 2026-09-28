package edit

import (
	"strings"
	"testing"
)

func TestAddConstraintBodies(t *testing.T) {
	tests := []struct {
		name, source, owner, kind, member, expression, want string
	}{
		{
			name:   "constraint definition",
			source: "package P {\n    constraint def Base;\n}\n",
			owner:  "P", kind: "constraint def", member: "C",
			expression: "true",
			want:       "package P {\n    constraint def Base;\n    constraint def C { true }\n}\n",
		},
		{
			name:   "constraint usage",
			source: "package P {\n    constraint def Base;\n}\n",
			owner:  "P", kind: "constraint", member: "c", expression: "true",
			want: "package P {\n    constraint def Base;\n    constraint c { true }\n}\n",
		},
		{
			name:   "root body",
			source: "part def Existing;\n",
			kind:   "constraint def", member: "RootConstraint", expression: "true",
			want: "part def Existing;\nconstraint def RootConstraint { true }\n",
		},
		{
			name:   "indented multiline body",
			source: "package P {\n    attribute x = 2;\n}\n",
			owner:  "P", kind: "constraint", member: "c",
			expression: "    x > 1\n    and x < 4",
			want:       "package P {\n    attribute x = 2;\n    constraint c {\n        x > 1\n        and x < 4\n    }\n}\n",
		},
		{
			name:   "tab-indented multiline body",
			source: "package P {\n\tattribute x = 2;\n}\n",
			owner:  "P", kind: "constraint", member: "c",
			expression: "    x > 1\n    and x < 4",
			want:       "package P {\n\tattribute x = 2;\n\tconstraint c {\n\t\tx > 1\n\t\tand x < 4\n\t}\n}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "constraint-body.sysml", test.source)
			op := AddMember(test.owner, test.kind, test.member)
			op.BodyExpression = test.expression
			result := applyOne(t, model, op)
			if got := string(result.Content); got != test.want {
				t.Fatalf("notation:\n%s\nwant:\n%s", got, test.want)
			}
			requireClean(t, loadContent(t, "constraint-body.sysml", test.want))
		})
	}
}

func TestAddAssertConstraintForms(t *testing.T) {
	source := "package P {\n" +
		"    constraint def C;\n" +
		"    attribute x = 2;\n" +
		"}\n"
	tests := []struct {
		name, kind, member, typ, expression, want string
	}{
		{"asserted", "assert constraint", "positive", "", "x > 1",
			"assert constraint positive { x > 1 }"},
		{"negated", "assert not constraint", "negative", "", "x > 3",
			"assert not constraint negative { x > 3 }"},
		{"unnamed typed", "assert constraint", "", "C", "", "assert constraint : C;"},
		{"unnamed body", "assert constraint", "", "", "x > 1",
			"assert constraint { x > 1 }"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "assert-constraint.sysml", source)
			op := AddMember("P", test.kind, test.member)
			op.Type, op.BodyExpression = test.typ, test.expression
			result := applyOne(t, model, op)
			if !strings.Contains(string(result.Content), "    "+test.want) {
				t.Fatalf("notation lacks %q:\n%s", test.want, result.Content)
			}
			requireClean(t, loadContent(t, "assert-constraint.sysml", string(result.Content)))
		})
	}
}

func TestAddExhibitStateAndReference(t *testing.T) {
	source := "package P {\n" +
		"    state def Cycle;\n" +
		"    part def Toaster {\n" +
		"        state running : Cycle;\n" +
		"    }\n" +
		"    part toaster : Toaster;\n" +
		"    part def Holder;\n" +
		"    part holder : Holder;\n" +
		"}\n"
	tests := []struct {
		name, owner, kind, member, typ, want string
	}{
		{"part definition", "P::Holder", "exhibit state", "cycle", "Cycle", "exhibit state cycle : Cycle;"},
		{"part usage", "P::holder", "exhibit state", "cycle", "Cycle", "exhibit state cycle : Cycle;"},
		{"reference", "P::Holder", "exhibit", "toaster.running", "", "exhibit toaster.running;"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "exhibit.sysml", source)
			op := AddMember(test.owner, test.kind, test.member)
			op.Type = test.typ
			result := applyOne(t, model, op)
			if !strings.Contains(string(result.Content), test.want) {
				t.Fatalf("notation lacks %q:\n%s", test.want, result.Content)
			}
			requireClean(t, loadContent(t, "exhibit.sysml", string(result.Content)))
		})
	}
}

func TestAddStateActionsInStateBodies(t *testing.T) {
	source := "package P {\n" +
		"    action def A;\n" +
		"    state def S {\n" +
		"        state heating;\n" +
		"    }\n" +
		"    state machine : S {\n" +
		"        state heating;\n" +
		"    }\n" +
		"}\n"
	tests := []struct {
		name, owner, kind, want string
	}{
		{"entry definition", "P::S", "entry", "entry action start : A;"},
		{"do definition", "P::S", "do", "do action work : A;"},
		{"exit definition", "P::S", "exit", "exit action finish : A;"},
		{"state usage", "P::machine", "do", "do action work : A;"},
		{"substate", "P::S::heating", "do", "do action work : A;"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "state-action.sysml", source)
			kindName := test.kind + " action"
			op := AddMember(test.owner, kindName, "work")
			if test.kind == "entry" {
				op.MemberName = "start"
			}
			if test.kind == "exit" {
				op.MemberName = "finish"
			}
			op.Type = "A"
			result := applyOne(t, model, op)
			if !strings.Contains(string(result.Content), test.want) {
				t.Fatalf("notation lacks %q:\n%s", test.want, result.Content)
			}
			requireClean(t, loadContent(t, "state-action.sysml", string(result.Content)))
		})
	}
}

func TestAddStateActionRefusals(t *testing.T) {
	for _, test := range []struct {
		name, source, kind string
	}{
		{"duplicate do", "state def S { do; }\n", "do"},
		{"duplicate entry", "state def S { entry; }\n", "entry"},
		{"duplicate exit", "state def S { exit; }\n", "exit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "duplicate-action.sysml", test.source)
			op := AddMember("S", test.kind+" action", "added")
			addFailure(t, model, op, FailureIllegalKind)
		})
	}
	addFailure(t, loadContent(t, "part-action.sysml", "part def P;\n"),
		AddMember("P", "do action", "a"), FailureIllegalKind)
}

func TestAddConstraintBodyRefusals(t *testing.T) {
	source := "package P {\n    attribute x = 2;\n    enum def E { enum a; }\n}\n"
	model := loadContent(t, "body-expression.sysml", source)
	op := AddMember("P", "part", "p")
	op.BodyExpression = "true"
	if failure := addFailure(t, model, op, FailureIllegalKind); !strings.Contains(failure.Message, `kind "part" cannot state a body expression`) {
		t.Fatalf("body-expression refusal = %q", failure.Message)
	}
	for _, expression := range []string{"x >", "x > 1; y"} {
		op = AddMember("P", "constraint", "bad")
		op.BodyExpression = expression
		addFailure(t, model, op, FailureInvalidValue)
	}
	op = AddMember("P", "constraint", "")
	addFailure(t, model, op, FailureInvalidName)
	op = AddMember("P::E", "assert constraint", "assertion")
	op.BodyExpression = "true"
	addFailure(t, model, op, FailureIllegalKind)
}

func TestAddMemberBodyAndValueTogether(t *testing.T) {
	model := loadContent(t, "body-and-value.sysml", "package P { attribute x = 2; }\n")
	op := AddMember("P", "constraint", "c")
	op.Value, op.BodyExpression = "x", "x > 1"
	result := applyOne(t, model, op)
	if !strings.Contains(string(result.Content), "constraint c = x { x > 1 }") {
		t.Fatalf("value and body notation missing:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "body-and-value.sysml", string(result.Content)))
}
