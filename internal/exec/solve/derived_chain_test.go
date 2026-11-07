package solve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// subjectChainSource states one condition twice: r3 through a value derived
// from the subject, r4 directly over the subject.
const subjectChainSource = `package SubjectChain {
	private import ScalarValues::*;
	part def Loan { attribute amount : Real; }

	abstract requirement def R {
		subject s : Loan;
		attribute ok : Boolean;
		require constraint { ok }
	}
	requirement r3 : R {
		attribute :>> ok = s.amount > 2.0 and s.amount < 1.0;
	}
	requirement r4 {
		subject s : Loan;
		require constraint { s.amount > 2.0 and s.amount < 1.0 }
	}
}`

// TestDerivedValueThroughTheSubjectIsItsEquation: a value declared through the subject, a parameter
// (SysML v2 §7.21.2) no object is held for, is its `=` equation (KerML 1.1 §7.3.4.5): r3 answers as r4.
func TestDerivedValueThroughTheSubjectIsItsEquation(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "subject_chain.sysml", subjectChainSource)
	r3 := symbolNamed(t, idx, "SubjectChain::r3")
	pins, unfixed := Fixed(ctx, r3, nil)
	if len(unfixed) != 0 {
		t.Fatalf("unfixed = %+v, want none", unfixed)
	}
	if ok := derivedPin(t, pins, "ok"); ok.Value.Kind != runtime.ValInvalid {
		t.Fatalf("ok = %+v, want derived with no value, since nothing gives s.amount one", ok)
	}
	q, err := RequirementWith(ctx, r3, r3.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate r3: %v", err)
	}
	if len(q.Derived) != 1 || len(q.Pinned) != 0 {
		t.Fatalf("Derived = %+v, Pinned = %+v, want the one equation for ok and nothing fixed", q.Derived, q.Pinned)
	}
	solved(t, solver, q, StatusUnsat)

	r4 := symbolNamed(t, idx, "SubjectChain::r4")
	pins, unfixed = Fixed(ctx, r4, nil)
	if len(pins) != 0 || len(unfixed) != 0 {
		t.Fatalf("pins = %+v, unfixed = %+v, want none: r4 declares no value", pins, unfixed)
	}
	q, err = RequirementWith(ctx, r4, r4.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate r4: %v", err)
	}
	solved(t, solver, q, StatusUnsat)
}

// TestDerivedValueThroughAHeldSubjectIsReadThroughIt: when an object is held for the subject,
// the chain is read through it as before: the value the evaluator derives is fixed, no equation stated.
func TestDerivedValueThroughAHeldSubjectIsReadThroughIt(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "held_subject_chain.sysml", `package HeldSubject {
		private import ScalarValues::*;
		part def Loan { attribute amount : Real; }
		part small : Loan { attribute :>> amount = 1.0; }
		requirement def R {
			subject s : Loan;
			attribute ok : Boolean = s.amount > 2.0;
			require constraint { ok }
		}
		requirement r5 : R { subject :>> s = small; }
	}`)
	r5 := symbolNamed(t, idx, "HeldSubject::r5")
	self, err := ctx.Instantiate(r5)
	if err != nil {
		t.Fatalf("instantiate r5: %v", err)
	}
	pins, unfixed := Fixed(ctx, r5, self)
	if len(unfixed) != 0 {
		t.Fatalf("unfixed = %+v, want none", unfixed)
	}
	var ok *Pin
	for i := range pins {
		if pins[i].Name == "ok" {
			ok = &pins[i]
		}
	}
	if ok == nil || ok.Derived != nil || ok.Source != PinHeld || ok.Value.Kind != runtime.ValConst || ok.Value.Const.Bool {
		t.Fatalf("pins = %+v, want ok fixed to false, read through the held small", pins)
	}
	q, err := RequirementWith(ctx, r5, r5.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate r5: %v", err)
	}
	if len(q.Derived) != 0 {
		t.Fatalf("Derived = %+v, want no equation: the chain is read through the held object", q.Derived)
	}
	solved(t, solver, q, StatusUnsat)
}

// TestDerivedValueThroughAParameterOfADefinitionIsItsEquation: a definition checked as such
// holds no object for its parameters, so a value declared through one is its equation.
func TestDerivedValueThroughAParameterOfADefinitionIsItsEquation(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "parameter_chain.sysml", `package ParameterChain {
		private import ScalarValues::*;
		part def Loan { attribute amount : Real; }
		requirement def Tight {
			subject s : Loan;
			attribute ok : Boolean = s.amount > 2.0 and s.amount < 1.0;
			require constraint { ok }
		}
		constraint def Within {
			in loan : Loan;
			attribute margin : Real = loan.amount - 1.0;
			margin > 0.0 and margin < 0.0
		}
	}`)
	tight := symbolNamed(t, idx, "ParameterChain::Tight")
	pins, unfixed := Fixed(ctx, tight, nil)
	if len(unfixed) != 0 {
		t.Fatalf("unfixed = %+v, want none", unfixed)
	}
	derivedPin(t, pins, "ok")
	q, err := RequirementWith(ctx, tight, tight.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate Tight: %v", err)
	}
	if len(q.Derived) != 1 {
		t.Fatalf("Derived = %+v, want the one equation for ok", q.Derived)
	}
	solved(t, solver, q, StatusUnsat)

	within := symbolNamed(t, idx, "ParameterChain::Within")
	pins, unfixed = Fixed(ctx, within, nil)
	if len(unfixed) != 0 {
		t.Fatalf("unfixed = %+v, want none", unfixed)
	}
	derivedPin(t, pins, "margin")
	q, err = ConstraintWith(ctx, within, within.OwnerScope, pins)
	if err != nil {
		t.Fatalf("translate Within: %v", err)
	}
	if len(q.Derived) != 1 {
		t.Fatalf("Derived = %+v, want the one equation for margin", q.Derived)
	}
	solved(t, solver, q, StatusUnsat)
}
