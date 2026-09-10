package analysis

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Plan is how a question was answered: every engine consulted, in the order
// their authority ranks them, and the answer that stood.
type Plan struct {
	Question Question
	Steps    []Step
	// Result is the first covered answer; when no engine covered the question
	// it is the last not-covered result, or one naming every refusal.
	Result Result
}

// Step is one engine's part in a plan: a refusal before running, the result it
// answered, or the fault that stopped the plan.
type Step struct {
	Engine string
	// Refusal is why Covers refused, nil when the engine ran.
	Refusal error
	// Result is what Run answered, nil when the engine refused or faulted.
	Result *Result
	// Err is the fault Run reported, which stopped the plan.
	Err error
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

// Answer answers q under `auto`: engines of q's kind strongest first, a refusal or a
// not-covered result advancing (and kept in the plan), an error from Run stopping the plan.
func (r *Registry) Answer(ctx context.Context, model *Model, q Question, budget Budget) (Plan, error) {
	candidates := r.ranked(q.Kind)
	if len(candidates) == 0 {
		return Plan{Question: q}, &NoEngineError{Kind: q.Kind}
	}
	plan := Plan{Question: q}
	var last *Result
	for _, e := range candidates {
		coverage := e.Covers(model, q)
		if !coverage.Covered {
			plan.Steps = append(plan.Steps, Step{Engine: e.Name(), Refusal: coverage.Refusal})
			continue
		}
		result, err := e.Run(ctx, model, q, budget)
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
	if last != nil {
		plan.Result = *last
		return plan, nil
	}
	plan.Result = Result{Question: q, Strength: NotCovered, Reason: refusalReason(plan.Steps)}
	return plan, nil
}

// ranked is every engine declaring the kind, strongest authority first and
// name order within one authority.
func (r *Registry) ranked(kind Kind) []Engine {
	var engines []Engine
	for _, e := range r.Engines() {
		if e.Describe().Answers(kind) {
			engines = append(engines, e)
		}
	}
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
