package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// objectiveBindingModel states one requirement def checked as the objective of
// analysis cases that bind its subject each way SysML v2 allows, or not at all.
const objectiveBindingModel = `
	package test {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		part def Ship { attribute hullMass : Real; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part heavy : Ship { attribute :>> hullMass = 5000.0; }

		requirement def MassLimit {
			subject s : Ship;
			attribute limit : Real = 2000.0;
			require constraint { s.hullMass < limit }
		}

		analysis def Keyword { subject ship : Ship; objective : MassLimit { subject = ship; } return r : Real = ship.hullMass; }
		analysis def Named { subject ship : Ship; objective : MassLimit { subject s = ship; } return r : Real = ship.hullMass; }
		analysis def Redefined { subject ship : Ship; objective : MassLimit { subject :>> s = ship; } return r : Real = ship.hullMass; }
		analysis def Unbound { subject ship : Ship; objective : MassLimit; return r : Real = ship.hullMass; }
		analysis def Defaulted { subject ship : Ship; objective : MassLimit; return picked : Ship = ship; }
		analysis def Resultless { subject ship : Ship; objective : MassLimit; out m : Real = ship.hullMass; }
		analysis def Inline { subject ship : Ship; objective { require constraint { ship.hullMass < 2000.0 } } return r : Real = ship.hullMass; }
		analysis def Failing { subject ship : Ship; objective : MassLimit { subject = ship.hullMass / 0.0; } return r : Real = ship.hullMass; }

		analysis keyword : Keyword { subject ship = test::ship; }
		analysis keywordHeavy : Keyword { subject ship = heavy; }
		analysis named : Named { subject ship = test::ship; }
		analysis redefined : Redefined { subject ship = test::ship; }
		analysis unbound : Unbound { subject ship = test::ship; }
		analysis defaulted : Defaulted { subject ship = test::ship; }
		analysis defaultedHeavy : Defaulted { subject ship = heavy; }
		analysis resultless : Resultless { subject ship = test::ship; }
		analysis inline : Inline { subject ship = test::ship; }
		analysis inlineHeavy : Inline { subject ship = heavy; }
		analysis failing : Failing { subject ship = test::ship; }

		analysis usageKeyword { subject ship = test::ship; objective : MassLimit { subject = ship; } return r : Real = ship.hullMass; }
		analysis usageRebound : Unbound { subject ship = heavy; objective :>> obj { subject = ship; } }

		analysis def Pick { subject ship : Ship; return picked : Ship = ship; }
		analysis def Stepped {
			subject ship : Ship;
			analysis inner : Pick;
			objective : MassLimit { subject = inner.picked; }
			return r : Real = inner.picked.hullMass;
		}
		analysis stepped : Stepped { subject ship = test::ship; }
		analysis steppedHeavy : Stepped { subject ship = heavy; }
		analysis def SteppedDefault {
			subject ship : Ship;
			analysis inner : Pick;
			objective : MassLimit;
			return picked : Ship = inner.picked;
		}
		analysis steppedDefault : SteppedDefault { subject ship = heavy; }

		action def Weigh { in s : Ship; out m : Real = s.hullMass; }
		requirement def MassCap { subject mass : Real; require constraint { mass < 2000.0 } }
		analysis def ActionStepped {
			subject ship : Ship;
			action weigh : Weigh { in s = ship; }
			objective : MassCap { subject = weigh.m; }
			assert constraint light { weigh.m < 2000.0 }
			return r : Real = weigh.m;
		}
		analysis actionStepped : ActionStepped { subject ship = test::ship; }
		analysis actionSteppedHeavy : ActionStepped { subject ship = heavy; }

		requirement def PairLimit { subject pair : Ship[2]; require constraint { pair->notEmpty() } }
		requirement def FleetLimit { subject fleet : Ship[1..*]; require constraint { fleet->notEmpty() } }
		analysis def OneForPair { subject ship : Ship; objective : PairLimit; return picked : Ship = ship; }
		analysis def TwoForOne { subject ship : Ship; objective : MassLimit; return picked : Ship[2] = (ship, heavy); }
		analysis def TwoForPair { subject ship : Ship; objective : PairLimit; return picked : Ship[2] = (ship, heavy); }
		analysis def TwoForFleet { subject ship : Ship; objective : FleetLimit; return picked : Ship[2] = (ship, heavy); }
		analysis oneForPair : OneForPair { subject ship = test::ship; }
		analysis twoForOne : TwoForOne { subject ship = test::ship; }
		analysis twoForPair : TwoForPair { subject ship = test::ship; }
		analysis twoForFleet : TwoForFleet { subject ship = test::ship; }
		analysis def RedeclaredPair { subject ship : Ship; objective : PairLimit { subject :>> pair; } return picked : Ship[2] = (ship, heavy); }
		analysis def RedeclaredPairOne { subject ship : Ship; objective : PairLimit { subject :>> pair; } return picked : Ship = ship; }
		analysis redeclaredPair : RedeclaredPair { subject ship = test::ship; }
		analysis redeclaredPairOne : RedeclaredPairOne { subject ship = test::ship; }
	}
`

// qualifiedResultModel binds objective subjects and body assertions to a case's
// unnamed result by qualified name (<Case>::result), the OMG pilot example's form.
const qualifiedResultModel = `
	package test {
		private import ScalarValues::*;
		part def Ship { attribute hullMass : Real; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part heavy : Ship { attribute :>> hullMass = 5000.0; }

		requirement def MassLimit { subject mass : Real; require constraint { mass < 2000.0 } }

		analysis def Qualified {
			subject vessel : Ship;
			objective : MassLimit { subject = Qualified::result; }
			vessel.hullMass
		}
		analysis qualified : Qualified { subject vessel = test::ship; }
		analysis qualifiedHeavy : Qualified { subject vessel = heavy; }

		analysis def Asserting {
			subject vessel : Ship;
			assert constraint capped { Asserting::result < 2000.0 }
			vessel.hullMass
		}
		analysis asserting : Asserting { subject vessel = test::ship; }
		analysis assertingHeavy : Asserting { subject vessel = heavy; }

		analysis def Returned {
			subject vessel : Ship;
			objective : MassLimit { subject = Returned::result; }
			return : Real = vessel.hullMass;
		}
		analysis returnedHeavy : Returned { subject vessel = heavy; }

		analysis def Library {
			subject vessel : Ship;
			objective : MassLimit { subject = Cases::Case::result; }
			vessel.hullMass
		}
		analysis libraryHeavy : Library { subject vessel = heavy; }

		analysis def Mass { subject vessel : Ship; vessel.hullMass }
		analysis def Outer {
			subject vessel : Ship;
			analysis inner : Mass { subject vessel = vessel; }
			objective : MassLimit { subject = inner.result; }
			return total : Real = inner.result * 2.0;
		}
		analysis outer : Outer { subject vessel = test::ship; }
		analysis outerHeavy : Outer { subject vessel = heavy; }

		analysis usageQualified { subject vessel = heavy; objective : MassLimit { subject = usageQualified::result; } vessel.hullMass }

		analysis def Other { subject vessel : Ship; vessel.hullMass }
		analysis def Sibling {
			subject vessel : Ship;
			objective : MassLimit { subject = Other::result; }
			vessel.hullMass
		}
		analysis sibling : Sibling { subject vessel = heavy; }
		analysis light : Mass { subject vessel = test::ship; }
		analysis siblingUsage : Mass {
			subject vessel = heavy;
			objective : MassLimit { subject = light::result; }
			assert constraint capped { light::result < 2000.0 }
		}
	}
`

// TestObjectiveSubjectBindsTheQualifiedResult pins that <Case>::result names the run's
// unnamed result in an objective binding, a body assertion and a step's inner.result read.
func TestObjectiveSubjectBindsTheQualifiedResult(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, qualifiedResultModel))
	for _, tc := range []struct {
		fqn, kind, output, value string
		status                   VerdictStatus
	}{
		{"test::qualified", "objective", "result", "1000.0", VerdictSatisfied},
		{"test::qualifiedHeavy", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::asserting", "assertion", "result", "1000.0", VerdictSatisfied},
		{"test::assertingHeavy", "assertion", "result", "5000.0", VerdictNotSatisfied},
		{"test::returnedHeavy", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::libraryHeavy", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::usageQualified", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::outer", "objective", "total", "2000.0", VerdictSatisfied},
		{"test::outerHeavy", "objective", "total", "10000.0", VerdictNotSatisfied},
	} {
		result, err := ctx.RunAnalysis(oneSymbol(t, idx, tc.fqn), AnalysisArgs{}, nil, nil)
		if err != nil {
			t.Fatalf("RunAnalysis(%s): %v", tc.fqn, err)
		}
		if len(result.Outputs) != 1 || result.Outputs[0].Name != tc.output || FormatValue(result.Outputs[0].Value) != tc.value {
			t.Errorf("%s: outputs = %+v, want %s = %s", tc.fqn, result.Outputs, tc.output, tc.value)
		}
		if len(result.Verdicts) != 1 || result.Verdicts[0].Kind != tc.kind || result.Verdicts[0].Status != tc.status {
			t.Errorf("%s: verdicts = %+v, want the %s %s", tc.fqn, result.Verdicts, tc.kind, tc.status)
		}
		if tc.status == VerdictNotSatisfied && !strings.Contains(result.Verdicts[0].Detail, "< 2000.0") {
			t.Errorf("%s: detail %q does not quote the failed condition", tc.fqn, result.Verdicts[0].Detail)
		}
	}
}

// TestQualifiedResultNamesItsOwnCase pins that <Case>::result reads the run of that case only:
// a sibling definition's result is not the running case's, and a sibling usage's is its own run's.
func TestQualifiedResultNamesItsOwnCase(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, qualifiedResultModel))

	sibling := objectiveVerdict(t, ctx, idx, "test::sibling")
	if sibling.Status != VerdictUndecided || strings.Contains(sibling.Detail, "< 2000.0") {
		t.Errorf("Other::result while Sibling runs: %s (%s), want undecided without reading Sibling's result", sibling.Status, sibling.Detail)
	}

	result, err := ctx.RunAnalysis(oneSymbol(t, idx, "test::siblingUsage"), AnalysisArgs{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outputs) != 1 || FormatValue(result.Outputs[0].Value) != "5000.0" {
		t.Errorf("outputs = %+v, want result = 5000.0", result.Outputs)
	}
	if len(result.Verdicts) != 2 {
		t.Fatalf("verdicts = %+v, want the objective's and the assertion's", result.Verdicts)
	}
	for _, verdict := range result.Verdicts {
		if verdict.Status != VerdictSatisfied {
			t.Errorf("%s %s over light::result: %s (%s), want satisfied (light's 1000.0, not 5000.0)", verdict.Kind, verdict.Name, verdict.Status, verdict.Detail)
		}
	}
}

// objectiveVerdict runs the analysis usage fqn and answers its one objective's verdict.
func objectiveVerdict(t *testing.T, ctx *Context, idx *symbols.Index, fqn string) AnalysisVerdict {
	t.Helper()
	result, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, nil)
	if err != nil {
		t.Fatalf("RunAnalysis(%s): %v", fqn, err)
	}
	if len(result.Verdicts) != 1 || result.Verdicts[0].Kind != "objective" {
		t.Fatalf("RunAnalysis(%s) reported %d verdict(s), want the objective's: %+v", fqn, len(result.Verdicts), result.Verdicts)
	}
	return result.Verdicts[0]
}

// TestObjectiveSubjectBinding pins that an objective binds its requirement's subject as a
// requirement usage does — by keyword alone, by name or by redefinition — and it decides the verdict.
func TestObjectiveSubjectBinding(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	for fqn, want := range map[string]VerdictStatus{
		"test::keyword":        VerdictSatisfied,
		"test::keywordHeavy":   VerdictNotSatisfied,
		"test::named":          VerdictSatisfied,
		"test::redefined":      VerdictSatisfied,
		"test::inline":         VerdictSatisfied,
		"test::inlineHeavy":    VerdictNotSatisfied,
		"test::usageKeyword":   VerdictSatisfied,
		"test::usageRebound":   VerdictNotSatisfied,
		"test::stepped":        VerdictSatisfied,
		"test::steppedHeavy":   VerdictNotSatisfied,
		"test::defaulted":      VerdictSatisfied,
		"test::defaultedHeavy": VerdictNotSatisfied,
		"test::steppedDefault": VerdictNotSatisfied,
	} {
		verdict := objectiveVerdict(t, ctx, idx, fqn)
		if verdict.Status != want {
			t.Errorf("%s: objective %s, want %s (%s)", fqn, verdict.Status, want, verdict.Detail)
		}
		if want == VerdictNotSatisfied && !strings.Contains(verdict.Detail, "hullMass <") {
			t.Errorf("%s: detail %q does not quote the failed condition", fqn, verdict.Detail)
		}
	}
}

// TestObjectiveSubjectDefaultsToTheResult pins the library's default for an unbound objective
// subject (Cases::Case::obj): a result of the wrong type, or none, is undecided for that reason.
func TestObjectiveSubjectDefaultsToTheResult(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))

	mismatch := objectiveVerdict(t, ctx, idx, "test::unbound")
	if mismatch.Status != VerdictUndecided {
		t.Fatalf("a Real result for a Ship subject: %s, want undecided", mismatch.Status)
	}
	for _, want := range []string{"subject s", "case's result", "Cases::Case::obj", "type mismatch", "1000.0 (a Real) is not a Ship"} {
		if !strings.Contains(mismatch.Detail, want) {
			t.Errorf("detail %q does not say %q", mismatch.Detail, want)
		}
	}
	if strings.Contains(mismatch.Detail, "no value for feature") {
		t.Errorf("detail %q reports the symptom, not the mismatch", mismatch.Detail)
	}

	unbound := objectiveVerdict(t, ctx, idx, "test::resultless")
	if unbound.Status != VerdictUndecided {
		t.Fatalf("a case returning no result: %s, want undecided", unbound.Status)
	}
	for _, want := range []string{"objective obj", "s subject is unbound", "return a result"} {
		if !strings.Contains(unbound.Detail, want) {
			t.Errorf("detail %q does not say %q", unbound.Detail, want)
		}
	}
}

// TestObjectiveSubjectBindsAnActionStepOutput pins that an objective's subject and a body
// assertion read a completed action step's output (`weigh.m`) as the case's outputs do.
func TestObjectiveSubjectBindsAnActionStepOutput(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	for fqn, want := range map[string]VerdictStatus{
		"test::actionStepped":      VerdictSatisfied,
		"test::actionSteppedHeavy": VerdictNotSatisfied,
	} {
		result, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, nil)
		if err != nil {
			t.Fatalf("RunAnalysis(%s): %v", fqn, err)
		}
		if len(result.Verdicts) != 2 {
			t.Fatalf("%s: verdicts = %+v, want the objective's and the assertion's", fqn, result.Verdicts)
		}
		for _, verdict := range result.Verdicts {
			if verdict.Status != want {
				t.Errorf("%s: %s %s: %s (%s), want %s", fqn, verdict.Kind, verdict.Name, verdict.Status, verdict.Detail, want)
			}
		}
	}
}

// TestObjectiveSubjectDefaultHonoursMultiplicity pins that the defaulted result must fit the
// subject's multiplicity as well as its type: one Ship for a Ship[2] subject is undecided,
// and a redeclaration stating no multiplicity (`subject :>> pair;`) keeps the Ship[2].
func TestObjectiveSubjectDefaultHonoursMultiplicity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	for fqn, want := range map[string]string{
		"test::oneForPair":        "1 value(s) bound to a feature with multiplicity lower bound 2",
		"test::redeclaredPairOne": "1 value(s) bound to a feature with multiplicity lower bound 2",
		"test::twoForOne":         "2 value(s) bound to a feature with multiplicity upper bound 1",
	} {
		verdict := objectiveVerdict(t, ctx, idx, fqn)
		if verdict.Status != VerdictUndecided {
			t.Fatalf("%s: %s (%s), want undecided", fqn, verdict.Status, verdict.Detail)
		}
		for _, part := range []string{"case's result", "Cases::Case::obj", "multiplicity violation", want} {
			if !strings.Contains(verdict.Detail, part) {
				t.Errorf("%s: detail %q does not say %q", fqn, verdict.Detail, part)
			}
		}
	}
	for _, fqn := range []string{"test::twoForPair", "test::twoForFleet", "test::redeclaredPair"} {
		if verdict := objectiveVerdict(t, ctx, idx, fqn); verdict.Status != VerdictSatisfied {
			t.Errorf("%s: %s (%s), want satisfied", fqn, verdict.Status, verdict.Detail)
		}
	}
}

// TestObjectiveBindingFailureIsUndecided pins that a subject binding that cannot be evaluated
// leaves the objective undecided, naming the binding, while the case's outputs are still reported.
func TestObjectiveBindingFailureIsUndecided(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	result, err := ctx.RunAnalysis(oneSymbol(t, idx, "test::failing"), AnalysisArgs{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outputs) != 1 || result.Outputs[0].Name != "r" {
		t.Errorf("outputs = %+v, want r", result.Outputs)
	}
	verdict := result.Verdicts[0]
	if verdict.Status != VerdictUndecided {
		t.Fatalf("verdict %s (%s), want undecided", verdict.Status, verdict.Detail)
	}
	for _, want := range []string{"objective obj", "subject binding evaluation failed", "division by zero"} {
		if !strings.Contains(verdict.Detail, want) {
			t.Errorf("detail %q does not say %q", verdict.Detail, want)
		}
	}
}

// TestObjectiveSubjectBindingOnAnObject pins that an objective of a usage owned by an object
// reads the object's parts (`subject = h;`), as the case's own subject binding does.
func TestObjectiveSubjectBindingOnAnObject(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Ship { attribute hullMass : Real = 1000.0; }
			requirement def MassLimit {
				subject s : Ship;
				require constraint { s.hullMass < 2000.0 }
			}
			part def Holder {
				part h : Ship;
				analysis inner { subject ship = h; objective : MassLimit { subject = h; } return r : Real = ship.hullMass; }
				analysis viaResult { subject ship = h; objective : MassLimit; return : Ship = ship; }
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	holder, err := ctx.Instantiate(oneSymbol(t, idx, "test::Holder"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fqn := range []string{"test::Holder::inner", "test::Holder::viaResult"} {
		result, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, holder)
		if err != nil {
			t.Fatalf("RunAnalysis(%s): %v", fqn, err)
		}
		if len(result.Verdicts) != 1 || result.Verdicts[0].Status != VerdictSatisfied {
			t.Errorf("%s: verdicts = %+v, want the objective satisfied", fqn, result.Verdicts)
		}
	}
}

const classifiedSubjectModel = `
	package test {
		private import ScalarValues::*;
		part def Ship { attribute hullMass : Real; }
		part def Tanker :> Ship { attribute cargo : Real = 100.0; }
		part def Buoy { attribute hullMass : Real = 1.0; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part heavy : Ship { attribute :>> hullMass = 5000.0; }
		part buoy : Buoy;
		requirement def LadenLimit {
			subject t : Tanker;
			require constraint { t.hullMass + t.cargo < 2000.0 }
		}
		requirement keyword : LadenLimit { subject = ship; }
		requirement keywordHeavy : LadenLimit { subject = heavy; }
		requirement named : LadenLimit { subject t = ship; }
		requirement disjoint : LadenLimit { subject = buoy; }
		requirement def DeclaredLaden { subject t : Tanker = ship; require constraint { t.cargo < 200.0 } }
		requirement declared : DeclaredLaden;
		requirement def DeclaredDisjoint { subject t : Tanker = buoy; require constraint { true } }
		requirement declaredDisjoint : DeclaredDisjoint;
		analysis def Pick {
			subject s : Ship;
			objective : LadenLimit;
			return picked : Ship = s;
		}
		analysis defaulted : Pick { subject s = ship; }
		analysis defaultedHeavy : Pick { subject s = heavy; }
		analysis def PickBound {
			subject s : Ship;
			objective : LadenLimit { subject = s; }
			return picked : Ship = s;
		}
		analysis bound : PickBound { subject s = ship; }
		analysis boundHeavy : PickBound { subject s = heavy; }
		analysis def PickBuoy {
			subject b : Buoy;
			objective : LadenLimit { subject = b; }
			return picked : Buoy = b;
		}
		analysis boundDisjoint : PickBuoy { subject b = buoy; }
	}
`

// TestBoundSubjectIsClassifiedByItsType pins that an object bound to a subject of a narrower
// type than it was declared with is held as one (KerML §7.3.4.1): the subject's own features
// (Tanker's cargo) answer the conditions, defaulted from the result or bound by an expression,
// in a requirement as in an objective. A value the subject cannot hold is refused before it.
func TestBoundSubjectIsClassifiedByItsType(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, classifiedSubjectModel))
	for fqn, want := range map[string]VerdictStatus{
		"test::defaulted":      VerdictSatisfied,
		"test::defaultedHeavy": VerdictNotSatisfied,
		"test::bound":          VerdictSatisfied,
		"test::boundHeavy":     VerdictNotSatisfied,
	} {
		verdict := objectiveVerdict(t, ctx, idx, fqn)
		if verdict.Status != want {
			t.Errorf("%s: objective %s, want %s (%s)", fqn, verdict.Status, want, verdict.Detail)
		}
	}
	for fqn, want := range map[string]error{"test::keyword": nil, "test::keywordHeavy": ErrViolated, "test::named": nil, "test::declared": nil} {
		if _, err := ctx.EvaluateRequirement(oneSymbol(t, idx, fqn), nil); !errors.Is(err, want) {
			t.Errorf("%s: error = %v, want %v", fqn, err, want)
		}
	}

	refused := objectiveVerdict(t, ctx, idx, "test::boundDisjoint")
	if refused.Status != VerdictUndecided {
		t.Fatalf("a Buoy for a Tanker subject: %s (%s), want undecided", refused.Status, refused.Detail)
	}
	for _, want := range []string{"objective obj: subject binding", "type mismatch", "Tanker"} {
		if !strings.Contains(refused.Detail, want) {
			t.Errorf("detail %q does not say %q", refused.Detail, want)
		}
	}
	for _, fqn := range []string{"test::disjoint", "test::declaredDisjoint"} {
		if _, err := ctx.EvaluateRequirement(oneSymbol(t, idx, fqn), nil); !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("requirement %s: error = %v, want %v", fqn, err, ErrTypeMismatch)
		}
	}
}

const boundSubjectMultiplicityModel = `
	package test {
		private import ScalarValues::*;
		private import RealFunctions::sum;
		private import ControlFunctions::select;
		part def Ship { attribute hullMass : Real; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part heavy : Ship { attribute :>> hullMass = 5000.0; }

		requirement def PairLimit {
			subject pair : Ship[2];
			require constraint { sum(pair.hullMass) < 8000.0 }
		}
		requirement def SubPairLimit :> PairLimit { subject :>> pair; }
		requirement def OneLimit {
			subject s : Ship;
			require constraint { s.hullMass < 8000.0 }
		}

		requirement twoForPair : PairLimit { subject = (ship, heavy); }
		requirement oneForPair : PairLimit { subject = ship; }
		requirement noneForPair : PairLimit { subject = (ship, heavy)->select { in s : Ship; s.hullMass > 9000.0 }; }
		requirement redeclaredOneForPair : PairLimit { subject :>> pair = ship; }
		requirement subTwoForPair : SubPairLimit { subject = (ship, heavy); }
		requirement subOneForPair : SubPairLimit { subject = ship; }
		requirement twoForOne : OneLimit { subject = (ship, heavy); }
		requirement noneForOne : OneLimit { subject = (ship, heavy)->select { in s : Ship; s.hullMass > 9000.0 }; }
		requirement def DeclaredOneForPair { subject pair : Ship[2] = ship; require constraint { true } }
		requirement declaredOneForPair : DeclaredOneForPair;

		analysis def PickPair {
			subject s : Ship;
			objective : PairLimit { subject = (s, heavy); }
			return picked : Ship = s;
		}
		analysis pickPair : PickPair { subject s = ship; }
		analysis def PickOneForPair {
			subject s : Ship;
			objective : PairLimit { subject = s; }
			return picked : Ship = s;
		}
		analysis pickOneForPair : PickOneForPair { subject s = ship; }
		analysis def PickSubOneForPair {
			subject s : Ship;
			objective : SubPairLimit { subject :>> pair = s; }
			return picked : Ship = s;
		}
		analysis pickSubOneForPair : PickSubOneForPair { subject s = ship; }
	}
`

// TestBoundSubjectHonoursMultiplicity pins that a subject bound by an expression holds as many
// values as it declares, a redeclaration without one keeping the redefined subject's (KerML §7.3.4.5):
// too few, too many or none is a multiplicity violation, in a requirement as in an objective.
func TestBoundSubjectHonoursMultiplicity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, boundSubjectMultiplicityModel))
	for _, fqn := range []string{"test::twoForPair", "test::subTwoForPair"} {
		if _, err := ctx.EvaluateRequirement(oneSymbol(t, idx, fqn), nil); err != nil {
			t.Errorf("%s: %v", fqn, err)
		}
	}
	for fqn, want := range map[string]string{
		"test::oneForPair":           "1 value(s) bound to a feature with multiplicity lower bound 2",
		"test::noneForPair":          "0 value(s) bound to a feature with multiplicity lower bound 2",
		"test::redeclaredOneForPair": "1 value(s) bound to a feature with multiplicity lower bound 2",
		"test::subOneForPair":        "1 value(s) bound to a feature with multiplicity lower bound 2",
		"test::twoForOne":            "2 value(s) bound to a feature with multiplicity upper bound 1",
		"test::noneForOne":           "0 value(s) bound to a feature with multiplicity lower bound 1",
		"test::declaredOneForPair":   "1 value(s) bound to a feature with multiplicity lower bound 2",
	} {
		_, err := ctx.EvaluateRequirement(oneSymbol(t, idx, fqn), nil)
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Fatalf("%s: error = %v, want %v", fqn, err, ErrMultiplicityViolation)
		}
		for _, part := range []string{"subject binding", want} {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: error %q does not say %q", fqn, err, part)
			}
		}
	}

	if verdict := objectiveVerdict(t, ctx, idx, "test::pickPair"); verdict.Status != VerdictSatisfied {
		t.Errorf("pickPair: %s (%s), want satisfied", verdict.Status, verdict.Detail)
	}
	for _, fqn := range []string{"test::pickOneForPair", "test::pickSubOneForPair"} {
		verdict := objectiveVerdict(t, ctx, idx, fqn)
		if verdict.Status != VerdictUndecided {
			t.Fatalf("%s: %s (%s), want undecided", fqn, verdict.Status, verdict.Detail)
		}
		for _, part := range []string{"objective obj: subject binding", "multiplicity violation", "lower bound 2"} {
			if !strings.Contains(verdict.Detail, part) {
				t.Errorf("%s: detail %q does not say %q", fqn, verdict.Detail, part)
			}
		}
	}
}

const suppliedSubjectModel = `
	package test {
		private import ScalarValues::*;
		private import RealFunctions::sum;
		part def Ship { attribute hullMass : Real; }
		part def Tanker :> Ship { attribute cargo : Real = 100.0; }
		part def Buoy { attribute hullMass : Real = 1.0; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part heavy : Ship { attribute :>> hullMass = 5000.0; }
		part buoy : Buoy;

		requirement def LadenLimit {
			subject t : Tanker;
			require constraint { t.hullMass + t.cargo < 2000.0 }
		}
		requirement def SubLadenLimit :> LadenLimit { subject :>> t; }
		requirement def PairLimit {
			subject pair : Ship[2];
			require constraint { sum(pair.hullMass) < 8000.0 }
		}
		requirement def OneOfPairLimit :> PairLimit { subject pair : Ship[1] :>> pair; }
		requirement def BoundPairLimit {
			subject pair : Ship[2] = (ship, heavy);
			require constraint { sum(pair.hullMass) < 8000.0 }
		}
		requirement def OneOfBoundPairLimit :> BoundPairLimit { subject pair : Ship[1] :>> pair; }
		requirement laden : LadenLimit;
		requirement subLaden : SubLadenLimit;
		requirement pairLimit : PairLimit;
		requirement oneOfPair : OneOfPairLimit;
		requirement boundPair : BoundPairLimit;
		requirement oneOfBoundPair : OneOfBoundPairLimit;
		part context {
			assert satisfy laden by ship;
			assert satisfy laden by heavy;
			assert satisfy laden by buoy;
			assert satisfy subLaden by buoy;
			assert satisfy pairLimit by ship;
			assert satisfy oneOfPair by ship;
		}
	}
`

// TestSuppliedSubjectIsHeldToItsDeclaration pins that the object a satisfaction supplies with `by` is
// held to the subject's effective declaration: classified, or refused on type or multiplicity.
func TestSuppliedSubjectIsHeldToItsDeclaration(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, suppliedSubjectModel))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 6 {
		t.Fatalf("found %d satisfaction assertions, want 6", len(assertions))
	}
	want := []struct {
		err   error
		parts []string
	}{
		{nil, nil},
		{ErrViolated, []string{"t.hullMass + t.cargo < 2000.0"}},
		{ErrTypeMismatch, []string{"satisfy laden by buoy: subject", "is not a Tanker"}},
		{ErrTypeMismatch, []string{"satisfy subLaden by buoy: subject", "is not a Tanker"}},
		{ErrMultiplicityViolation, []string{"satisfy pairLimit by ship: subject", "1 value(s) bound to a feature with multiplicity lower bound 2"}},
		{nil, nil},
	}
	for i, a := range assertions {
		_, err := ctx.EvaluateSatisfaction(a)
		if !errors.Is(err, want[i].err) {
			t.Errorf("%s: error = %v, want %v", a.Text(), err, want[i].err)
			continue
		}
		for _, part := range want[i].parts {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: error %q does not say %q", a.Text(), err, part)
			}
		}
	}
}

// TestInheritedBindingIsHeldToTheRedefiningDeclaration pins that the value an inherited subject
// binds is held to the declaration redefining it: two Ships satisfy `BoundPairLimit` and are refused
// by `OneOfBoundPairLimit`, whose `subject pair : Ship[1] :>> pair` supersedes the `[2]`.
func TestInheritedBindingIsHeldToTheRedefiningDeclaration(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, suppliedSubjectModel))
	if _, err := ctx.EvaluateRequirement(oneSymbol(t, idx, "test::boundPair"), nil); err != nil {
		t.Fatalf("boundPair: %v, want satisfied", err)
	}
	_, err := ctx.EvaluateRequirement(oneSymbol(t, idx, "test::oneOfBoundPair"), nil)
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("oneOfBoundPair: error = %v, want ErrMultiplicityViolation", err)
	}
	for _, part := range []string{"requirement oneOfBoundPair: subject binding", "2 value(s) bound to a feature with multiplicity upper bound 1"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("oneOfBoundPair: error %q does not say %q", err, part)
		}
	}
}

const transactionalBindingModel = `
	package test {
		private import ScalarValues::*;
		part def Ship { attribute hullMass : Real; }
		part def Tanker :> Ship { attribute cargo : Real = 100.0; }
		part def Pilot;
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part pilot : Pilot;

		requirement def PilotedLimit {
			subject t : Tanker;
			actor pilots : Pilot[2];
			require constraint { t.hullMass + t.cargo < 2000.0 }
		}
		requirement def LadenLimit {
			subject t : Tanker;
			require constraint { t.hullMass + t.cargo < 2000.0 }
		}
		requirement piloted : PilotedLimit { actor :>> pilots = pilot; }
		requirement laden : LadenLimit;
		part context {
			assert satisfy piloted by ship;
			assert satisfy laden by ship;
		}
		analysis def Check {
			subject ship : Ship;
			objective : PilotedLimit { subject = ship; actor :>> pilots = pilot; }
			return picked : Ship = ship;
		}
		analysis check : Check { subject ship = test::ship; }
	}
`

// TestFailedBindingLeavesNoClassification pins that member binding is one transaction: a refused
// actor binding undoes the subject's classification and the object it made; the subject alone holds.
func TestFailedBindingLeavesNoClassification(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, transactionalBindingModel))
	tanker, shipUsage, pilotUsage := oneSymbol(t, idx, "test::Tanker"), oneSymbol(t, idx, "test::ship"), oneSymbol(t, idx, "test::pilot")
	// objectOf is the object last made for usage: each evaluation materializes its own.
	objectOf := func(usage *symbols.Symbol) *Instance {
		var latest *Instance
		for _, inst := range ctx.instances {
			if inst.Type == usage && (latest == nil || inst.ID > latest.ID) {
				latest = inst
			}
		}
		return latest
	}
	untouched := func(when string) {
		t.Helper()
		ship := objectOf(shipUsage)
		if ship == nil {
			t.Fatalf("%s: no object for test::ship", when)
		}
		if ctx.instanceConforms(ship, tanker) {
			t.Errorf("%s: the Ship is held as a Tanker (%v)", when, ship.classifiers)
		}
		if pilot := objectOf(pilotUsage); pilot != nil {
			t.Errorf("%s: the Pilot the refused actor binding made is kept (%d)", when, pilot.ID)
		}
	}

	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 2 {
		t.Fatalf("found %d satisfaction assertions, want 2", len(assertions))
	}
	_, err := ctx.EvaluateSatisfaction(assertions[0])
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "actor binding") {
		t.Fatalf("satisfy piloted by ship: error = %v, want the actor's multiplicity violation", err)
	}
	untouched("after the satisfaction")

	verdict := objectiveVerdict(t, ctx, idx, "test::check")
	if verdict.Status != VerdictUndecided || !strings.Contains(verdict.Detail, "actor binding") {
		t.Fatalf("check: %s (%s), want undecided on the actor's multiplicity violation", verdict.Status, verdict.Detail)
	}
	untouched("after the objective")

	if _, err := ctx.EvaluateSatisfaction(assertions[1]); err != nil {
		t.Fatalf("satisfy laden by ship: %v, want satisfied", err)
	}
	if ship := objectOf(shipUsage); !ctx.instanceConforms(ship, tanker) {
		t.Errorf("the Ship is not held as a Tanker after the subject alone bound it")
	}
}

// TestHoldAsReportsAnUndeterminedValueType pins that a value whose type cannot be judged (an object
// the runtime does not know) is refused with that error, not passed as conforming, and that the
// objects beside it are left unclassified.
func TestHoldAsReportsAnUndeterminedValueType(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, suppliedSubjectModel))
	ship, err := ctx.Instantiate(oneSymbol(t, idx, "test::Ship"))
	if err != nil {
		t.Fatalf("Instantiate(Ship): %v", err)
	}
	owner, pair := oneSymbol(t, idx, "test::PairLimit"), oneSymbol(t, idx, "test::PairLimit::pair")
	unknown := ship.ID + 1000
	if _, known := ctx.instances[unknown]; known {
		t.Fatalf("instance %d unexpectedly exists", unknown)
	}
	seq := NewSequence()
	seq.Append(Value{Kind: ValInstance, Instance: ship.ID})
	seq.Append(Value{Kind: ValInstance, Instance: unknown})
	writes, journals := len(ctx.journalWrites), ctx.journals

	err = ctx.holdAs(DeclScope(owner), "subject binding", ctx.boundMemberDecl(owner, []*symbols.Symbol{pair}), NewSequenceValue(seq), pair)
	if !errors.Is(err, ErrUndeterminedValueType) {
		t.Fatalf("error = %v, want ErrUndeterminedValueType", err)
	}
	if errors.Is(err, ErrTypeMismatch) {
		t.Errorf("error = %v, must not be reported as a type mismatch", err)
	}
	if !strings.HasPrefix(err.Error(), "subject binding: ") {
		t.Errorf("error %q does not name the binding", err)
	}
	if len(ship.classifiers) != 0 {
		t.Errorf("ship classified by %v, want nothing classified on refusal", ship.classifiers)
	}
	if len(ctx.journalWrites) != writes || ctx.journals != journals {
		t.Errorf("journal changed on refusal: %d writes, %d open, want %d, %d", len(ctx.journalWrites), ctx.journals, writes, journals)
	}
}

// TestObjectiveBindingKeepsErrorIdentity pins that the undecided detail of a
// mismatched default carries the typed error the requirement engine raises.
func TestObjectiveBindingKeepsErrorIdentity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	for fqn, want := range map[string]error{
		"test::unbound":    ErrTypeMismatch,
		"test::oneForPair": ErrMultiplicityViolation,
		"test::twoForOne":  ErrMultiplicityViolation,
	} {
		sym := oneSymbol(t, idx, fqn)
		run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, nil, nil), sym)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = run.outputValues(ctx); err != nil {
			t.Fatal(err)
		}
		objectives := ctx.ObjectivesOf(sym, nil)
		if len(objectives) != 1 {
			t.Fatalf("%s: %d objectives, want one", fqn, len(objectives))
		}
		_, err = ctx.objectiveBindings(run, objectives[0].Symbol, objectives[0].Name, run.bindingsFrame(ctx))
		if !errors.Is(err, want) {
			t.Errorf("%s: error = %v, want %v", fqn, err, want)
		}
	}
}

const implicitActorModel = `
	package test {
		private import ScalarValues::*;
		part def Ship { attribute hullMass : Real; }
		part def Pilot;
		part def Buoy;
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part pilot : Pilot;
		part copilot : Pilot;
		part buoy : Buoy;

		requirement def PilotedLimit {
			subject t : Ship;
			actor pilots : Pilot[2];
			require constraint { t.hullMass < 2000.0 }
		}
		requirement two : PilotedLimit { subject = ship; actor pilots = (pilot, copilot); }
		requirement one : PilotedLimit { subject = ship; actor pilots = pilot; }
		requirement explicitOne : PilotedLimit { subject = ship; actor :>> pilots = pilot; }
		requirement buoys : PilotedLimit { subject = ship; actor pilots = (buoy, buoy); }
		requirement renamedTwo : PilotedLimit { subject = ship; actor crew = (pilot, copilot); }
		requirement renamedOne : PilotedLimit { subject = ship; actor crew = pilot; }

		requirement def Escorted :> PilotedLimit { subject :>> t; actor :>> pilots; actor tug : Ship; }
		requirement escorted : Escorted { subject = ship; actor pilots = (pilot, copilot); actor tug = ship; }
		requirement escortedByPilot : Escorted { subject = ship; actor pilots = (pilot, copilot); actor tug = pilot; }

		analysis def Check {
			subject ship : Ship;
			objective : PilotedLimit { subject = ship; actor pilots = (pilot, copilot); }
			return picked : Ship = ship;
		}
		analysis check : Check { subject ship = test::ship; }
		analysis def CheckOne {
			subject ship : Ship;
			objective : PilotedLimit { subject = ship; actor pilots = pilot; }
			return picked : Ship = ship;
		}
		analysis checkOne : CheckOne { subject ship = test::ship; }
	}
`

// TestImplicitActorIsHeldToTheInheritedDeclaration pins that an actor bound without `:>>`
// redefines the general's actor at its position, so its value is held to that actor's declaration.
func TestImplicitActorIsHeldToTheInheritedDeclaration(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, implicitActorModel))
	for _, fqn := range []string{"test::two", "test::renamedTwo", "test::escorted"} {
		if _, err := ctx.EvaluateRequirement(oneSymbol(t, idx, fqn), nil); err != nil {
			t.Errorf("%s: %v, want satisfied", fqn, err)
		}
	}
	for fqn, want := range map[string]struct {
		err   error
		parts []string
	}{
		"test::one":             {ErrMultiplicityViolation, []string{"requirement one: actor binding", "1 value(s) bound to a feature with multiplicity lower bound 2"}},
		"test::explicitOne":     {ErrMultiplicityViolation, []string{"requirement explicitOne: actor binding", "lower bound 2"}},
		"test::renamedOne":      {ErrMultiplicityViolation, []string{"requirement renamedOne: actor binding", "lower bound 2"}},
		"test::buoys":           {ErrTypeMismatch, []string{"requirement buoys: actor binding", "is not a Pilot"}},
		"test::escortedByPilot": {ErrTypeMismatch, []string{"requirement escortedByPilot: actor binding", "is not a Ship"}},
	} {
		_, err := ctx.EvaluateRequirement(oneSymbol(t, idx, fqn), nil)
		if !errors.Is(err, want.err) {
			t.Errorf("%s: error = %v, want %v", fqn, err, want.err)
			continue
		}
		for _, part := range want.parts {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: error %q does not say %q", fqn, err, part)
			}
		}
	}

	if verdict := objectiveVerdict(t, ctx, idx, "test::check"); verdict.Status != VerdictSatisfied {
		t.Errorf("check: %s (%s), want satisfied", verdict.Status, verdict.Detail)
	}
	verdict := objectiveVerdict(t, ctx, idx, "test::checkOne")
	if verdict.Status != VerdictUndecided {
		t.Fatalf("checkOne: %s (%s), want undecided", verdict.Status, verdict.Detail)
	}
	for _, part := range []string{"objective obj: actor binding", "multiplicity violation", "lower bound 2"} {
		if !strings.Contains(verdict.Detail, part) {
			t.Errorf("checkOne: detail %q does not say %q", verdict.Detail, part)
		}
	}
}
