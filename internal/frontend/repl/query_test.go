package repl

import (
	"strings"
	"testing"
)

// checkFixture is a model whose constraints and requirements decide both ways.
const checkFixture = `package Rover {
    part def Battery {
        attribute capacity;
        attribute charge;
    }

    part pack : Battery {
        attribute :>> capacity = 100.0;
        attribute :>> charge = 80.0;
        constraint notOvercharged { charge <= capacity }
        constraint nearlyEmpty { charge <= 5.0 }
    }

    constraint MassBudget { 180.0 <= 200.0 }
    constraint TooHeavy { 210.0 <= 200.0 }

    requirement PowerMargin {
        assume constraint { 600.0 > 0.0 }
        require constraint { 600.0 >= 450.0 }
    }
    requirement PowerShortfall {
        require constraint { 300.0 >= 450.0 }
    }
}
`

func loadSource(t *testing.T, src string) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(src); len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// rejectVerdict checks that a verdict's report does not say something, which is
// how wording that contradicts its status is caught.
func rejectVerdict(t *testing.T, v Verdict, substrings ...string) {
	t.Helper()
	joined := strings.Join(v.Lines, "\n")
	for _, unwanted := range substrings {
		if strings.Contains(joined, unwanted) {
			t.Errorf("report of %q says %q, got:\n%s", v.Subject, unwanted, joined)
		}
	}
}

// wantVerdict checks a verdict's status and that its report says so.
func wantVerdict(t *testing.T, v Verdict, want VerdictStatus, substrings ...string) {
	t.Helper()
	if v.Status != want {
		t.Errorf("status of %q = %v, want %v (lines: %v)", v.Subject, v.Status, want, v.Lines)
	}
	if v.Holds() != (want == VerdictHolds) {
		t.Errorf("Holds() of %q = %v, want %v", v.Subject, v.Holds(), want == VerdictHolds)
	}
	joined := strings.Join(v.Lines, "\n")
	for _, want := range substrings {
		if !strings.Contains(joined, want) {
			t.Errorf("report of %q missing %q, got:\n%s", v.Subject, want, joined)
		}
	}
}

// TestCheckConstraintStatus checks that the status a caller decides on agrees
// with the report: a condition the model answers, either way, is not the same as
// a check that could not be made.
func TestCheckConstraintStatus(t *testing.T) {
	s := loadSource(t, checkFixture)

	wantVerdict(t, s.CheckConstraint("Rover::MassBudget"), VerdictHolds,
		"✓ Constraint Rover::MassBudget passed")
	wantVerdict(t, s.CheckConstraint("Rover::TooHeavy"), VerdictFails,
		"✗ Constraint Rover::TooHeavy failed",
		"Assertion evaluated to false: 210.0 <= 200.0")
	wantVerdict(t, s.CheckConstraint("nosuch"), VerdictUnresolved,
		"unresolved reference: nosuch")
	wantVerdict(t, NewSession().CheckConstraint("Rover::MassBudget"), VerdictUnresolved,
		"no declarations loaded")
}

// TestCheckRequirementStatus is the requirement half of the same contract.
func TestCheckRequirementStatus(t *testing.T) {
	s := loadSource(t, checkFixture)

	wantVerdict(t, s.CheckRequirement("Rover::PowerMargin"), VerdictHolds,
		"✓ Requirement Rover::PowerMargin satisfied")
	wantVerdict(t, s.CheckRequirement("Rover::PowerShortfall"), VerdictFails,
		"✗ Requirement Rover::PowerShortfall failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckRequirement("nosuch"), VerdictUnresolved,
		"unresolved reference: nosuch")
}

// unevaluableFixture states a condition over a subject nothing binds, so its
// evaluation cannot be carried out at all.
const unevaluableFixture = `package Landing {
    part def Lander { attribute verticalSpeed; }
    requirement def TouchdownRequirement {
        subject lander : Lander;
        require constraint { lander.verticalSpeed <= 1.5 }
    }
    requirement touchdown : TouchdownRequirement;
}
`

// TestConditionThatCouldNotBeEvaluated checks that an evaluation which could not
// be carried out is not counted against the model: a requirement over an unbound
// subject decided nothing, and its report says so rather than reading as a
// failure a reader would draw the opposite conclusion from.
func TestConditionThatCouldNotBeEvaluated(t *testing.T) {
	s := loadSource(t, unevaluableFixture)

	v := s.CheckRequirement("Landing::touchdown")
	wantVerdict(t, v, VerdictUnresolved,
		"? Requirement Landing::touchdown could not be evaluated",
		"lander subject is unbound",
		"satisfy touchdown by <element>")
	rejectVerdict(t, v, "Requirement Landing::touchdown failed", "✗")
}

// TestSatisfactionThatCouldNotBeEvaluated checks the third verdict surface: an
// assertion whose evaluation failed reads as undecided, as its exit status is.
func TestSatisfactionThatCouldNotBeEvaluated(t *testing.T) {
	s := loadSource(t, `package Landing {
    part def Lander { attribute verticalSpeed; }
    requirement def TouchdownRequirement {
        subject lander : Lander;
        require constraint { lander.verticalSpeed <= 1.5 }
    }
    requirement touchdown : TouchdownRequirement;
    part lander1 : Lander;
    part context { assert satisfy touchdown by lander1; }
}
`)
	verdicts := s.CheckSatisfy("Landing::context")
	if len(verdicts) != 1 {
		t.Fatalf("got %d verdicts, want 1: %v", len(verdicts), verdicts)
	}
	wantVerdict(t, verdicts[0], VerdictUnresolved, "could not be evaluated")
	rejectVerdict(t, verdicts[0], "fails", "✗")
}

// TestCheckOnInstantiatedObject checks that a check made after InstantiateNamed
// is about that object, so its verdict is about concrete feature values.
func TestCheckOnInstantiatedObject(t *testing.T) {
	s := loadSource(t, checkFixture)

	lines, err := s.InstantiateNamed("Rover::pack")
	if err != nil {
		t.Fatalf("InstantiateNamed: %v", err)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "✓ Created instance of Rover::pack") {
		t.Errorf("InstantiateNamed reported:\n%s", got)
	}

	wantVerdict(t, s.CheckConstraint("Rover::pack::notOvercharged"), VerdictHolds,
		"✓ Constraint Rover::pack::notOvercharged passed (on Rover::pack ID: 1)")
	wantVerdict(t, s.CheckConstraint("Rover::pack::nearlyEmpty"), VerdictFails,
		"✗ Constraint Rover::pack::nearlyEmpty failed (on Rover::pack ID: 1)")
}

// inheritedConditionFixture declares its conditions on the definition, which two
// usages carry with values of their own.
const inheritedConditionFixture = `package P {
    part def Sensor {
        attribute reading default = 0.0;
        constraint inRange { reading <= 100.0 }
        requirement ok { require constraint { reading <= 100.0 } }
    }
    part hot : Sensor { attribute :>> reading = 140.0; }
    part cold : Sensor { attribute :>> reading = 20.0; }
}
`

// TestCheckOfInheritedConditionIsAboutTheObject: a condition a definition
// declares is checked against the object of a usage carrying it, since a verdict
// about the definition's defaults would pass while that object violates it.
func TestCheckOfInheritedConditionIsAboutTheObject(t *testing.T) {
	s := loadSource(t, inheritedConditionFixture)
	if _, err := s.InstantiateNamed("P::hot"); err != nil {
		t.Fatalf("InstantiateNamed: %v", err)
	}

	wantVerdict(t, s.CheckConstraint("P::Sensor::inRange"), VerdictFails,
		"✗ Constraint P::Sensor::inRange failed (on P::hot ID: 1)")
	wantVerdict(t, s.CheckRequirement("P::Sensor::ok"), VerdictFails,
		"✗ Requirement P::Sensor::ok failed (on P::hot ID: 1)")
}

// A session holding no object of the declaring type still answers about declared
// defaults, and says nothing about an object.
func TestCheckWithoutAnObjectUsesDeclaredDefaults(t *testing.T) {
	s := loadSource(t, inheritedConditionFixture)
	v := s.CheckConstraint("P::Sensor::inRange")
	wantVerdict(t, v, VerdictHolds, "✓ Constraint P::Sensor::inRange passed")
	if got := strings.Join(v.Lines, "\n"); strings.Contains(got, "(on ") {
		t.Errorf("verdict claims an object:\n%s", got)
	}
}

// Two objects carry the same inherited condition, so which one a verdict would
// be about is a question: answering about either would report the other's
// verdict half the time.
func TestCheckOfConditionCarriedBySeveralObjectsIsUnresolved(t *testing.T) {
	s := loadSource(t, inheritedConditionFixture)
	for _, name := range []string{"P::hot", "P::cold"} {
		if _, err := s.InstantiateNamed(name); err != nil {
			t.Fatalf("InstantiateNamed(%s): %v", name, err)
		}
	}

	wantVerdict(t, s.CheckConstraint("P::Sensor::inRange"), VerdictUnresolved,
		"carried by more than one object", "P::cold, P::hot")
	wantVerdict(t, s.CheckRequirement("P::Sensor::ok"), VerdictUnresolved,
		"carried by more than one object")
}

// TestEvalOfInheritedFeatureIsAboutTheObject: %eval must choose the subject a
// check does, or the same session answers a condition about the object and the
// feature it reads about the definition's default.
func TestEvalOfInheritedFeatureIsAboutTheObject(t *testing.T) {
	s := loadSource(t, inheritedConditionFixture)
	if _, err := s.InstantiateNamed("P::hot"); err != nil {
		t.Fatalf("InstantiateNamed: %v", err)
	}

	out, err := s.EvalExpr("P::Sensor::reading")
	if err != nil {
		t.Fatalf("EvalExpr: %v", err)
	}
	got := strings.Join(out, "\n")
	for _, want := range []string{"(on P::hot ID: 1)", "= 140.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("%%eval missing %q, got:\n%s", want, got)
		}
	}
}

// Several objects carrying the feature is the same question it is for a check,
// so %eval names them rather than answering about one of them.
func TestEvalOfFeatureCarriedBySeveralObjectsIsRefused(t *testing.T) {
	s := loadSource(t, inheritedConditionFixture)
	for _, name := range []string{"P::hot", "P::cold"} {
		if _, err := s.InstantiateNamed(name); err != nil {
			t.Fatalf("InstantiateNamed(%s): %v", name, err)
		}
	}

	_, err := s.EvalExpr("P::Sensor::reading")
	if err == nil {
		t.Fatal("eval chose between two objects carrying the feature")
	}
	for _, want := range []string{"carried by more than one object", "P::cold, P::hot"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

// Without an object the declared default is still an answer, and it claims no
// object.
func TestEvalWithoutAnObjectUsesDeclaredDefaults(t *testing.T) {
	s := loadSource(t, inheritedConditionFixture)
	out, err := s.EvalExpr("P::Sensor::reading")
	if err != nil {
		t.Fatalf("EvalExpr: %v", err)
	}
	got := strings.Join(out, "\n")
	if !strings.Contains(got, "= 0.0") || strings.Contains(got, "(on ") {
		t.Errorf("%%eval without an object reported:\n%s", got)
	}
}

// A name of another kind is answered as such however many objects of its type
// the session holds, rather than as a question about which one it is about.
func TestCheckOfWrongKindIsAnsweredBeforeItsSubject(t *testing.T) {
	s := loadSource(t, inheritedConditionFixture)
	for _, name := range []string{"P::hot", "P::cold"} {
		if _, err := s.InstantiateNamed(name); err != nil {
			t.Fatalf("InstantiateNamed(%s): %v", name, err)
		}
	}

	wantVerdict(t, s.CheckConstraint("P::Sensor::reading"), VerdictUnresolved,
		"not a constraint")
	wantVerdict(t, s.CheckRequirement("P::Sensor::reading"), VerdictUnresolved,
		"not a requirement")
}

// TestInstantiateNamedRejectsUnknownName checks that a name the session cannot
// resolve is an error a caller can fail on, not a silent no-op.
func TestInstantiateNamedRejectsUnknownName(t *testing.T) {
	s := loadSource(t, checkFixture)
	if _, err := s.InstantiateNamed("nosuch"); err == nil {
		t.Fatal("InstantiateNamed accepted an unknown name")
	}
	if _, err := NewSession().InstantiateNamed("Rover::pack"); err == nil {
		t.Fatal("InstantiateNamed accepted a name with no declarations loaded")
	}
}

// TestCheckSatisfyStatuses checks the verdicts of the satisfaction assertions a
// model states, reached both through the whole session and through the element
// stating them.
func TestCheckSatisfyStatuses(t *testing.T) {
	s := loadFixture(t, "testdata/satisfy_landing.sysml")

	all := s.CheckSatisfy("")
	if len(all) != 3 {
		t.Fatalf("got %d verdicts, want 3: %v", len(all), all)
	}
	wantVerdict(t, all[0], VerdictHolds, "✓ satisfy touchdown by slowLander holds")
	wantVerdict(t, all[1], VerdictFails, "✗ satisfy touchdown by fastLander fails",
		"Required condition evaluated to false: lander.verticalSpeed <= maxVerticalSpeed")
	wantVerdict(t, all[2], VerdictHolds, "✓ not satisfy touchdown by fastLander holds")

	stated := s.CheckSatisfy("Landing::analysisContext")
	if len(stated) != 3 {
		t.Fatalf("got %d verdicts for the stating element, want 3: %v", len(stated), stated)
	}

	// A requirement is not itself an assertion and states none, so nothing was
	// checked — which says nothing about the model either way.
	none := s.CheckSatisfy("Landing::touchdown")
	if len(none) != 1 {
		t.Fatalf("got %d verdicts, want 1: %v", len(none), none)
	}
	wantVerdict(t, none[0], VerdictUnresolved, "no satisfaction assertion in Landing::touchdown")

	unknown := s.CheckSatisfy("nosuch")
	wantVerdict(t, unknown[0], VerdictUnresolved, "unresolved reference: nosuch")

	empty := NewSession().CheckSatisfy("")
	wantVerdict(t, empty[0], VerdictUnresolved, "no declarations loaded")
}

// TestWorstStatus checks how a set of verdicts is judged: a check that was never
// made outranks a failure, since it makes no claim about the model.
func TestWorstStatus(t *testing.T) {
	tests := []struct {
		name     string
		statuses []VerdictStatus
		want     VerdictStatus
	}{
		{"empty holds", nil, VerdictHolds},
		{"all hold", []VerdictStatus{VerdictHolds, VerdictHolds}, VerdictHolds},
		{"one fails", []VerdictStatus{VerdictHolds, VerdictFails}, VerdictFails},
		{"unresolved outranks a failure", []VerdictStatus{VerdictFails, VerdictUnresolved}, VerdictUnresolved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verdicts := make([]Verdict, 0, len(tt.statuses))
			for _, status := range tt.statuses {
				verdicts = append(verdicts, Verdict{Status: status})
			}
			if got := WorstStatus(verdicts); got != tt.want {
				t.Errorf("WorstStatus = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckReportsTrace checks that a check made outside the prompt still carries
// the execution trace `%trace on` asks for, rather than leaving it buffered for
// whatever command runs next.
func TestCheckReportsTrace(t *testing.T) {
	s := loadSource(t, checkFixture)
	s.SetTracing(true)

	v := s.CheckConstraint("Rover::MassBudget")
	if got := strings.Join(v.Lines, "\n"); !strings.Contains(got, "[trace] ") {
		t.Errorf("verdict carries no trace:\n%s", got)
	}
	if again := run(t, s, "%instances"); strings.Contains(again, "[trace] ") {
		t.Errorf("trace of the check was reported again later:\n%s", again)
	}
}

// TestMetaCommandsRenderTheSameVerdicts checks that the prompt reports what the
// verdict API decides, so there is one evaluation behind both.
func TestMetaCommandsRenderTheSameVerdicts(t *testing.T) {
	s := loadSource(t, checkFixture)
	for _, tt := range []struct{ command, name string }{
		{"%constraint", "Rover::TooHeavy"},
		{"%requirement", "Rover::PowerMargin"},
	} {
		var want Verdict
		if tt.command == "%constraint" {
			want = s.CheckConstraint(tt.name)
		} else {
			want = s.CheckRequirement(tt.name)
		}
		got := run(t, s, tt.command+" "+tt.name)
		if got != strings.Join(want.Lines, "\n") {
			t.Errorf("%s %s printed\n%s\nwant\n%s", tt.command, tt.name, got, strings.Join(want.Lines, "\n"))
		}
	}
}
