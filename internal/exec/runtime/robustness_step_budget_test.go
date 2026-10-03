package runtime

import (
	"errors"
	"math"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessStepBudget: a budget at the int64 limit still binds, and
// a spent budget stays spent, however the steps are charged.
func TestRuntimeRobustnessStepBudget(t *testing.T) {
	limitContext := func(t *testing.T, spent int64) *Context {
		t.Helper()
		resolver := resolve.New(symbols.NewIndex())
		ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), math.MaxInt64)
		ctx.run.steps = spent
		return ctx
	}
	exceeded := func(t *testing.T, ctx *Context, err error) {
		t.Helper()
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("want ErrStepLimitExceeded, got %v", err)
		}
		if ctx.run.steps != math.MaxInt64 {
			t.Fatalf("counter = %d after the budget was spent, want the limit %d", ctx.run.steps, int64(math.MaxInt64))
		}
	}

	t.Run("the_last_step_of_an_int64_budget_is_spent_and_the_next_fails", func(t *testing.T) {
		ctx := limitContext(t, math.MaxInt64-1)
		if err := ctx.incrementStep(); err != nil {
			t.Fatalf("the last step of the budget: %v", err)
		}
		exceeded(t, ctx, ctx.incrementStep())
		exceeded(t, ctx, ctx.incrementStep())
	})

	t.Run("a_charge_past_an_int64_budget_fails_without_wrapping", func(t *testing.T) {
		ctx := limitContext(t, math.MaxInt64-2)
		exceeded(t, ctx, ctx.chargeSteps(5))
		exceeded(t, ctx, ctx.chargeSteps(1))
		exceeded(t, ctx, ctx.incrementStep())
	})

	t.Run("a_charge_reaching_an_int64_budget_exactly_is_spent", func(t *testing.T) {
		ctx := limitContext(t, math.MaxInt64-5)
		if err := ctx.chargeSteps(5); err != nil {
			t.Fatalf("a charge of the budget's last 5 steps: %v", err)
		}
		exceeded(t, ctx, ctx.chargeSteps(1))
	})

	t.Run("a_charge_of_the_whole_int64_budget_fails_once_any_is_spent", func(t *testing.T) {
		ctx := limitContext(t, 1)
		exceeded(t, ctx, ctx.chargeSteps(math.MaxInt64))
	})
}
