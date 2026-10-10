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
