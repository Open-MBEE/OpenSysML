package analysis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Plan is how a question was answered: the selection made, every engine
// consulted in the order the selection ranks them, and the answer that stood.
type Plan struct {
	Question  Question
	Selection Selection
	Steps     []Step
	// Result is the first covered answer under auto or a named engine, and the
	// composition of every finished engine's answer under all; when no engine
	// covered the question it is the last not-covered result, or one naming
	// every refusal.
	Result Result
	// Disagreements are the contradictions the composition under all resolved,
	// each in the interpreter's favor; empty under auto or a named engine.
	Disagreements []Disagreement
}

// Step is one engine's part in a plan: a refusal before running, the result it
// answered, the fault that stopped the plan, or its cancellation under all.
type Step struct {
	Engine string
	// Refusal is why Covers refused, nil when the engine ran.
	Refusal error
	// Result is what Run answered, nil when the engine refused, faulted or was cancelled.
	Result *Result
	// Err is the fault that stopped the plan at this engine: what Run reported, or the
	// context's error when it was met before the engine ran or before its answer was taken.
	Err error
	// Cancelled marks an engine the plan under all stopped before it answered: the one
	// that met the deadline, and every one behind it in name order.
	Cancelled bool
	// Bounds is the bound a cancelled engine reached: the plan's deadline, when one was set.
	Bounds Bounds
}

// Disagreement is two engines contradicting each other about one question: a
// witnessed existential claim against a universal claim it refutes. The
// interpreter is normative, so the witness stands and the other result is
// demoted to not covered with the disagreement as its reason.
type Disagreement struct {
	// Stands names the engine whose witnessed result the composition kept.
	Stands string
	// Demoted names the engine whose result was demoted, and Claimed is that result as it answered.
	Demoted string
	Claimed Result
	// Reason is the disagreement as the demoted result's Reason spells it.
	Reason string
}

// Refusals is every pre-run refusal in the plan, in step order.
func (p Plan) Refusals() []error {
	var refusals []error
	for _, step := range p.Steps {
		if step.Refusal != nil {
			refusals = append(refusals, step.Refusal)
		}
	}
	return refusals
}

// Refused is the typed error for a plan no engine ran: every refusal it met.
// It is nil when some engine ran, whatever it answered.
func (p Plan) Refused() error {
	for _, step := range p.Steps {
		if step.Refusal == nil {
			return nil
		}
	}
	return &RefusedError{Kind: p.Question.Kind, Refusals: p.Refusals()}
}

// Results is every result the plan's engines answered, in step order, a
// demoted one as demoted; nil when no engine ran.
func (p Plan) Results() []Result {
	var results []Result
	for _, step := range p.Steps {
		if step.Result != nil {
			results = append(results, *step.Result)
		}
	}
	return results
}

// RefusedError reports a question every engine declaring its kind refused.
type RefusedError struct {
	Kind     Kind
	Refusals []error
}

// Error is the one refusal's text when one engine refused, else every refusal
// named by its engine.
func (e *RefusedError) Error() string {
	if len(e.Refusals) == 1 {
		return e.Refusals[0].Error()
	}
	parts := make([]string, len(e.Refusals))
	for i, refusal := range e.Refusals {
		parts[i] = refusal.Error()
	}
	return fmt.Sprintf("no engine answers %s: %s", e.Kind, strings.Join(parts, "; "))
}

// Unwrap exposes each refusal to errors.Is and errors.As.
func (e *RefusedError) Unwrap() []error { return e.Refusals }

// ErrOverclaim is the typed error for an engine answering above its declared authority.
var ErrOverclaim = errors.New("engine answered above its authority")

// OverclaimError reports a result whose strength exceeds what its engine's
// Description says it can earn: a defect in the engine, never a stronger answer.
type OverclaimError struct {
	Engine    string
	Claimed   Strength
	Authority Strength
}

// Error names the engine and the two strengths.
func (e *OverclaimError) Error() string {
	return fmt.Sprintf("analysis: %s answered %s, above the %s it can earn", e.Engine, e.Claimed, e.Authority)
}

// Is matches ErrOverclaim.
func (e *OverclaimError) Is(target error) bool { return target == ErrOverclaim }

// Answer answers q under `auto`: engines of q's kind strongest first, a refusal or a
// not-covered result advancing (and kept in the plan), an error from Run stopping the plan.
// A Deadline in the budget bounds the plan through ctx; meeting it is an error like any other,
// checked before each engine is consulted and before an answer is taken from it. The plan
// works on a copy of model, so its worker is its own and model is never written.
func (r *Registry) Answer(ctx context.Context, model *Model, q Question, budget Budget) (Plan, error) {
	return r.AnswerWith(ctx, model, q, budget, Auto())
}

// AnswerWith answers q under the selection: as Answer does under auto; putting it to one
// engine alone under a named selection, whose refusal or not-covered result is the result;
// and under all to every covering engine one after another in name order, composing what
// they answered. Under all a deadline met cancels the engine that met it and every one
// behind it, each named in the plan with the bound it reached, and the finished engines'
// results stand composed; only a plan no engine finished fails with the deadline. The plan
// works on a copy of model, so its worker is its own and model is never written.
func (r *Registry) AnswerWith(ctx context.Context, model *Model, q Question, budget Budget, selection Selection) (Plan, error) {
	if !budget.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, budget.Deadline)
		defer cancel()
	}
	held := model.plan()
	held.compute(r.newToolRunner(ctx, held, budget, selection))
	defer held.release()
	return r.answer(ctx, held, q, budget, selection)
}

// answer answers q on the plan's copy of the model, whose tool runner puts every
// tool-computed performance of its runs back through here as a Compute question.
func (r *Registry) answer(ctx context.Context, held *Model, q Question, budget Budget, selection Selection) (Plan, error) {
	plan := Plan{Question: q, Selection: selection}
	candidates, err := r.candidates(q.Kind, selection)
	if err != nil {
		return plan, err
	}
	if selection.Mode == SelectAll {
		return r.answerAll(ctx, held, q, budget, plan, candidates)
	}
	var last *Result
	for _, e := range candidates {
		if err := ctx.Err(); err != nil {
			plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Err: err})
			return plan, err
		}
		coverage := e.Covers(held, q)
		if !coverage.Covered {
			plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Refusal: coverage.Refusal})
			continue
		}
		result, err := run(ctx, e, held, q, budget)
		if err != nil {
			plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Err: err})
			return plan, err
		}
		plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Result: &result})
		if result.Covered() {
			plan.Result = result
			return plan, nil
		}
		last = &result
	}
	plan.Result = uncovered(q, plan.Steps, last)
	return plan, nil
}

// answerAll runs every candidate in turn on the plan's copy of the model and
// composes the finished results.
func (r *Registry) answerAll(ctx context.Context, model *Model, q Question, budget Budget, plan Plan, candidates []Engine) (Plan, error) {
	started := time.Now()
	var finished []Result
	var last *Result
	for i, e := range candidates {
		if err := ctx.Err(); err != nil {
			plan.Steps = append(plan.Steps, cancelled(candidates[i:], err, budget, started)...)
			break
		}
		coverage := e.Covers(model, q)
		if !coverage.Covered {
			plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Refusal: coverage.Refusal})
			continue
		}
		result, err := run(ctx, e, model, q, budget)
		if err != nil {
			if ctx.Err() == nil {
				plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Err: err})
				return plan, err
			}
			plan.Steps = append(plan.Steps, cancelled(candidates[i:], err, budget, started)...)
			break
		}
		plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Result: &result})
		if result.Covered() {
			finished = append(finished, result)
		} else {
			last = &result
		}
	}
	if len(finished) == 0 {
		if last == nil && ctx.Err() != nil {
			return plan, ctx.Err()
		}
		plan.Result = uncovered(q, plan.Steps, last)
		return plan, nil
	}
	plan.Result, plan.Disagreements = Compose(finished)
	for _, d := range plan.Disagreements {
		plan.demote(d)
	}
	return plan, nil
}

// demote replaces the demoted engine's step result with the not-covered one
// the disagreement leaves it; the plan keeps what it claimed in the disagreement.
func (p *Plan) demote(d Disagreement) {
	for i := range p.Steps {
		if p.Steps[i].Engine == d.Demoted && p.Steps[i].Result != nil {
			demoted := demotedResult(d)
			p.Steps[i].Result = &demoted
		}
	}
}

// run takes one engine's answer, checked against the context and the scale: the
// claim and strength must agree, and a universal claim may not exceed the
// engine's authority, so no engine promotes what it earned. The plan's workers
// and their warming so far are recorded on the result.
func run(ctx context.Context, e Engine, model *Model, q Question, budget Budget) (Result, error) {
	result, err := e.Run(ctx, model, q, budget)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return Result{}, err
	}
	if result.Engine == "" {
		result.Engine = e.Name()
	}
	result.Workers, result.Warming = model.warmed()
	if Consistent(result.Claim, result.Strength) != nil {
		return Result{}, &InconsistentResultError{Engine: e.Name(), Claim: result.Claim, Strength: result.Strength}
	}
	if authority := e.Describe().Authority; result.Claim.Universal() && result.Strength > authority {
		return Result{}, &OverclaimError{Engine: e.Name(), Claimed: result.Strength, Authority: authority}
	}
	return result, nil
}

// cancelled is the steps of the engines a done context stopped: each marked
// cancelled with the context's error and the deadline it reached, when one was set.
func cancelled(engines []Engine, err error, budget Budget, started time.Time) []Step {
	var bounds Bounds
	if !budget.Deadline.IsZero() {
		bounds = Bounds{{Name: "deadline", Limit: budget.Deadline.Sub(started).Milliseconds(), Reached: true}}
	}
	steps := make([]Step, len(engines))
	for i, e := range engines {
		steps[i] = Step{Engine: e.Name(), Err: err, Cancelled: true, Bounds: bounds}
	}
	return steps
}

// uncovered is the result of a plan no engine covered: the last not-covered
// result, or one naming every refusal.
func uncovered(q Question, steps []Step, last *Result) Result {
	if last != nil {
		return *last
	}
	return Result{Question: q, Strength: NotCovered, Reason: refusalReason(steps)}
}

// candidates is the engines a selection puts a kind of question to, in the order
// they are consulted: the named one alone, or every engine declaring the kind,
// ranked by authority under auto and by name under all.
func (r *Registry) candidates(kind Kind, selection Selection) ([]Engine, error) {
	switch selection.Mode {
	case SelectNamed:
		e, ok := r.engines[selection.Engine]
		if !ok {
			return nil, &UnknownEngineError{Name: selection.Engine, Known: r.Names()}
		}
		return []Engine{e}, nil
	case SelectAll:
		engines := r.declaring(kind)
		if len(engines) == 0 {
			return nil, &NoEngineError{Kind: kind}
		}
		return engines, nil
	}
	engines := r.ranked(kind)
	if len(engines) == 0 {
		return nil, &NoEngineError{Kind: kind}
	}
	return engines, nil
}

// declaring is every engine declaring the kind, in name order.
func (r *Registry) declaring(kind Kind) []Engine {
	var engines []Engine
	for _, e := range r.Engines() {
		if e.Describe().Answers(kind) {
			engines = append(engines, e)
		}
	}
	return engines
}

// ranked is every engine declaring the kind, strongest authority first and
// name order within one authority.
func (r *Registry) ranked(kind Kind) []Engine {
	engines := r.declaring(kind)
	sort.SliceStable(engines, func(i, j int) bool {
		return engines[i].Describe().Authority > engines[j].Describe().Authority
	})
	return engines
}

// refusalReason names every refusal, in plan order.
func refusalReason(steps []Step) string {
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		if step.Refusal != nil {
			parts = append(parts, step.Engine+" refused: "+step.Refusal.Error())
		}
	}
	return strings.Join(parts, "; ")
}
