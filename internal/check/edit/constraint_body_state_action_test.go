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

func TestAddExhibitStateRedefines(t *testing.T) {
	source := "package P {\n" +
		"    state def S;\n" +
		"    part def Q { exhibit state cycle : S; }\n" +
		"    part def R :> Q { }\n" +
		"}\n"
	model := loadContent(t, "exhibit-state-redefines.sysml", source)
	op := AddMember("P::R", "exhibit state", "")
	op.Redefines = []string{"cycle"}
	result := applyOne(t, model, op)
	if !strings.Contains(string(result.Content), "exhibit state :>> cycle;") {
		t.Fatalf("redefined exhibit state notation missing:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "exhibit-state-redefines.sysml", string(result.Content)))
}

func TestAddMemberReferenceKindsRejectRedefines(t *testing.T) {
	model := loadContent(t, "reference-redefines.sysml", "package P { part def Holder; }\n")
	for _, test := range []struct {
		kind, reference string
	}{
		{"perform", "actions.start"},
		{"exhibit", "states.running"},
	} {
		t.Run(test.kind, func(t *testing.T) {
			op := AddMember("P::Holder", test.kind, test.reference)
			op.Redefines = []string{"existing"}
			addFailure(t, model, op, FailureIllegalKind)
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

func TestAddNewMemberNamesTaken(t *testing.T) {
	model := loadContent(t, "new-member-names.sysml",
		"package P {\n"+
			"    state def S;\n"+
			"    action def A;\n"+
			"    part def Toaster { state running : S; }\n"+
			"    part toaster : Toaster;\n"+
			"    part def Host {\n"+
			"        assert constraint existing { true }\n"+
			"        exhibit state cycle : S;\n"+
			"        state running : S;\n"+
			"    }\n"+
			"    state def Actions { state work; }\n"+
			"}\n")
	tests := []struct {
		name string
		op   Operation
	}{
		{"assert constraint", AddMember("P::Host", "assert constraint", "existing")},
		{"exhibit state", AddMember("P::Host", "exhibit state", "cycle")},
		{"do action", AddMember("P::Actions", "do action", "work")},
		{"exhibit reference", AddMember("P::Host", "exhibit", "toaster.running")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addFailure(t, model, test.op, FailureMemberNameTaken)
		})
	}
}

func TestAddNewMemberModifierRefusals(t *testing.T) {
	source := "package P {\n" +
		"    state def S;\n" +
		"    action def A;\n" +
		"    part def Toaster { state running : S; }\n" +
		"    part toaster : Toaster;\n" +
		"    part def Host;\n" +
		"    state def Actions;\n" +
		"}\n"
	model := loadContent(t, "new-member-modifiers.sysml", source)
	kinds := []struct {
		kind, owner, name, typ string
	}{
		{"assert constraint", "P::Host", "assertion", ""},
		{"assert not constraint", "P::Host", "assertion", ""},
		{"exhibit state", "P::Host", "shown", "S"},
		{"exhibit", "P::Host", "toaster.running", ""},
		{"entry action", "P::Actions", "enter", "A"},
		{"do action", "P::Actions", "processing", "A"},
		{"exit action", "P::Actions", "leave", "A"},
	}
	for _, test := range kinds {
		t.Run(test.kind, func(t *testing.T) {
			for _, modifier := range []string{"abstract", "direction", "default"} {
				t.Run(modifier, func(t *testing.T) {
					op := AddMember(test.owner, test.kind, test.name)
					op.Type = test.typ
					switch modifier {
					case "abstract":
						op.IsAbstract = true
					case "direction":
						op.Direction = "in"
					case "default":
						op.IsDefault = true
					}
					addFailure(t, model, op, FailureIllegalKind)
				})
			}
		})
	}
}

func TestAddNewMemberKindsRejectKerML(t *testing.T) {
	model := loadContent(t, "new-member.kerml", "package P;\n")
	for _, kind := range []string{
		"assert constraint", "assert not constraint", "exhibit state", "exhibit",
		"entry action", "do action", "exit action",
	} {
		t.Run(kind, func(t *testing.T) {
			addFailure(t, model, AddMember("", kind, "member"), FailureIllegalKind)
		})
	}
}

func TestAddExhibitKindsRequireBehaviorUsageOwner(t *testing.T) {
	model := loadContent(t, "exhibit-enum.sysml",
		"package P { state def S; enum def E { enum one; } }\n")
	for _, op := range []Operation{
		func() Operation {
			op := AddMember("P::E", "exhibit state", "shown")
			op.Type = "S"
			return op
		}(),
		AddMember("P::E", "exhibit", "someState"),
	} {
		failure := addFailure(t, model, op, FailureIllegalKind)
		if !strings.Contains(failure.Message, "is not admitted in the body") {
			t.Fatalf("owner refusal = %q; expected behavior-usage admission refusal", failure.Message)
		}
	}
}

func TestAddNewMemberKindsRequireNames(t *testing.T) {
	model := loadContent(t, "unnamed-new-members.sysml",
		"package P { state def S; action def A; part def Host; state def Actions; }\n")
	exhibitState := AddMember("P::Host", "exhibit state", "")
	exhibitState.Type = "S"
	doAction := AddMember("P::Actions", "do action", "")
	doAction.Type = "A"
	for _, op := range []Operation{exhibitState, doAction} {
		addFailure(t, model, op, FailureInvalidName)
	}
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
