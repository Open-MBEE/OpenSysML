package runtime

import (
	"errors"
	"reflect"
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
	if fv == nil || fv.Value.Const.AsReal() != 99.0 {
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

// subsettingCollectionSrc gathers objects of several declarations into one
// collection: `small` and the two `smalls` subset `subsystem`, which holds no
// object of its own, and `outer` nests the whole assembly one level down.
const subsettingCollectionSrc = `package test {
    part def Component {
        attribute v default = 1.0;
        assert constraint ok { v < 10.0 }
    }
    part def Assembly {
        part subsystem : Component[*];
        part small : Component :> subsystem {
            attribute :>> v = 99.0;
        }
        part smalls : Component[2] :> subsystem {
            attribute :>> v = 50.0;
        }
    }
    part def Outer {
        part inner : Assembly;
    }
    part assembly : Assembly;
    part outer : Outer;
}`

// Validation labels an object by the usage that declares it — `small`, `smalls[2]` —
// never by its position in the collection that usage subsets (KerML §7.3.4.4, SysML §7.6.3).
func TestValidationLabelsObjectsByTheirDeclaration(t *testing.T) {
	for _, tc := range []struct {
		root string
		want [][]string
	}{
		{"assembly", [][]string{{"small"}, {"smalls[1]"}, {"smalls[2]"}}},
		{"outer", [][]string{{"inner", "small"}, {"inner", "smalls[1]"}, {"inner", "smalls[2]"}}},
	} {
		ctx, pkg := nestedSubjectFixture(t, subsettingCollectionSrc)
		root, err := ctx.Instantiate(memberPath(t, pkg, tc.root))
		if err != nil {
			t.Fatalf("instantiate %s: %v", tc.root, err)
		}
		report, err := ctx.ValidateObject(root, nil)
		if err != nil {
			t.Fatalf("validate %s: %v", tc.root, err)
		}
		var got [][]string
		for _, v := range report.Verdicts {
			got = append(got, v.Path)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("validate %s: paths = %q, want %q", tc.root, got, tc.want)
		}
	}
}

// The objects a collection gathers from subsetting usages are held under those
// usages' segments, so a listing of what an object holds spells them as a verdict does.
func TestHeldObjectsLabelsSubsettingObjectsByTheirDeclaration(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, subsettingCollectionSrc)
	assembly, err := ctx.Instantiate(memberPath(t, pkg, "assembly"))
	if err != nil {
		t.Fatalf("instantiate assembly: %v", err)
	}
	held, err := ctx.HeldObjects(assembly)
	if err != nil {
		t.Fatalf("held objects: %v", err)
	}
	var got []string
	for _, h := range held {
		got = append(got, h.Segment)
	}
	if want := []string{"small", "smalls[1]", "smalls[2]"}; !reflect.DeepEqual(got, want) {
		t.Errorf("segments = %q, want %q", got, want)
	}
}

// A check about a nested subsetting object and the validation reaching it label
// the object by one path, the declaration at every segment.
func TestValidationAndCheckAgreeOnANestedSubsettingObject(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, `package test {
    part def Component {
        attribute v default = 1.0;
        assert constraint ok { v < 10.0 }
    }
    part def Assembly {
        part subsystem : Component[*];
        part small : Component :> subsystem {
            attribute :>> v = 99.0;
        }
    }
    part def Outer {
        part inner : Assembly;
    }
    part outer : Outer;
}`)
	outer, err := ctx.Instantiate(memberPath(t, pkg, "outer"))
	if err != nil {
		t.Fatalf("instantiate outer: %v", err)
	}
	ok := memberPath(t, pkg, "Component", "ok")
	result, err := ctx.CheckConstraintOn(ok, ok.OwnerScope, outer)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("ok on outer: %v", err)
	}
	want := []string{"inner", "small"}
	if !reflect.DeepEqual(result.SubjectPath, want) {
		t.Errorf("check subject path = %q, want %q", result.SubjectPath, want)
	}
	report, err := ctx.ValidateObject(outer, nil)
	if err != nil {
		t.Fatalf("validate outer: %v", err)
	}
	if len(report.Verdicts) != 1 || !reflect.DeepEqual(report.Verdicts[0].Path, want) {
		t.Errorf("validation paths = %v, want one verdict on %q", report.Verdicts, want)
	}
}

// A usage subsetting a feature through another (`leaf :> middle :> items`)
// names its object at whichever collection reaches it first.
func TestValidationLabelsATransitiveSubsettingObjectByItsDeclaration(t *testing.T) {
	ctx, pkg := nestedSubjectFixture(t, `package test {
    part def Component {
        attribute v default = 1.0;
        assert constraint ok { v < 10.0 }
    }
    part def Assembly {
        part items : Component[*];
        part middle : Component[*] :> items;
        part leaf : Component :> middle {
            attribute :>> v = 99.0;
        }
    }
    part assembly : Assembly;
}`)
	assembly, err := ctx.Instantiate(memberPath(t, pkg, "assembly"))
	if err != nil {
		t.Fatalf("instantiate assembly: %v", err)
	}
	report, err := ctx.ValidateObject(assembly, nil)
	if err != nil {
		t.Fatalf("validate assembly: %v", err)
	}
	var paths [][]string
	for _, v := range report.Verdicts {
		paths = append(paths, v.Path)
	}
	if want := [][]string{{"leaf"}}; !reflect.DeepEqual(paths, want) {
		t.Errorf("validation paths = %q, want %q", paths, want)
	}
	held, err := ctx.HeldObjects(assembly)
	if err != nil {
		t.Fatalf("held objects: %v", err)
	}
	if len(held) != 1 || held[0].Segment != "leaf" {
		t.Errorf("held segments = %v, want the one object under leaf", held)
	}
}
