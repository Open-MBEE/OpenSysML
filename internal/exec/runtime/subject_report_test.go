package runtime

import (
	"errors"
	"testing"
)

// nestedSatisfySrc states a requirement on a nested definition and asserts it of
// the object holding that nesting, whose redefinition violates it.
const nestedSatisfySrc = `package test {
	part def Inner {
		attribute mass : Real = 1.0;
		requirement lim {
			require constraint { mass < 10.0 }
		}
	}
	part def Outer {
		part inner : Inner;
	}
	part big : Outer {
		part :>> inner {
			attribute :>> mass = 99.0;
		}
	}
	assert satisfy Inner::lim by big;
}`

// ownConditionsSatisfySrc states a satisfaction that declares its own require
// condition rather than referencing a requirement, on a definition an object
// redefines.
const ownConditionsSatisfySrc = `package test {
	part def Box {
		attribute size = 1.0;
		satisfy requirement fits {
			require constraint { size < 10.0 }
		}
	}
	part big : Box {
		attribute :>> size = 99.0;
	}
}`

// subsettingSubjectSrc reaches the object of one declaration through the
// collection it subsets, so the feature walked to it is the collection's.
const subsettingSubjectSrc = `package test {
	part def Component {
		attribute v = 1.0;
		constraint ok { v < 10.0 }
	}
	part def Assembly {
		part subsystem : Component[*];
		part small : Component :> subsystem {
			attribute :>> v = 99.0;
		}
	}
	part assembly : Assembly;
}`

// The features reported for a nested subject end in the declaration the object
// materializes, so an object reached through a collection is named by its own
// declaration rather than by the collection gathering it.
func TestReportedFeaturesEndInTheDeclaration(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, subsettingSubjectSrc)
	assembly, err := ctx.Instantiate(memberPath(t, pkg, "assembly"))
	if err != nil {
		t.Fatalf("instantiate assembly: %v", err)
	}
	ok := memberPath(t, pkg, "Component", "ok")
	result, err := ctx.CheckConstraintOn(ok, ok.OwnerScope, assembly)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("ok on assembly: %v", err)
	}
	if result.Holds {
		t.Error("ok on assembly: holds = true, want the subsetting object's 99 to violate it")
	}
	if len(result.SubjectPath) != 1 || result.SubjectPath[0] != "small" {
		t.Errorf("subject path = %q, want the declaration %q rather than the collection it subsets", result.SubjectPath, []string{"small"})
	}
}

// A check reports the object it was evaluated against, which for a condition
// reached through a nested redefinition is the nested object rather than the one
// supplied — so a caller can label the verdict with the object it answers about.
func TestCheckReportsTheChosenSubject(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, nestedSubjectSrc)
	top, err := ctx.Instantiate(memberPath(t, pkg, "top"))
	if err != nil {
		t.Fatalf("instantiate top: %v", err)
	}
	small := memberPath(t, pkg, "Leaf", "small")
	result, err := ctx.CheckConstraintOn(small, small.OwnerScope, top)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("small on top: %v", err)
	}
	if result.Holds {
		t.Error("small on top: holds = true, want the nested redefinition to violate it")
	}
	if result.Subject == nil || result.Subject.ID == top.ID {
		t.Fatalf("subject = %v, want the nested leaf object rather than top #%d", result.Subject, top.ID)
	}
	fv, err := result.Subject.GetFeatureValue(ctx, "value")
	if err != nil {
		t.Fatalf("value of the subject: %v", err)
	}
	if fv == nil || fv.Value.Const.Real != 99.0 {
		t.Errorf("subject's value = %v, want the redefined 99", fv)
	}
}

// A condition with no object to be about reports no subject, so a caller labels
// the verdict with the declaration.
func TestCheckWithNoObjectReportsNoSubject(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, nestedSubjectSrc)
	small := memberPath(t, pkg, "Leaf", "small")
	result, err := ctx.CheckConstraintOn(small, small.OwnerScope, nil)
	if err != nil {
		t.Fatalf("small with no object: %v", err)
	}
	if !result.Holds || result.Subject != nil {
		t.Errorf("small with no object: holds = %t, subject = %v, want the declared value to hold and no subject", result.Holds, result.Subject)
	}
}

// A satisfaction assertion chooses the object its requirement's conditions read
// the same way the requirement does on its own, so `by` naming an object that
// carries the requirement nested answers about that nested object.
func TestSatisfactionRoutesThroughTheSubjectRule(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, nestedSatisfySrc)
	assertions := ctx.SatisfyAssertionsIn(pkg)
	if len(assertions) != 1 {
		t.Fatalf("assertions = %d, want the one the package states", len(assertions))
	}
	big, err := ctx.Instantiate(memberPath(t, pkg, "big"))
	if err != nil {
		t.Fatalf("instantiate big: %v", err)
	}
	result, err := ctx.CheckSatisfactionOn(assertions[0], big)
	if !errors.Is(err, ErrViolated) {
		t.Fatalf("satisfaction on big: holds = %t, err = %v, want the nested redefinition to violate it", result.Holds, err)
	}
	if result.Subject == nil || result.Subject.ID == big.ID {
		t.Fatalf("subject = %v, want the nested inner object rather than big #%d", result.Subject, big.ID)
	}

	// The requirement itself answers alike, which is the agreement the routing
	// is for.
	lim := memberPath(t, pkg, "Inner", "lim")
	if _, err := ctx.EvaluateRequirementOn(lim, lim.OwnerScope, big); !errors.Is(err, ErrViolated) {
		t.Errorf("requirement on big: err = %v, want the same violation the assertion reports", err)
	}
}

// An assertion stating its own conditions routes through the subject rule too,
// so it answers about the object carrying it rather than about the declaration.
func TestSatisfactionOfItsOwnConditionsRoutesThroughTheSubjectRule(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, ownConditionsSatisfySrc)
	big, err := ctx.Instantiate(memberPath(t, pkg, "big"))
	if err != nil {
		t.Fatalf("instantiate big: %v", err)
	}
	assertions := ctx.SatisfyAssertionsIn(pkg)
	if len(assertions) != 1 {
		t.Fatalf("assertions = %d, want the one Box states", len(assertions))
	}
	result, err := ctx.CheckSatisfactionOn(assertions[0], nil)
	if !errors.Is(err, ErrViolated) {
		t.Fatalf("satisfaction: holds = %t, err = %v, want the object's redefined size to violate it", result.Holds, err)
	}
	if result.Subject == nil || result.Subject.ID != big.ID {
		t.Errorf("subject = %v, want the object big #%d that carries the assertion", result.Subject, big.ID)
	}
}
