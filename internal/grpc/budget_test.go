package grpc

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestNewServiceResolvesBudgets: the service reads its bounds once at
// construction, defaulting when the variables are unset.
func TestNewServiceResolvesBudgets(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		clearBudgetEnv(t)
		svc, err := NewService(4, "test")
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc.budgets != runtime.DefaultBudgets() {
			t.Errorf("budgets = %+v, want the defaults %+v", svc.budgets, runtime.DefaultBudgets())
		}
	})

	t.Run("raised", func(t *testing.T) {
		clearBudgetEnv(t)
		t.Setenv(runtime.MaxStepsEnvVar, "1234567")
		t.Setenv(runtime.MaxActionStepsEnvVar, "55555")
		t.Setenv(runtime.MaxElementsEnvVar, "4321")
		svc, err := NewService(4, "test")
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc.budgets.MaxSteps != 1234567 || svc.budgets.MaxActionSteps != 55555 ||
			svc.budgets.MaxElements != 4321 {
			t.Errorf("budgets = %+v, want MaxSteps 1234567, MaxActionSteps 55555 and MaxElements 4321", svc.budgets)
		}
	})

	t.Run("applied_to_every_context", func(t *testing.T) {
		clearBudgetEnv(t)
		t.Setenv(runtime.MaxStepsEnvVar, "777")
		svc, err := NewService(4, "test")
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		ctx := svc.newRuntime(&CachedModel{Index: symbols.NewIndex()})
		if got := ctx.Budgets(); got != svc.budgets {
			t.Errorf("context bounds = %+v, want the service's %+v", got, svc.budgets)
		}
	})
}

// clearBudgetEnv unsets every budget variable for the duration of the test, so
// the surrounding environment cannot decide the outcome.
func clearBudgetEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		runtime.MaxStepsEnvVar,
		runtime.MaxActionStepsEnvVar,
		runtime.MaxStateEventsEnvVar,
		runtime.MaxDoStepsEnvVar,
		runtime.MaxElementsEnvVar,
		runtime.MaxCalcDepthEnvVar,
	} {
		t.Setenv(name, "")
	}
}

// TestNewServiceRejectsUnusableBudget: a value that is not a positive integer
// fails service construction with a message naming the variable, rather than
// falling back to the default silently.
func TestNewServiceRejectsUnusableBudget(t *testing.T) {
	vars := []string{
		runtime.MaxStepsEnvVar,
		runtime.MaxActionStepsEnvVar,
		runtime.MaxStateEventsEnvVar,
		runtime.MaxDoStepsEnvVar,
		runtime.MaxElementsEnvVar,
		runtime.MaxCalcDepthEnvVar,
	}
	for _, name := range vars {
		for _, value := range []string{"0", "-1", "plenty"} {
			t.Run(name+"/"+value, func(t *testing.T) {
				clearBudgetEnv(t)
				t.Setenv(name, value)
				svc, err := NewService(4, "test")
				if err == nil {
					t.Fatalf("NewService accepted %s=%q", name, value)
				}
				if svc != nil {
					t.Error("expected no service alongside the error")
				}
				if !strings.Contains(err.Error(), name) {
					t.Errorf("error %q does not name %s", err, name)
				}
			})
		}
	}
}
