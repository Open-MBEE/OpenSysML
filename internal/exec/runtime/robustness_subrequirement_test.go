package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessSubrequirement covers subrequirements — requirement usages
// nested in a requirement definition or usage — whose container binds no subject
// or which name their own container.
func TestRuntimeRobustnessSubrequirement(t *testing.T) {
	t.Run("container_subject_unbound", testSubrequirementContainerSubjectUnbound)
	t.Run("naming_its_container", testSubrequirementNamingItsContainer)
}

// testSubrequirementContainerSubjectUnbound: a subrequirement reading the
// container's subject, checked on a declaration that binds none, reports the
// unbound subject rather than a vacuous pass.
func testSubrequirementContainerSubjectUnbound(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			part def Veh { attribute mass : Real; }
			requirement def R {
				subject v : Veh;
				requirement light { require constraint { v.mass < 10.0 } }
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "R", ast.DefRequirement)
	if sym == nil {
		t.Fatal("R not found")
	}
	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, errUnboundSubject) {
		t.Errorf("expected the unbound subject, got: %v", err)
	}
}

// testSubrequirementNamingItsContainer: a subrequirement whose `require` names
// the requirement containing it is a cycle, reported as stating no condition
// rather than expanded without end.
func testSubrequirementNamingItsContainer(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			requirement r {
				requirement loop { require r; }
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "r", ast.DefRequirement)
	if sym == nil {
		t.Fatal("r not found")
	}
	type outcome struct {
		satisfied bool
		err       error
	}
	done := make(chan outcome, 1)
	go func() {
		satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
		done <- outcome{satisfied, err}
	}()
	select {
	case got := <-done:
		if got.err == nil {
			t.Fatalf("expected an error, got satisfied = %v", got.satisfied)
		}
		if !errors.Is(got.err, ErrNoConditions) {
			t.Errorf("expected ErrNoConditions, got: %v", got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("evaluating a requirement whose subrequirement names it did not return")
	}
}
