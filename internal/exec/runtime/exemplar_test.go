package runtime

import (
	"errors"
	"testing"
)

// A constraint checked about no object reads its type's declared defaults, and
// `this` there denotes an exemplar of the type made for the check and abandoned
// with it: the usage typed by a constraint def binding its context to `this`
// decides from the defaults, and leaves no object behind.
func TestConstraintAboutNoObjectReadsThisAsExemplar(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::Real;
			part def Tank {
				attribute level : Real default = 80.0;
				attribute capacity : Real default = 100.0;
				constraint def 'Below capacity' {
					in ref context : Tank[1];
					context.level <= context.capacity
				}
				assert constraint 'below capacity' : 'Below capacity' {
					in ref :>> context = this;
				}
				constraint def 'Over capacity' {
					in ref context : Tank[1];
					context.level > context.capacity
				}
				assert constraint 'over capacity' : 'Over capacity' {
					in ref :>> context = this;
				}
			}
			part def Open {
				attribute level : Real;
				assert constraint bounded { this.level >= 0.0 }
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	tank := requirementNamed(t, pkg, "Tank")
	before := len(ctx.instances)

	holds, err := ctx.EvaluateConstraint(requirementNamed(t, tank.Scope, "below capacity"), tank.Scope)
	if err != nil || !holds {
		t.Errorf("below capacity = %v, %v; want true from the defaults", holds, err)
	}
	holds, err = ctx.EvaluateConstraint(requirementNamed(t, tank.Scope, "over capacity"), tank.Scope)
	var violation *ViolationError
	if holds || !errors.As(err, &violation) {
		t.Errorf("over capacity = %v, %v; want a violation from the defaults", holds, err)
	}
	if after := len(ctx.instances); after != before {
		t.Errorf("the checks left %d object(s) behind; want none", after-before)
	}

	open := requirementNamed(t, pkg, "Open")
	_, err = ctx.EvaluateConstraint(requirementNamed(t, open.Scope, "bounded"), open.Scope)
	if errors.Is(err, ErrThisNotAnObject) || err == nil {
		t.Errorf("bounded: err = %v; want the open feature reported, not this", err)
	}
}

// The check abandons only its exemplar: a usage the condition reads on the way,
// materialized then, is the object its usage denotes and keeps its identity.
func TestConstraintAboutNoObjectKeepsUsagesItReads(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::Real;
			part def Tank {
				attribute level : Real default = 80.0;
				attribute capacity : Real default = 100.0;
				constraint def 'Within sample' {
					in ref context : Tank[1];
					context.level <= sample.capacity
				}
				assert constraint 'within sample' : 'Within sample' {
					in ref :>> context = this;
				}
			}
			part sample : Tank;
		}
	`
	ctx, pkg := conditionFixture(t, src)
	tank := requirementNamed(t, pkg, "Tank")
	sample := requirementNamed(t, pkg, "sample")
	before := len(ctx.instances)

	check := func() {
		t.Helper()
		holds, err := ctx.EvaluateConstraint(requirementNamed(t, tank.Scope, "within sample"), tank.Scope)
		if err != nil || !holds {
			t.Fatalf("within sample = %v, %v; want true from the defaults", holds, err)
		}
	}
	check()
	first, live := ctx.liveOccurrences(sample)
	if !live || len(first) != 1 {
		t.Fatalf("after the check sample denotes %v (live %v); want its one object", first, live)
	}
	check()
	second, live := ctx.liveOccurrences(sample)
	if !live || len(second) != 1 || second[0].ID != first[0].ID {
		t.Errorf("after the second check sample denotes %v (live %v); want object %d still", second, live, first[0].ID)
	}
	if after := len(ctx.instances); after != before+1 {
		t.Errorf("the checks left %d object(s); want only sample's", after-before)
	}
}
