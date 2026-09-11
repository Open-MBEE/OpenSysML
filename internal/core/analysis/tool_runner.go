package analysis

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// toolRunner is the runner one plan attaches to every context its runs use: each
// tool-computed performance becomes a Compute question put to the plan's registry, and
// the answers to equal inputs are remembered across the plan's runs to report divergence.
type toolRunner struct {
	registry  *Registry
	ctx       context.Context
	model     *Model
	budget    Budget
	selection Selection

	mu sync.Mutex
	// answered is the outputs each request, by its bytes, was first answered with.
	answered map[string]string
}

// newToolRunner is the runner of one plan over the held model, putting each computation to
// the registry under the plan's selection.
func (r *Registry) newToolRunner(ctx context.Context, model *Model, budget Budget, selection Selection) *toolRunner {
	return &toolRunner{registry: r, ctx: ctx, model: model, budget: budget, selection: selection, answered: make(map[string]string)}
}

// ToolRunner is the runner a surface attaches to a context it drives outside any plan — the
// prompt's action debugger — so a tool-computed action it steps is put to the registry as a
// Compute question under the selection, as one performed inside a plan is. Divergence between
// equal-input answers is remembered for as long as the runner is attached.
func (r *Registry) ToolRunner(ctx context.Context, held *runtime.Context, budget Budget, selection Selection) runtime.ToolRunner {
	return r.newToolRunner(ctx, Held(held), budget, selection)
}

// RunTool puts the call to the registry as a Compute question. A tool no engine answers for
// is runtime.ToolNotRegisteredError; the refusal of the tool's own engine, or the fault of
// its run, fails the performance as is.
func (t *toolRunner) RunTool(call *runtime.ToolCall) (runtime.ToolAnswer, error) {
	q := Question{Kind: Compute, Subject: symbols.FQNOf(call.Action), Compute: &ComputeAsk{Call: call}}
	plan, err := t.registry.answer(t.ctx, t.model, q, t.budget, t.selection)
	if err != nil {
		if errors.Is(err, ErrNoEngine) {
			return runtime.ToolAnswer{}, &runtime.ToolNotRegisteredError{Tool: call.ToolName}
		}
		return runtime.ToolAnswer{}, err
	}
	if plan.Refused() != nil {
		return runtime.ToolAnswer{}, refusalOf(plan, call.ToolName)
	}
	outputs := make(map[string]runtime.Value, len(plan.Result.Values))
	for _, v := range plan.Result.Values {
		outputs[v.Name] = v.Value
	}
	diverged, err := t.remember(call, outputs)
	if err != nil {
		return runtime.ToolAnswer{}, err
	}
	return runtime.ToolAnswer{Outputs: outputs, Diverged: diverged}, nil
}

// refusalOf is the error a plan every engine refused leaves the performance: under a named
// selection that engine's refusal, else the refusal of the tool's own engine when one is
// registered, else the tool is not registered.
func refusalOf(plan Plan, tool string) error {
	name := ToolEngineName(tool)
	if plan.Selection.Mode == SelectNamed {
		name = plan.Selection.Engine
	}
	for _, step := range plan.Steps {
		if step.Engine == name && step.Refusal != nil {
			return step.Refusal
		}
	}
	return &runtime.ToolNotRegisteredError{Tool: tool}
}

// remember records what the call was answered and reports whether an earlier call with
// the same request was answered differently.
func (t *toolRunner) remember(call *runtime.ToolCall, outputs map[string]runtime.Value) (bool, error) {
	request, err := ToolRequestOf(call)
	if err != nil {
		return false, err
	}
	answer := renderOutputs(outputs)
	t.mu.Lock()
	defer t.mu.Unlock()
	earlier, seen := t.answered[string(request)]
	if !seen {
		t.answered[string(request)] = answer
		return false, nil
	}
	return earlier != answer, nil
}

// renderOutputs spells bound outputs in one order, so two answers compare as text.
func renderOutputs(outputs map[string]runtime.Value) string {
	parts := make([]string, 0, len(outputs))
	for name, value := range outputs {
		parts = append(parts, name+"="+runtime.FormatValue(value))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}
