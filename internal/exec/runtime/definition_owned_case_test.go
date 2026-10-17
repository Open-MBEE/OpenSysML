package runtime

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// definitionOwnedCaseModel declares a verification case inside a part
// definition, a feature of every object of it: its body reads the owner's cost.
const definitionOwnedCaseModel = `
package test {
	private import ScalarValues::Real;
	private import ScalarValues::Boolean;

	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement { :>> maximumCost = 100.0; }

	part def Definition {
		attribute cost : Real;
		satisfy costReq;

		verification costTest {
			objective costObjective { verify costReq; }
			return verdict = VerificationCases::PassIf(cost <= 100.0);
		}

		verification subjectTest {
			subject s : Definition;
			return verdict = VerificationCases::PassIf(s.cost <= 100.0 and cost <= 100.0);
		}

		verification planTest {
			verification step { return verdict = VerificationCases::PassIf(cost <= 100.0); }
			return verdict = step.verdict;
		}
	}

	part cheap : Definition { attribute :>> cost = 80.0; }
	part dear : Definition { attribute :>> cost = 120.0; }

	part def Other { attribute ok : Boolean; }
	part other : Other { attribute :>> ok = true; }

	part def Holder {
		attribute cost : Real;
		verification heldTest {
			subject s : Other;
			return verdict = VerificationCases::PassIf(cost <= 100.0 and s.ok);
		}
	}
	part holder : Holder { attribute :>> cost = 80.0; }
}`

// definitionOwnedCase builds the model and materializes the named objects.
func definitionOwnedCase(t *testing.T, objects ...string) (*symbols.Index, *Context, []*Instance) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, definitionOwnedCaseModel))
	var made []*Instance
	for _, name := range objects {
		inst, err := ctx.Instantiate(oneSymbol(t, idx, name))
		if err != nil {
			t.Fatalf("instantiate %s: %v", name, err)
		}
		made = append(made, inst)
	}
	return idx, ctx, made
}

// runOn runs the case on an object given as the run's object argument.
func runOn(t *testing.T, idx *symbols.Index, ctx *Context, name string, object *Instance) VerificationVerdict {
	t.Helper()
	sym := oneSymbol(t, idx, name)
	result, err := ctx.RunVerification(sym, AnalysisArgs{Subject: object}, sym.OwnerScope, nil)
	if err != nil {
		t.Fatalf("RunVerification(%s): %v", name, err)
	}
	return result.Verdict
}

// TestDefinitionOwnedCaseRunsOnTheObjectGiven pins that an object given to a case
// a definition owns is the object the case is a feature of, not its subject: the
// body reads that object's cost, and the verdict names it.
func TestDefinitionOwnedCaseRunsOnTheObjectGiven(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::cheap", "test::dear")
	cheap, dear := objects[0], objects[1]
	for _, tc := range []struct {
		object *Instance
		want   VerdictKind
	}{{cheap, VerdictPass}, {dear, VerdictFail}} {
		got := runOn(t, idx, ctx, "test::Definition::costTest", tc.object)
		if got.Kind != tc.want || got.Subject != tc.object {
			t.Errorf("costTest on #%d = %s (%s) about #%v, want %s about it", tc.object.ID, got.Kind, got.Detail, got.Subject, tc.want)
		}
	}
}

// TestDefinitionOwnedCaseRunsOnTheObjectGivenOverOneFound pins that the object
// given wins over an object of the definition the caller found for the case: the
// body reads the given object's cost, not the found object's unset one.
func TestDefinitionOwnedCaseRunsOnTheObjectGivenOverOneFound(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::Definition", "test::dear")
	found, dear := objects[0], objects[1]
	sym := oneSymbol(t, idx, "test::Definition::costTest")
	result, err := ctx.RunVerification(sym, AnalysisArgs{Subject: dear}, sym.OwnerScope, found)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Verdict; got.Kind != VerdictFail || got.Subject != dear {
		t.Errorf("costTest on dear with #%d found = %s (%s) about %v, want fail about dear", found.ID, got.Kind, got.Detail, got.Subject)
	}
}

// TestHeldCaseBindsTheObjectGivenToItsSubject pins that an object given to a case
// an object holds binds the subject the case declares, of another type than the
// holder, and the holder stays the object the case runs as a feature of.
func TestHeldCaseBindsTheObjectGivenToItsSubject(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::holder", "test::other")
	holder, other := objects[0], objects[1]
	sym := oneSymbol(t, idx, "test::Holder::heldTest")
	result, err := ctx.RunVerification(sym, AnalysisArgs{Subject: other}, sym.OwnerScope, holder)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Verdict; got.Kind != VerdictPass || got.Subject != holder {
		t.Errorf("heldTest on other held by #%d = %s (%s) about %v, want pass about holder", holder.ID, got.Kind, got.Detail, got.Subject)
	}
}

// TestDefinitionOwnedCaseErrorVerdictNamesTheObject pins that a body that fails
// on an object still answers a verdict about that object.
func TestDefinitionOwnedCaseErrorVerdictNamesTheObject(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::Definition")
	got := runOn(t, idx, ctx, "test::Definition::costTest", objects[0])
	if got.Kind != verdictError || !strings.Contains(got.Detail, "no value for feature cost") || got.Subject != objects[0] {
		t.Errorf("costTest on an object with no cost = %s (%s) about %v, want error about #%d", got.Kind, got.Detail, got.Subject, objects[0].ID)
	}
}

// TestDefinitionOwnedCaseSubcasesAreAboutTheObjectToo pins that a verification
// case a run performs as a step answers for the object the run is about.
func TestDefinitionOwnedCaseSubcasesAreAboutTheObjectToo(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::cheap", "test::dear")
	sym := oneSymbol(t, idx, "test::Definition::planTest")
	for _, tc := range []struct {
		object *Instance
		want   VerdictKind
	}{{objects[0], VerdictPass}, {objects[1], VerdictFail}} {
		result, err := ctx.RunVerification(sym, AnalysisArgs{Subject: tc.object}, sym.OwnerScope, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Verdict.Kind != tc.want || result.Verdict.Subject != tc.object {
			t.Errorf("planTest on #%d = %s (%s) about %v, want %s about it", tc.object.ID, result.Verdict.Kind, result.Verdict.Detail, result.Verdict.Subject, tc.want)
		}
		if len(result.Subcases) != 1 || result.Subcases[0].Kind != tc.want || result.Subcases[0].Subject != tc.object {
			t.Errorf("planTest on #%d subcases = %+v, want one %s about it", tc.object.ID, result.Subcases, tc.want)
		}
	}
}

// TestDefinitionOwnedCaseBindsAnUnboundSubjectToTheObjectToo pins that a case
// declaring a subject it leaves unbound reads the object given as both its owner
// and its subject.
func TestDefinitionOwnedCaseBindsAnUnboundSubjectToTheObjectToo(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::cheap", "test::dear")
	if got := runOn(t, idx, ctx, "test::Definition::subjectTest", objects[0]); got.Kind != VerdictPass {
		t.Errorf("subjectTest on cheap = %s (%s), want pass", got.Kind, got.Detail)
	}
	if got := runOn(t, idx, ctx, "test::Definition::subjectTest", objects[1]); got.Kind != VerdictFail {
		t.Errorf("subjectTest on dear = %s (%s), want fail", got.Kind, got.Detail)
	}
}

// TestDefinitionOwnedCaseKeepsANamedSubject pins that a subject the arguments
// bind by name is kept when an object is given as the owner.
func TestDefinitionOwnedCaseKeepsANamedSubject(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::cheap", "test::dear")
	sym := oneSymbol(t, idx, "test::Definition::subjectTest")
	args := AnalysisArgs{Subject: objects[0], Named: map[string]Value{"s": {Kind: ValInstance, Instance: objects[1].ID}}}
	result, err := ctx.RunVerification(sym, args, sym.OwnerScope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict.Kind != VerdictFail || result.Run.Subject != objects[1] || result.Verdict.Subject != objects[0] {
		t.Errorf("subjectTest(s = dear) on cheap = %s, subject %v, owner %v; want fail reading dear as the subject on cheap", result.Verdict.Kind, result.Run.Subject, result.Verdict.Subject)
	}
}

// TestVerificationRunsRecordTheRunsMade pins that a report sharing one
// VerificationRuns runs a case once per object checked, not per check.
func TestVerificationRunsRecordTheRunsMade(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::cheap")
	root := idx.DocumentRoot("<test>")
	req := oneSymbol(t, idx, "test::costReq")
	runs := &VerificationRuns{}
	scopes := []*symbols.Scope{root}
	if got := ctx.VerificationVerdictsOnce(scopes, req, objects[0], runs); len(got) != 1 {
		t.Fatalf("first check = %d verdicts, want 1", len(got))
	}
	if got := ctx.VerificationVerdictsOnce(scopes, req, objects[0], runs); len(got) != 0 {
		t.Errorf("second check of the same object = %d verdicts, want none: the case already ran on it", len(got))
	}
	if got := ctx.VerificationVerdictsOnce(scopes, req, nil, runs); len(got) != 0 {
		t.Errorf("check about no object = %d verdicts, want none: cheap is the only object", len(got))
	}
}

// TestDefinitionOwnedCaseWithoutAnObject pins the run made when no object is
// given: inconclusive, asking for an object, when the run holds none; on the
// single object of the definition; a refusal naming several.
func TestDefinitionOwnedCaseWithoutAnObject(t *testing.T) {
	idx, ctx, _ := definitionOwnedCase(t)
	got := runOn(t, idx, ctx, "test::Definition::costTest", nil)
	if got.Kind != VerdictInconclusive || got.Detail != "needs an object of test::Definition: none is instantiated" || got.Subject != nil {
		t.Errorf("costTest with no object = %s (%s) about %v, want inconclusive, needing an object of the definition", got.Kind, got.Detail, got.Subject)
	}

	idx, ctx, objects := definitionOwnedCase(t, "test::dear")
	if got := runOn(t, idx, ctx, "test::Definition::costTest", nil); got.Kind != VerdictFail || got.Subject != objects[0] {
		t.Errorf("costTest with one object = %s about %v, want fail about it", got.Kind, got.Subject)
	}
	// The one object found binds the unbound declared subject too.
	if got := runOn(t, idx, ctx, "test::Definition::subjectTest", nil); got.Kind != VerdictFail {
		t.Errorf("subjectTest with one object = %s (%s), want fail", got.Kind, got.Detail)
	}

	idx, ctx, _ = definitionOwnedCase(t, "test::cheap", "test::dear")
	got = runOn(t, idx, ctx, "test::Definition::costTest", nil)
	if got.Kind != verdictError || !strings.Contains(got.Detail, "ambiguous subject") || !strings.Contains(got.Detail, "dear") {
		t.Errorf("costTest with two objects = %s (%s), want a refusal naming both", got.Kind, got.Detail)
	}
}

// TestDefinitionOwnedCaseRefusesAnObjectOfAnotherType pins that an object that
// is not of the definition and holds no object of it is refused by name.
func TestDefinitionOwnedCaseRefusesAnObjectOfAnotherType(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::other")
	sym := oneSymbol(t, idx, "test::Definition::costTest")
	_, _, _, err := ctx.runCase(sym, AnalysisArgs{Subject: objects[0]}, sym.OwnerScope, nil)
	if !errors.Is(err, ErrNotCarried) || !strings.Contains(err.Error(), "test::Definition") {
		t.Errorf("costTest on an Other = %v, want ErrNotCarried naming the definition", err)
	}
}

// TestVerificationVerdictsOfADefinitionOwnedCaseRunPerObject pins that the cases
// verifying a requirement run once per object of the owning definition, and for
// a verdict about one object on that object alone.
func TestVerificationVerdictsOfADefinitionOwnedCaseRunPerObject(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::cheap", "test::dear")
	root := idx.DocumentRoot("<test>")
	req := oneSymbol(t, idx, "test::costReq")
	spell := func(verdicts []VerificationVerdict) string {
		var got []string
		for _, v := range verdicts {
			id := int64(0)
			if v.Subject != nil {
				id = v.Subject.ID
			}
			got = append(got, strings.TrimPrefix(v.Case, "test::Definition::")+"="+string(v.Kind)+"@"+strconv.FormatInt(id, 10))
		}
		return strings.Join(got, ",")
	}
	all := spell(ctx.VerificationVerdictsIn([]*symbols.Scope{root}, req))
	want := "costTest=pass@" + strconv.FormatInt(objects[0].ID, 10) + ",costTest=fail@" + strconv.FormatInt(objects[1].ID, 10)
	if all != want {
		t.Errorf("VerificationVerdictsIn = %s, want %s", all, want)
	}
	about := spell(ctx.VerificationVerdictsOn([]*symbols.Scope{root}, req, objects[1]))
	if want := "costTest=fail@" + strconv.FormatInt(objects[1].ID, 10); about != want {
		t.Errorf("VerificationVerdictsOn(dear) = %s, want %s", about, want)
	}
	// An object neither of the definition nor holding one gets no verdict of
	// the case: it is not borrowed from another object.
	other, err := ctx.Instantiate(oneSymbol(t, idx, "test::other"))
	if err != nil {
		t.Fatal(err)
	}
	if got := spell(ctx.VerificationVerdictsOn([]*symbols.Scope{root}, req, other)); got != "" {
		t.Errorf("VerificationVerdictsOn(other) = %s, want none", got)
	}
}

// TestDefinitionOwnedSatisfyIsCheckedOnTheObjectOfTheDefinition pins that a
// `satisfy` without `by` a definition states is checked on the single object of
// that definition when none is given.
func TestDefinitionOwnedSatisfyIsCheckedOnTheObjectOfTheDefinition(t *testing.T) {
	idx, ctx, objects := definitionOwnedCase(t, "test::dear")
	root := idx.DocumentRoot("<test>")
	var stated *SatisfyAssertion
	for _, a := range ctx.SatisfyAssertionsIn(root) {
		if a.Owner != nil && a.Owner.Name == "Definition" {
			stated = a
		}
	}
	if stated == nil {
		t.Fatal("the definition's satisfy was not found")
	}
	result, err := ctx.CheckSatisfactionOn(stated, nil)
	if !errors.Is(err, ErrViolated) || result.Holds || result.Subject != objects[0] {
		t.Errorf("satisfy costReq on the one object = holds %v about %v (%v), want violated about dear", result.Holds, result.Subject, err)
	}
}
