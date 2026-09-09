package runtime

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// analysisModel is the model the analysis robustness cases run in.
const analysisModel = `
	package test {
		private import ScalarValues::*;
		part def Ship { attribute cost : Real = 5.0; attribute other : Real = 7.0; }
		calc def Sum { in a : Real; in b : Real; return : Real = a + b; }
		calc def Fail { in a : Real; return : Real = a / 0.0; }
		analysis def CostAnalysis {
			subject s : Ship;
			out total : Real = Sum(s.cost, s.other);
		}
		analysis def Scaled {
			subject s : Ship;
			in factor : Real;
			out total : Real = s.cost * factor;
		}
		analysis def FailingStep {
			subject s : Ship;
			action bad { out v : Real = Fail(s.cost); }
			return : Real = bad.v;
		}
		analysis def Recursive {
			in n : Real;
			return : Real = Recursive(n);
		}
		analysis def RecursiveStep {
			subject s : Ship;
			analysis again : RecursiveStep { subject s = s; }
			return r : Real = again.r;
		}
		calc def RecursiveUsage {
			in n : Real;
			calc g : RecursiveUsage { in n = n; }
			return : Real = g;
		}
		analysis def Looping {
			subject s : Ship;
			action a { out v : Real = s.cost; }
			action b { out w : Real = a.v; }
			succession first a then b;
			succession first b then a;
			return : Real = b.w;
		}
		analysis def Deadlocked {
			subject s : Ship;
			first a;
			action a { out v : Real = s.cost; }
			action stranded;
			action b { out w : Real = a.v; }
			join sync;
			succession first a then sync;
			succession first stranded then sync;
			succession first sync then b;
			return : Real = b.w;
		}
		analysis def Unstarted {
			subject s : Ship;
			action a { out v : Real = s.cost; }
			action b { out w : Real = s.other; }
			action c { out x : Real = a.v + b.w; }
			succession first a then c;
			succession first b then c;
			return : Real = c.x;
		}
		part ship : Ship;
		analysis selfRead { out x : Real = selfRead.x + 1.0; }
		analysis pair { out p : Real = q + 1.0; out q : Real = p + 1.0; }
		analysis deferred { out d : Real; }
	}
`

// analysisRuntime builds the analysis robustness model and resolves a case in it.
func analysisRuntime(t *testing.T, fqn string) (*Context, *symbols.Index, *symbols.Symbol) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, analysisModel))
	return ctx, idx, oneSymbol(t, idx, fqn)
}

func TestAnalysisRobustness(t *testing.T) {
	t.Run("unbound_subject", testAnalysisUnboundSubject)
	t.Run("subject_of_the_wrong_type", testAnalysisSubjectOfTheWrongType)
	t.Run("missing_in_parameter", testAnalysisMissingInParameter)
	t.Run("too_many_arguments", testAnalysisTooManyArguments)
	t.Run("unknown_named_argument", testAnalysisUnknownNamedArgument)
	t.Run("failing_step", testAnalysisFailingStep)
	t.Run("self_recursion", testAnalysisSelfRecursion)
	t.Run("recursion_through_a_step", testAnalysisRecursionThroughAStep)
	t.Run("recursion_through_a_calc_usage", testCalcRecursionThroughAUsage)
	t.Run("usage_reading_its_own_output", testAnalysisUsageReadingItsOwnOutput)
	t.Run("outputs_reading_each_other", testAnalysisOutputsReadingEachOther)
	t.Run("output_never_bound", testAnalysisOutputNeverBound)
	t.Run("step_budget", testAnalysisStepBudget)
	t.Run("cyclic_successions", testAnalysisCyclicSuccessions)
	t.Run("deadlocked_body", testAnalysisDeadlockedBody)
	t.Run("flow_with_no_start", testAnalysisFlowWithNoStart)
	t.Run("not_an_analysis", testAnalysisNotAnAnalysis)
}

// testAnalysisUnboundSubject: a definition run without a subject reports the
// unbound subject by name, like a requirement checked without one.
func testAnalysisUnboundSubject(t *testing.T) {
	ctx, _, sym := analysisRuntime(t, "test::CostAnalysis")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{}, nil, nil)
	var unbound *UnboundSubjectError
	if !errors.As(err, &unbound) || !errors.Is(err, ErrUnboundSubject) {
		t.Fatalf("error = %v, want *UnboundSubjectError", err)
	}
	if unbound.Subject != "s" || unbound.Element != "test::CostAnalysis" {
		t.Errorf("unbound subject %q of %q, want s of test::CostAnalysis", unbound.Subject, unbound.Element)
	}
}

// testAnalysisSubjectOfTheWrongType: a supplied subject must conform to the
// subject parameter's type.
func testAnalysisSubjectOfTheWrongType(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::CostAnalysis")
	other, err := ctx.Instantiate(oneSymbol(t, idx, "test::Sum"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ctx.RunAnalysis(sym, AnalysisArgs{Subject: other}, nil, nil)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "test::CostAnalysis") {
		t.Errorf("error %q does not name the case", err)
	}
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	if _, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil); err != nil {
		t.Errorf("a conforming subject: %v", err)
	}
}

// testAnalysisMissingInParameter: an input parameter no argument and no default
// binds is reported by name.
func testAnalysisMissingInParameter(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::Scaled")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil)
	if !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("error = %v, want ErrUnboundParameter", err)
	}
	if !strings.Contains(err.Error(), "factor") || !strings.Contains(err.Error(), "test::Scaled") {
		t.Errorf("error %q names neither the parameter nor the case", err)
	}
}

// testAnalysisTooManyArguments: positional arguments beyond the parameters are
// refused before anything runs, so no partial run comes with the error.
func testAnalysisTooManyArguments(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::Scaled")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	args := AnalysisArgs{Subject: ship, Positional: []Value{constReal(2), constReal(3)}}
	result, err := ctx.RunAnalysis(sym, args, nil, nil)
	if !errors.Is(err, ErrCalcArity) {
		t.Fatalf("error = %v, want ErrCalcArity", err)
	}
	assertNoPartialRun(t, result)
}

// testAnalysisUnknownNamedArgument: a named argument must name an input
// parameter; one that does not is refused before anything runs.
func testAnalysisUnknownNamedArgument(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::Scaled")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	args := AnalysisArgs{Subject: ship, Named: map[string]Value{"nope": constReal(2)}}
	result, err := ctx.RunAnalysis(sym, args, nil, nil)
	if !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("error = %v, want ErrUnknownParameter", err)
	}
	assertNoPartialRun(t, result)
}

// assertNoPartialRun checks a refused request left nothing of a run behind.
func assertNoPartialRun(t *testing.T, result AnalysisResult) {
	t.Helper()
	if result.Case != "" || result.Subject != nil || len(result.Outputs) != 0 ||
		len(result.Verdicts) != 0 || len(result.Evaluations) != 0 {
		t.Errorf("result = %+v, want none for a request refused before running", result)
	}
}

// testAnalysisFailingStep: a step whose computation fails fails the case,
// naming the step and the case, not the bare arithmetic error.
func testAnalysisFailingStep(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::FailingStep")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}
	if !strings.Contains(err.Error(), "bad") || !strings.Contains(err.Error(), "test::FailingStep") {
		t.Errorf("error %q names neither the step nor the case", err)
	}
}

// testAnalysisSelfRecursion: a case invoking itself hits the recursion limit,
// as a calc def calling itself does.
func testAnalysisSelfRecursion(t *testing.T) {
	ctx, _, sym := analysisRuntime(t, "test::Recursive")
	// The default step budget, so the nesting reaches the depth bound first.
	ctx.maxSteps = DefaultMaxSteps
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Positional: []Value{constReal(1)}}, nil, nil)
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Fatalf("error = %v, want ErrCalcRecursionLimit", err)
	}
	if !strings.Contains(err.Error(), "test::Recursive") {
		t.Errorf("error %q does not name the case", err)
	}
}

// recursionFrames is the collapsed frame count a recursion-limit message reports.
var recursionFrames = regexp.MustCompile(`… (\d+) frames:`)

// assertRecursionCollapsed checks that a recursion-limit error reads as one
// line: bounded, naming the recurring case or calc a handful of times at most,
// reporting how many frames collapsed, and still the typed error.
func assertRecursionCollapsed(t *testing.T, err error, name string) {
	t.Helper()
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Fatalf("error = %v, want ErrCalcRecursionLimit", err)
	}
	msg := err.Error()
	if len(msg) > 1024 {
		t.Errorf("message is %d bytes, want one line: %.200s…", len(msg), msg)
	}
	if n := strings.Count(msg, name); n == 0 || n > 4 {
		t.Errorf("message names %s %d times, want 1–4: %q", name, n, msg)
	}
	if m := recursionFrames.FindStringSubmatch(msg); m == nil || m[1] == "0" || m[1] == "1" {
		t.Errorf("message %q does not report the collapsed frames", msg)
	}
	if !strings.Contains(msg, "OPENSYSML_MAX_CALC_DEPTH") {
		t.Errorf("message %q does not say how to raise the limit", msg)
	}
}

// testAnalysisRecursionThroughAStep: a case performing itself as a nested
// analysis step hits the recursion limit with a message that collapses the
// repeated frames rather than nesting `node again:` once per level.
func testAnalysisRecursionThroughAStep(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::RecursiveStep")
	ctx.maxSteps = DefaultMaxSteps
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil)
	assertRecursionCollapsed(t, err, "test::RecursiveStep::again")
	if !strings.Contains(err.Error(), "test::RecursiveStep") {
		t.Errorf("error %q does not name the case", err)
	}
}

// testCalcRecursionThroughAUsage: a calc def recursing through its own calc
// usage member collapses the same way a calc def calling itself does.
func testCalcRecursionThroughAUsage(t *testing.T) {
	ctx, _, sym := analysisRuntime(t, "test::RecursiveUsage")
	ctx.maxSteps = DefaultMaxSteps
	_, err := ctx.InvokeCalc(sym, []Value{constReal(1)}, nil)
	assertRecursionCollapsed(t, err, "test::RecursiveUsage::g")
}

// testAnalysisUsageReadingItsOwnOutput: a usage whose output reads itself is a
// cycle, reported as a calc usage's is, without recursing until the limit.
func testAnalysisUsageReadingItsOwnOutput(t *testing.T) {
	ctx, _, sym := analysisRuntime(t, "test::selfRead")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{}, nil, nil)
	if !errors.Is(err, ErrCyclicOutput) {
		t.Fatalf("error = %v, want ErrCyclicOutput", err)
	}
	if !strings.Contains(err.Error(), "test::selfRead") {
		t.Errorf("error %q does not name the case", err)
	}
}

// testAnalysisOutputsReadingEachOther: two outputs defined in terms of each
// other are a cycle inside the case.
func testAnalysisOutputsReadingEachOther(t *testing.T) {
	ctx, _, sym := analysisRuntime(t, "test::pair")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{}, nil, nil)
	if !errors.Is(err, ErrCyclicOutput) {
		t.Fatalf("error = %v, want ErrCyclicOutput", err)
	}
	if !strings.Contains(err.Error(), "test::pair") {
		t.Errorf("error %q does not name the case", err)
	}
}

// testAnalysisOutputNeverBound: a case binding none of its outputs computes
// nothing, which is reported naming the outputs left unbound.
func testAnalysisOutputNeverBound(t *testing.T) {
	ctx, _, sym := analysisRuntime(t, "test::deferred")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{}, nil, nil)
	if !errors.Is(err, ErrNoResultExpression) {
		t.Fatalf("error = %v, want ErrNoResultExpression", err)
	}
	if !strings.Contains(err.Error(), "d") {
		t.Errorf("error %q does not name the output", err)
	}
}

// testAnalysisStepBudget: an exhausted step budget stops the run with the
// typed limit error rather than hanging.
func testAnalysisStepBudget(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::CostAnalysis")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	ctx.maxSteps = 2
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrStepLimitExceeded", err)
	}
}

// testAnalysisCyclicSuccessions: steps sequenced in a cycle leave no step to
// start at, which the flow reports as invalid rather than running forever.
func testAnalysisCyclicSuccessions(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::Looping")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil)
	if err == nil {
		t.Fatal("a cyclic body ran to completion")
	}
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
	for _, want := range []string{"test::Looping", "cycle"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// testAnalysisFlowWithNoStart: two steps no succession leads to leave the start
// unstated; the flow is reported invalid, naming both.
func testAnalysisFlowWithNoStart(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::Unstarted")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil)
	if err == nil {
		t.Fatal("a flow with no start ran to completion")
	}
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Fatalf("error = %v, want ErrInvalidActionFlow", err)
	}
	for _, want := range []string{"test::Unstarted", `"a"`, `"b"`, "'first'"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

// testAnalysisDeadlockedBody: a join waiting for a token that never comes
// deadlocks the body, which is reported as such.
func testAnalysisDeadlockedBody(t *testing.T) {
	ctx, idx, sym := analysisRuntime(t, "test::Deadlocked")
	ship := instanceOfUsage(t, ctx, idx, "test::ship")
	_, err := ctx.RunAnalysis(sym, AnalysisArgs{Subject: ship}, nil, nil)
	if err == nil {
		t.Fatal("a deadlocked body ran to completion")
	}
	if !errors.Is(err, ErrActionDeadlock) {
		t.Fatalf("error = %v, want ErrActionDeadlock", err)
	}
	if !strings.Contains(err.Error(), "test::Deadlocked") {
		t.Errorf("error %q does not name the case", err)
	}
}

// testAnalysisNotAnAnalysis: only analysis cases run through RunAnalysis; a
// calc, part or verification case is refused by kind.
func testAnalysisNotAnAnalysis(t *testing.T) {
	ctx, idx, _ := analysisRuntime(t, "test::CostAnalysis")
	for _, fqn := range []string{"test::Sum", "test::ship", "test::Ship"} {
		_, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, nil)
		if !errors.Is(err, ErrNotAnAnalysis) {
			t.Errorf("%s: error = %v, want ErrNotAnAnalysis", fqn, err)
		}
	}
}
