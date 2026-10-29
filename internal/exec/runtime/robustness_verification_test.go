package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestRuntimeRobustnessVerification covers verification cases whose bodies, objectives or
// verdicts cannot be produced.
func TestRuntimeRobustnessVerification(t *testing.T) {
	t.Run("verification_body_that_cannot_run", testVerificationBodyThatCannotRun)
	t.Run("verification_body_step_that_fails", testVerificationBodyStepThatFails)
	t.Run("verification_subcase_that_cannot_run", testVerificationSubcaseThatCannotRun)
	t.Run("verification_of_a_symbol_that_is_not_a_case", testVerificationOfASymbolThatIsNotACase)
	t.Run("verification_with_an_argument_the_case_does_not_take", testVerificationWithAnArgumentTheCaseDoesNotTake)
	t.Run("verification_objective_subject_of_another_type", testVerificationObjectiveSubjectOfAnotherType)
	t.Run("verification_objective_subject_left_unbound", testVerificationObjectiveSubjectLeftUnbound)
	t.Run("verification_objective_subject_rebound", testVerificationObjectiveSubjectRebound)
	t.Run("verification_runs_once_per_repeated_object", testVerificationRunsOncePerRepeatedObject)
	t.Run("verification_runs_on_every_object_of_one_declaration", testVerificationRunsOnEveryObjectOfOneDeclaration)
}

// testVerificationBodyThatCannotRun: a body whose subject nothing binds is the
// case's error verdict carrying the typed error, not a failed call and not a
// panic.
func testVerificationBodyThatCannotRun(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::Unbound"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != verdictError {
		t.Fatalf("verdict = %q, want error", result.Verdict.Kind)
	}
	if !strings.Contains(result.Verdict.Detail, "sensor") {
		t.Errorf("verdict detail %q does not name the unbound subject", result.Verdict.Detail)
	}
}

// testVerificationBodyStepThatFails: a step that fails at run time — an action
// performed with an input nothing binds — ends the run in the same error verdict.
func testVerificationBodyStepThatFails(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::stepping"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != verdictError {
		t.Fatalf("verdict = %q (%s), want error", result.Verdict.Kind, result.Verdict.Detail)
	}
	if !strings.Contains(result.Verdict.Detail, "adder") {
		t.Errorf("verdict detail %q does not name the step that failed", result.Verdict.Detail)
	}
}

// testVerificationSubcaseThatCannotRun: a performed subcase whose input nothing
// binds ends the performing case's run, so its verdict is the error naming the
// subcase and the reason rather than a verdict of the body it never finished.
func testVerificationSubcaseThatCannotRun(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::plan"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != verdictError {
		t.Fatalf("verdict = %q (%s), want error", result.Verdict.Kind, result.Verdict.Detail)
	}
	for _, want := range []string{"sub", "threshold"} {
		if !strings.Contains(result.Verdict.Detail, want) {
			t.Errorf("verdict detail %q does not name %q", result.Verdict.Detail, want)
		}
	}
	if len(result.Subcases) != 0 {
		t.Errorf("subcase verdicts = %v, want none from a body that did not finish", result.Subcases)
	}
}

// testVerificationOfASymbolThatIsNotACase: asking a part for a verdict faults
// the request, which is an error rather than a verdict.
func testVerificationOfASymbolThatIsNotACase(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	_, err := ctx.RunVerification(lookupOne(t, idx, "test::Sensor"), AnalysisArgs{}, scope, nil)
	if !errors.Is(err, errNotAVerification) {
		t.Fatalf("error = %v, want errNotAVerification", err)
	}
}

// testVerificationWithAnArgumentTheCaseDoesNotTake: a named argument no
// parameter of the case declares faults the request too.
func testVerificationWithAnArgumentTheCaseDoesNotTake(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationRobustnessModel))
	scope := idx.DocumentRoot("<test>")
	args := AnalysisArgs{Named: map[string]Value{"nope": integerValue(1)}}
	_, err := ctx.RunVerification(lookupOne(t, idx, "test::stepping"), args, scope, nil)
	if !errors.Is(err, errUnknownParameter) {
		t.Fatalf("error = %v, want errUnknownParameter", err)
	}
}

// testVerificationObjectiveSubjectOfAnotherType: the case's subject, a Lander, is
// what the library binds the objective's subject to, so a requirement wanting a
// Rover leaves the objective undecided by a typed mismatch naming both types,
// while the body still answers.
func testVerificationObjectiveSubjectOfAnotherType(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationObjectiveModel))
	scope := idx.DocumentRoot("<test>")
	result, err := ctx.RunVerification(lookupOne(t, idx, "test::checkRover"), AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want the body's verdict", err)
	}
	if result.Verdict.Kind != VerdictPass {
		t.Fatalf("verdict = %q (%s), want pass", result.Verdict.Kind, result.Verdict.Detail)
	}
	if len(result.Run.Verdicts) != 1 || result.Run.Verdicts[0].Status != VerdictUndecided {
		t.Fatalf("verdicts = %+v, want the objective undecided", result.Run.Verdicts)
	}
	detail := result.Run.Verdicts[0].Detail
	for _, want := range []string{"subject rover", "case's subject", "VerificationCases::VerificationCase::obj", ErrTypeMismatch.Error(), "Lander", "is not a Rover"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail %q does not say %q", detail, want)
		}
	}
	if strings.Contains(detail, "Cases::Case::obj") || strings.Contains(detail, "result") {
		t.Errorf("detail %q reports the analysis case's default, not the verification's binding", detail)
	}
}

// testVerificationObjectiveSubjectLeftUnbound: a verification binding no subject
// is refused by the typed error naming the verification's subject, not the
// objective's, on the run surface as on the verdict surface.
func testVerificationObjectiveSubjectLeftUnbound(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, verificationObjectiveModel))
	scope := idx.DocumentRoot("<test>")
	sym := lookupOne(t, idx, "test::checkNothing")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{}, scope, nil)
	var unbound *subjectUnboundError
	if !errors.As(err, &unbound) {
		t.Fatalf("RunAnalysis error = %v, want an subjectUnboundError", err)
	}
	if unbound.Kind != "verification" || unbound.Element != "test::checkNothing" || unbound.Subject != "lander" {
		t.Errorf("error names %s %s: %s, want verification test::checkNothing: lander", unbound.Kind, unbound.Element, unbound.Subject)
	}
	result, err := ctx.RunVerification(sym, AnalysisArgs{}, scope, nil)
	if err != nil {
		t.Fatalf("RunVerification error = %v, want an error verdict", err)
	}
	if result.Verdict.Kind != verdictError || result.Verdict.Detail != unbound.Error() {
		t.Errorf("verdict = %q (%s), want error carrying %q", result.Verdict.Kind, result.Verdict.Detail, unbound.Error())
	}
}

// testVerificationObjectiveSubjectRebound: the library binds a verification
// objective's subject with `=`, so a usage binding it itself is refused by the
// constraint tier, whichever way it names the subject.
func testVerificationObjectiveSubjectRebound(t *testing.T) {
	const src = `
		package test {
			private import ScalarValues::*;
			part def Lander { attribute touchdownSpeed : Real; }
			part scout : Lander { attribute :>> touchdownSpeed = 1.2; }
			part other : Lander { attribute :>> touchdownSpeed = 1.3; }

			requirement def SoftLanding {
				subject lander : Lander;
				require constraint { lander.touchdownSpeed <= 1.5 }
			}

			verification def Named {
				subject lander : Lander;
				objective : SoftLanding { subject lander = other; }
				VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
			}
			verification def Anonymous {
				subject lander : Lander;
				objective : SoftLanding { subject = other; }
				VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
			}
			verification def Redefining {
				subject lander : Lander;
				objective : SoftLanding { subject :>> subj = other; }
				VerificationCases::PassIf(lander.touchdownSpeed <= 1.5)
			}
		}
	`
	file := parseAndBuild(t, src)
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	var refusals []string
	for _, d := range passes.Analyze("<test>", file, nil, idx) {
		if d.Code == "feature-value-overriding" && d.Severity == diag.SeverityError {
			refusals = append(refusals, d.Message)
		}
	}
	if len(refusals) != 3 {
		t.Fatalf("got %d refusals, want one per rebinding: %v", len(refusals), refusals)
	}
	for _, msg := range refusals {
		if !strings.Contains(msg, "cannot override the binding value of VerificationCases::VerificationCase::obj::subj") {
			t.Errorf("refusal %q does not name the library's binding", msg)
		}
	}
}

// testVerificationRunsOncePerRepeatedObject: four repeated wheels' bolts are four runs while a check
// resolves them to one subject; instantiating the car again leaves two live cars, so the case runs
// on the bolts of both — eight live bolts, eight runs — rather than on the latest car's alone.
func testVerificationRunsOncePerRepeatedObject(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::Real;
			requirement def Tightness { subject b : Bolt; require constraint { b.torque < 10.0 } }
			requirement tight : Tightness;
			part def Bolt {
				attribute torque : Real default = 99.0;
				constraint tightEnough { torque < 10.0 }
				verification torqueTest {
					objective { verify tight; }
					return verdict = VerificationCases::PassIf(tight(this));
				}
			}
			part def Wheel { part bolt : Bolt; }
			part def Car { part wheels : Wheel[4]; }
			part car : Car;
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	scope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(lookupOne(t, idx, "test::car")); err != nil {
		t.Fatalf("instantiate car: %v", err)
	}
	runsOnBolts := func(label string, want int) map[int64]bool {
		results := ctx.VerificationResultsOn([]*symbols.Scope{scope}, lookupOne(t, idx, "test::tight"), nil)
		subjects := map[int64]bool{}
		for _, r := range results {
			if r.Verdict.Subject == nil {
				t.Fatalf("%s: a run of %s names no object", label, r.Verdict.Case)
			}
			subjects[r.Verdict.Subject.ID] = true
			if r.Verdict.Kind != VerdictFail {
				t.Errorf("%s: verdict on object #%d = %s (%s), want fail", label, r.Verdict.Subject.ID, r.Verdict.Kind, r.Verdict.Detail)
			}
		}
		if len(results) != want || len(subjects) != want {
			t.Fatalf("%s: %d run(s) on %d object(s), want one on each of the %d bolts", label, len(results), len(subjects), want)
		}
		return subjects
	}
	first := runsOnBolts("one car", 4)
	tightEnough := lookupOne(t, idx, "test::Bolt::tightEnough")
	if _, err := ctx.EvaluateConstraint(tightEnough, tightEnough.OwnerScope); !errors.Is(err, ErrViolated) {
		t.Errorf("EvaluateConstraint on the repeated bolts: %v, want the one subject's violation", err)
	}
	if _, err := ctx.Instantiate(lookupOne(t, idx, "test::car")); err != nil {
		t.Fatalf("instantiate car again: %v", err)
	}
	both := runsOnBolts("two cars", 8)
	for id := range first {
		if !both[id] {
			t.Errorf("object #%d of the first car is no longer run on; it is a live bolt the case is a feature of", id)
		}
	}
}

// testVerificationRunsOnEveryObjectOfOneDeclaration: a declaration instantiated twice leaves two
// live objects of its definition, so a type-owned case verifying a requirement runs on each, while a
// check of a type-owned constraint naming no object is a question between the two — answered once
// one of them is named — rather than silently about the latest.
func testVerificationRunsOnEveryObjectOfOneDeclaration(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::Real;
			requirement def Tightness { subject b : Bolt; require constraint { b.torque < 10.0 } }
			requirement tight : Tightness;
			part def Bolt {
				attribute torque : Real default = 99.0;
				constraint tightEnough { torque < 10.0 }
				verification torqueTest {
					objective { verify tight; }
					return verdict = VerificationCases::PassIf(tight(this));
				}
				satisfy tight;
			}
			part b : Bolt;
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	scope := idx.DocumentRoot("<test>")
	bolt := lookupOne(t, idx, "test::b")
	first, err := ctx.Instantiate(bolt)
	if err != nil {
		t.Fatalf("instantiate b: %v", err)
	}
	second, err := ctx.Instantiate(bolt)
	if err != nil {
		t.Fatalf("instantiate b again: %v", err)
	}
	results := ctx.VerificationResultsOn([]*symbols.Scope{scope}, lookupOne(t, idx, "test::tight"), nil)
	subjects := map[int64]bool{}
	for _, r := range results {
		if r.Verdict.Subject == nil {
			t.Fatalf("a run of %s names no object", r.Verdict.Case)
		}
		subjects[r.Verdict.Subject.ID] = true
		if r.Verdict.Kind != VerdictFail {
			t.Errorf("verdict on object #%d = %s (%s), want fail", r.Verdict.Subject.ID, r.Verdict.Kind, r.Verdict.Detail)
		}
	}
	if len(results) != 2 || !subjects[first.ID] || !subjects[second.ID] {
		t.Fatalf("%d run(s) on objects %v, want one on each of #%d and #%d", len(results), subjects, first.ID, second.ID)
	}
	tightEnough := lookupOne(t, idx, "test::Bolt::tightEnough")
	_, err = ctx.EvaluateConstraint(tightEnough, tightEnough.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("EvaluateConstraint over two bolts of one declaration: %v, want ErrAmbiguousSubject", err)
	}
	for _, want := range []string{fmt.Sprintf("Bolt #%d (displaced from b)", first.ID), fmt.Sprintf("Bolt #%d (b)", second.ID), "check it on one of them"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not say %q", err, want)
		}
	}
	for _, obj := range []*Instance{first, second} {
		if _, err := ctx.EvaluateConstraintOn(tightEnough, tightEnough.OwnerScope, obj); !errors.Is(err, ErrViolated) {
			t.Errorf("EvaluateConstraintOn object #%d: %v, want the named object's violation", obj.ID, err)
		}
	}
}
