package analysis

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	// uses is every tool call the plan's runs made; seq orders them.
	uses []ToolUse
	next uint64
}

// ToolUse is one tool call a plan made: the entry it went to and what was run.
type ToolUse struct {
	Tool, Version, File, Executable string
	// Args are the argv entries after the executable, nil for an entry without an
	// invocation block.
	Args []string
	// Stdin is the request an entry without an invocation block read.
	Stdin []byte
	// Failed is the failure the call ended with, empty when it answered.
	Failed string
	// in is the context the call was made from.
	in *runtime.Context
	// seq is the call's order among the runner's calls.
	seq uint64
}

// String spells the call as one line: `ThermalSolver 2.3 from
// /etc/opensysml/tools/thermal.json: /usr/bin/python3 solve.py --mass 12.5`.
func (u ToolUse) String() string {
	head := u.Tool
	if u.Version != "" {
		head += " " + u.Version
	}
	if u.File != "" {
		head += " from " + u.File
	}
	text := head + ": " + u.Executable
	if u.Args == nil {
		text += " < " + string(u.Stdin)
	} else {
		for _, arg := range u.Args {
			if arg == "" || strings.ContainsAny(arg, " \t\r\n\"'") {
				arg = strconv.Quote(arg)
			}
			text += " " + arg
		}
	}
	if u.Failed != "" {
		text += " failed: " + u.Failed
	}
	return text
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
	t.mu.Lock()
	t.next++
	seq := t.next
	t.mu.Unlock()
	ask := &ComputeAsk{Call: call, used: func(use ToolUse) { t.note(call, seq, use) }}
	q := Question{Kind: Compute, Subject: symbols.FQNOf(call.Action), Compute: ask}
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
	diverged, err := t.remember(call, plan.Result.Reply)
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

// remember records the reply the call's request was answered with and reports whether an
// earlier call with the same request was answered another; the reply is compared as the
// canonical rendering of every mapped output, not as any one action binds it.
func (t *toolRunner) remember(call *runtime.ToolCall, answer string) (bool, error) {
	request, err := ToolRequestOf(call)
	if err != nil {
		return false, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	earlier, seen := t.answered[string(request)]
	if !seen {
		t.answered[string(request)] = answer
		return false, nil
	}
	return earlier != answer, nil
}

// note keeps a use an engine reported for the call, attributed to the context the call
// was made from and to its place in call order; a run the plan dropped is kept too.
func (t *toolRunner) note(call *runtime.ToolCall, seq uint64, use ToolUse) {
	use.in = call.Context()
	use.seq = seq
	t.mu.Lock()
	defer t.mu.Unlock()
	t.uses = append(t.uses, use)
}

// used is every tool call the runner's runs made, in call order.
func (t *toolRunner) used() []ToolUse {
	t.mu.Lock()
	defer t.mu.Unlock()
	uses := append([]ToolUse(nil), t.uses...)
	sort.SliceStable(uses, func(i, j int) bool { return uses[i].seq < uses[j].seq })
	return uses
}
