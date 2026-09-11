package analysis

import (
	"errors"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Worker is one model-derived runtime part — resolver, semantic model and the runtime's memo
// tables — over the shared frozen index; all memoize into plain maps, so a worker serves one
// plan's runs and never another plan's, and every run of the plan reuses what it memoized.
type Worker struct {
	Model *runtime.Model
	// Warming is the time building the worker took: the cost of one more worker.
	Warming time.Duration
}

// ErrNoRuntime is the typed error for a run needing a context the model does not build.
var ErrNoRuntime = errors.New("model builds no runtime of a run's own")

// NoRuntimeError reports an engine refusing a model that builds no context its runs need.
type NoRuntimeError struct {
	Engine string
}

// Error names the engine.
func (e *NoRuntimeError) Error() string {
	return "analysis: " + e.Engine + " needs a runtime of a run's own, which the model does not build"
}

// Is matches ErrNoRuntime.
func (e *NoRuntimeError) Is(target error) bool { return target == ErrNoRuntime }

// plan is the model as one plan holds it: the same surface, a worker of the plan's own.
func (m *Model) plan() *Model {
	if m == nil {
		return nil
	}
	return &Model{Context: m.Context, Semantics: m.Semantics, Fresh: m.Fresh}
}

// toolAttachment is a surface context carrying the plan's tool runner, and the one it carried.
type toolAttachment struct {
	ctx      *runtime.Context
	previous runtime.ToolRunner
}

// compute sets the plan's tool runner, put on every context its runs use.
func (m *Model) compute(tools runtime.ToolRunner) {
	if m != nil {
		m.tools = tools
	}
}

// attach puts the plan's tool runner on a context the surface holds, remembering what it
// carried so release can give it back.
func (m *Model) attach(ctx *runtime.Context) {
	for _, held := range m.attached {
		if held.ctx == ctx {
			return
		}
	}
	m.attached = append(m.attached, toolAttachment{ctx: ctx, previous: ctx.ToolRunner()})
	ctx.SetToolRunner(m.tools)
}

// release gives every surface context its own tool runner back when the plan ends.
func (m *Model) release() {
	if m == nil {
		return
	}
	for _, held := range m.attached {
		held.ctx.SetToolRunner(held.previous)
	}
	m.attached = nil
}

// builds reports whether the model can make a context of a run's own.
func (m *Model) builds() bool {
	return m != nil && m.Semantics != nil && m.Fresh != nil
}

// Worker is the plan's worker, built on first use and shared by every run of the plan.
func (m *Model) Worker() (*Worker, error) {
	if m.worker != nil {
		return m.worker, nil
	}
	if !m.builds() {
		return nil, ErrNoRuntime
	}
	started := time.Now()
	model, err := m.Semantics()
	if err != nil {
		return nil, err
	}
	m.worker = &Worker{Model: model, Warming: time.Since(started)}
	return m.worker, nil
}

// NewContext builds a run's own context on the plan's worker, taking the budget's Steps as
// MaxSteps and Memory as MaxElements; a zero field, like every other bound, keeps Fresh's.
func (m *Model) NewContext(budget Budget) (*runtime.Context, error) {
	worker, err := m.Worker()
	if err != nil {
		return nil, err
	}
	ctx, err := m.Fresh(worker)
	if err != nil {
		return nil, err
	}
	limits := ctx.Budgets()
	if budget.Steps > 0 {
		limits.MaxSteps = int64(budget.Steps)
	}
	if budget.Memory > 0 {
		limits.MaxElements = int64(budget.Memory)
	}
	if err := ctx.SetBudgets(limits); err != nil {
		return nil, err
	}
	if m.tools != nil {
		ctx.SetToolRunner(m.tools)
	}
	return ctx, nil
}

// holds reports whether the surface holds a context of its own over the model.
func (m *Model) holds() bool { return m != nil && m.Context != nil }

// running is the context an execution runs in: the surface's own where it holds one,
// its limits untouched, else one of the run's own under the budget.
func (m *Model) running(engine string, budget Budget) (*runtime.Context, error) {
	if m.holds() {
		ctx, err := m.Context()
		if err == nil && ctx != nil && m.tools != nil {
			m.attach(ctx)
		}
		return ctx, err
	}
	if !m.builds() {
		return nil, &NoRuntimeError{Engine: engine}
	}
	return m.NewContext(budget)
}

// warmed is how many workers the plan built and the time that took.
func (m *Model) warmed() (int, time.Duration) {
	if m == nil || m.worker == nil {
		return 0, 0
	}
	return 1, m.worker.Warming
}
