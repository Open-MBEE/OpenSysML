package edit

import (
	"strings"
	"testing"
)

const transitionTestModel = "package P {\n" +
	"    attribute def CycleStart;\n" +
	"    action def cool;\n" +
	"    state def S {\n" +
	"        state idle;\n" +
	"        state toasting;\n" +
	"    }\n" +
	"}\n"

func TestAddTransitionOptionalClauses(t *testing.T) {
	tests := []struct {
		name    string
		trigger string
		guard   string
		effect  string
		want    string
	}{
		{name: "trigger", trigger: "CycleStart", want: "accept CycleStart"},
		{name: "guard", guard: "true", want: "if true"},
		{name: "effect", effect: "action cool", want: "do action cool"},
		{
			name:    "all clauses",
			trigger: "CycleStart", guard: "true", effect: "action cool",
			want: "accept CycleStart if true do action cool",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "transition.sysml", transitionTestModel)
			requireClean(t, model)
			result := applyOne(t, model, AddTransition(
				"P::S", "start", "idle", "toasting",
				test.trigger, test.guard, test.effect, false,
			))
			if !strings.Contains(string(result.Content), "transition start first idle "+test.want+" then toasting;") {
				t.Fatalf("transition clauses missing:\n%s", result.Content)
			}
			requireClean(t, loadContent(t, "transition.sysml", string(result.Content)))
		})
	}
}

func TestAddTransitionRejectsRepeatedGuardText(t *testing.T) {
	model := loadContent(t, "transition-repeated-guard.sysml", transitionTestModel)
	addFailure(t, model, AddTransition(
		"P::S", "start", "idle", "toasting", "", "true if false", "", false,
	), FailureInvalidValue)
}

func TestAddTransitionAcceptsCompoundGuard(t *testing.T) {
	model := loadContent(t, "transition-compound-guard.sysml",
		"package P { state def S { attribute x : ScalarValues::Real; attribute y : ScalarValues::Real; state idle; state toasting; } }\n")
	requireClean(t, model)
	result := applyOne(t, model, AddTransition(
		"P::S", "start", "idle", "toasting", "", "x > 0 and y < 1", "", false,
	))
	if !strings.Contains(string(result.Content),
		"transition start first idle if x > 0 and y < 1 then toasting;") {
		t.Fatalf("compound guard missing:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "transition-compound-guard.sysml", string(result.Content)))
}

func TestAddTransitionPreservesQuotedNames(t *testing.T) {
	model := loadContent(t, "quoted-transition.sysml",
		"state def S { state 'waiting room'; state done; }\n")
	requireClean(t, model)
	result := applyOne(t, model, AddTransition(
		"S", "'to done'", "'waiting room'", "done", "", "", "", false,
	))
	if !strings.Contains(string(result.Content),
		"transition 'to done' first 'waiting room' then done;") {
		t.Fatalf("quoted transition missing:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "quoted-transition.sysml", string(result.Content)))
}

func TestAddEntryTransition(t *testing.T) {
	model := loadContent(t, "entry-transition.sysml",
		"state def S { state idle; }\n")
	requireClean(t, model)
	result := applyOne(t, model, AddTransition("S", "", "", "idle", "", "", "", true))
	if !strings.Contains(string(result.Content), "entry; then idle;") {
		t.Fatalf("entry transition missing:\n%s", result.Content)
	}
	requireClean(t, loadContent(t, "entry-transition.sysml", string(result.Content)))
}

func TestAddTransitionRefusesGrammarInjection(t *testing.T) {
	for _, test := range []struct {
		name    string
		trigger string
		guard   string
	}{
		{name: "semicolon member", trigger: "X; part p"},
		{name: "injected target", trigger: "X then idle"},
		{name: "closing body", guard: "true }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "transition-injection.sysml", transitionTestModel)
			addFailure(t, model, AddTransition(
				"P::S", "", "idle", "toasting",
				test.trigger, test.guard, "", false,
			), FailureInvalidValue)
		})
	}
}

func TestAddEntryTransitionRefusals(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
	}{
		{"name", AddTransition("S", "named", "", "idle", "", "", "", true)},
		{"source", AddTransition("S", "", "idle", "idle", "", "", "", true)},
		{"trigger", AddTransition("S", "", "", "idle", "CycleStart", "", "", true)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := loadContent(t, "entry-refusal.sysml", "state def S { state idle; }\n")
			addFailure(t, model, test.op, FailureIllegalKind)
		})
	}

	model := loadContent(t, "second-entry.sysml",
		"state def S { state idle; entry; then idle; }\n")
	addFailure(t, model, AddTransition("S", "", "", "idle", "", "", "", true), FailureIllegalKind)
}

func TestAddTransitionRequiresStateOwnerAndSysML(t *testing.T) {
	model := loadContent(t, "transition-owner.sysml", "part def P;\n")
	addFailure(t, model, AddTransition("P", "", "a", "b", "", "", "", false), FailureIllegalKind)

	kerml := loadContent(t, "transition-owner.kerml", "package P;\n")
	addFailure(t, kerml, AddTransition("", "", "a", "b", "", "", "", false), FailureIllegalKind)
}

func TestAddTransitionUnknownTargetIsRefusedByAnalysis(t *testing.T) {
	model := loadContent(t, "transition-unknown.sysml",
		"state def S { state idle; }\n")
	addFailure(t, model, AddTransition("S", "", "idle", "missing", "", "", "", false),
		FailureResultInvalid)
}

func TestAddTransitionRejectsAnExistingName(t *testing.T) {
	model := loadContent(t, "transition-name.sysml", transitionTestModel)
	addFailure(t, model, AddTransition(
		"P::S", "idle", "idle", "toasting", "", "", "", false,
	), FailureMemberNameTaken)
}
