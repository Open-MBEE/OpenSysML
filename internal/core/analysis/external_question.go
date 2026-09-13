package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/enginewire"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// coversParams is the covers request: the question and the model in the entry's forms.
func (e externalEngine) coversParams(model *Model, q Question) (enginewire.CoversParams, error) {
	question, err := e.wireQuestion(model, q)
	if err != nil {
		return enginewire.CoversParams{}, err
	}
	forms, err := e.wireModel(model, q)
	if err != nil {
		return enginewire.CoversParams{}, err
	}
	return enginewire.CoversParams{Question: question, Model: forms}, nil
}

// runParams is the run request: the covers request's members, the bounds the question fixes
// and the budget with its deadline as an absolute time.
func (e externalEngine) runParams(model *Model, q Question, budget Budget) (enginewire.RunParams, error) {
	covers, err := e.coversParams(model, q)
	if err != nil {
		return enginewire.RunParams{}, err
	}
	return enginewire.RunParams{
		Question: covers.Question,
		Model:    covers.Model,
		Bounds:   wireBounds(e.entry.Bounds, budget),
		Budget:   wireBudget(budget),
	}, nil
}

// wireBounds fixes the bounds the entry declares it takes at the budget's limits, in the
// manifest's order; a limit of 0 leaves that bound open.
func wireBounds(names []string, b Budget) []enginewire.Bound {
	out := []enginewire.Bound{}
	for _, name := range names {
		var limit int64
		switch name {
		case "depth":
			limit = int64(b.Depth)
		case "steps":
			limit = int64(b.Steps)
		case "runs":
			limit = int64(b.Runs)
		case "memory":
			limit = int64(b.Memory)
		}
		out = append(out, enginewire.Bound{Name: name, Limit: limit})
	}
	return out
}

// wireBudget is the budget as the protocol carries it.
func wireBudget(b Budget) enginewire.Budget {
	out := enginewire.Budget{
		Depth: int64(b.Depth), Steps: int64(b.Steps), Runs: int64(b.Runs),
		Memory: int64(b.Memory), Jobs: int64(b.Jobs),
	}
	if !b.Deadline.IsZero() {
		out.Deadline = b.Deadline.UTC().Format(time.RFC3339Nano)
	}
	return out
}

// wireQuestion is the question as the protocol carries it: its kind, subject as the surface
// spelled it with the kind of declaration it resolves to, the schedule, what is free, and
// the ask of its kind by name and text; no closure crosses the wire.
func (e externalEngine) wireQuestion(model *Model, q Question) (enginewire.Question, error) {
	out := enginewire.Question{
		Kind:     q.Kind.String(),
		Subject:  q.Subject,
		Schedule: q.Schedule.String(),
		Free:     freeNames(q.Free),
	}
	out.SubjectKind = subjectFamily(model, q.Subject)
	switch q.Kind {
	case Holds, Outcomes:
		if q.Check != nil {
			for _, p := range q.Check.Properties {
				out.Conditions = append(out.Conditions, enginewire.ConditionSet{
					Name:       p.Name,
					Features:   []enginewire.FreeInput{},
					Assertions: []enginewire.Condition{{Name: p.Name}},
				})
			}
			if len(q.Check.Properties) == 1 {
				out.Condition = &enginewire.Condition{Name: q.Check.Properties[0].Name}
			}
		}
	case Satisfiable:
		if q.Solve != nil {
			for _, query := range q.Solve.Queries {
				set, err := wireQuery(query)
				if err != nil {
					return enginewire.Question{}, err
				}
				out.Conditions = append(out.Conditions, set)
			}
		}
	case Sweep:
		if q.Sweep != nil {
			sweep, err := wireSweep(q.Sweep.Plan)
			if err != nil {
				return enginewire.Question{}, err
			}
			out.Sweep = sweep
		}
	case Compute:
		if q.Compute != nil && q.Compute.Call != nil {
			for _, in := range q.Compute.Call.Inputs {
				value, err := wireToolValue(in.Variable, in.Value)
				if err != nil {
					return enginewire.Question{}, err
				}
				out.Bindings = append(out.Bindings, value)
			}
		}
	}
	return out, nil
}

// freeNames spells what the question leaves free, in the protocol's words.
func freeNames(f Freedom) []string {
	names := []string{}
	if f.Has(FreeSchedule) {
		names = append(names, "schedule")
	}
	if f.Has(FreeInputs) {
		names = append(names, "inputs")
	}
	return names
}

// subjectFamily is the subject's declaration kind as a manifest's subjects spells it
// (action, state, calc, …), empty for a subject the model does not declare once.
func subjectFamily(model *Model, subject string) string {
	if sym := subjectOf(model, subject); sym != nil {
		return sym.Kind.Family()
	}
	return ""
}

// subjectOf is the one symbol the subject's qualified name resolves to in the model, nil
// when the model resolves none or more than one, or builds no semantics.
func subjectOf(model *Model, subject string) *symbols.Symbol {
	if subject == "" {
		return nil
	}
	semantics, err := model.semantics()
	if err != nil || semantics == nil || semantics.Resolver() == nil {
		return nil
	}
	found := semantics.Resolver().Index().LookupQualified(subject)
	if len(found) != 1 {
		return nil
	}
	return found[0]
}

// wireQuery is one query as a condition set: its free features with their sorts and
// units, its assertions as written, and its pinned values as the notation writes them.
func wireQuery(q *solve.Query) (enginewire.ConditionSet, error) {
	set := enginewire.ConditionSet{Name: q.Element, Features: []enginewire.FreeInput{}, Assertions: []enginewire.Condition{}}
	for _, v := range q.Free() {
		set.Features = append(set.Features, enginewire.FreeInput{Name: v.Name, Type: v.Sort.Name, Unit: v.Unit, Domain: v.Dimension})
	}
	for _, a := range q.Assertions {
		set.Assertions = append(set.Assertions, enginewire.Condition{Name: a.From.Element, Text: a.From.Condition})
	}
	for _, p := range q.Pinned {
		set.Pinned = append(set.Pinned, enginewire.Pinned{Name: p.Var.Name, Text: p.Value})
	}
	return set, nil
}

// wireSweep is a sweep's domain as the protocol carries it.
func wireSweep(plan runtime.SweepPlan) (*enginewire.Sweep, error) {
	out := &enginewire.Sweep{Ranges: []enginewire.Range{}, Sampled: plan.Sampled, Samples: plan.Samples, Seed: plan.Seed}
	for _, r := range plan.Ranges {
		from, err := wireValue(r.Param, r.From)
		if err != nil {
			return nil, err
		}
		to, err := wireValue(r.Param, r.To)
		if err != nil {
			return nil, err
		}
		wired := enginewire.Range{Parameter: r.Param, Type: sweepTypeName(r.Type), Unit: from.Unit, From: from.Value, To: to.Value}
		if r.HasStep {
			step, err := wireValue(r.Param, r.Step)
			if err != nil {
				return nil, err
			}
			wired.Step = step.Value
		}
		out.Ranges = append(out.Ranges, wired)
	}
	return out, nil
}

// sweepTypeName spells a swept parameter's type: its declared type's qualified name, else
// the numbers its range produces.
func sweepTypeName(t runtime.SweepType) string {
	if decl := t.Declared(); decl != nil {
		return symbols.FQNOf(decl)
	}
	switch t.Numbers {
	case runtime.SweepIntegers:
		return "Integer"
	case runtime.SweepReals:
		return "Real"
	}
	return ""
}

// wireValue is one runtime value as the protocol carries it, named; a value the protocol
// does not carry is a WireValueError.
func wireValue(name string, held runtime.Value) (enginewire.Value, error) {
	tool, ok := runtime.ToolValueOf(held)
	if !ok {
		return enginewire.Value{}, &WireValueError{Name: name, Held: runtime.FormatValue(held)}
	}
	return wireToolValue(name, tool)
}

// wireToolValue is one value of the tool protocol as the engine protocol carries it.
func wireToolValue(name string, tool runtime.ToolValue) (enginewire.Value, error) {
	raw, err := encodeValue(tool)
	if err != nil {
		return enginewire.Value{}, &WireValueError{Name: name, Held: err.Error()}
	}
	return enginewire.Value{Name: name, Value: raw, Unit: tool.Unit}, nil
}

// readValue is one value of the protocol as the tool protocol carries it.
func readValue(v enginewire.Value) (runtime.ToolValue, error) {
	raw := wiredValue{Value: v.Value}
	if v.Unit != "" {
		unit, err := json.Marshal(v.Unit)
		if err != nil {
			return runtime.ToolValue{}, err
		}
		raw.Unit = unit
	}
	return decodeValue(raw)
}

// ErrWireValue is the typed error for a value the protocol does not carry.
var ErrWireValue = errors.New("value is not carried by the engine protocol")

// WireValueError names a value the protocol does not carry.
type WireValueError struct {
	Name string
	Held string
}

// Error names the value and what it holds.
func (e *WireValueError) Error() string {
	return fmt.Sprintf("%s holds %s, which the engine protocol does not carry", e.Name, e.Held)
}

// Is matches ErrWireValue.
func (e *WireValueError) Is(target error) bool { return target == ErrWireValue }

// wireModel is the model in every form the entry asks for: sources always, graphs:1 of the
// subject when asked, and a typed refusal of a form or version this build does not export.
func (e externalEngine) wireModel(model *Model, q Question) (enginewire.Model, error) {
	semantics, err := model.semantics()
	if err != nil {
		return enginewire.Model{}, err
	}
	var out enginewire.Model
	for _, form := range e.entry.Forms() {
		switch {
		case form == FormSources:
			sources, err := export.SourcesOf(semantics)
			if err != nil {
				return enginewire.Model{}, err
			}
			out.Sources = sources
		case form == FormRDF:
			return enginewire.Model{}, export.RefuseRDFForm()
		case form == GraphsForm(export.GraphsVersion):
			subject := subjectOf(model, q.Subject)
			if subject == nil {
				return enginewire.Model{}, fmt.Errorf("%w: %q resolves to no one declaration", export.ErrGraphsSubject, q.Subject)
			}
			graphs, err := export.GraphsOf(semantics, subject)
			if err != nil {
				return enginewire.Model{}, err
			}
			raw, err := export.MarshalGraphs(graphs)
			if err != nil {
				return enginewire.Model{}, err
			}
			out.Graphs = json.RawMessage(raw)
		default:
			version, _ := form.GraphsVersion()
			return enginewire.Model{}, &export.FormUnsupportedError{Form: string(form),
				Reason: "this build exports graphs:" + strconv.Itoa(export.GraphsVersion) + ", not version " + strconv.Itoa(version)}
		}
	}
	return out, nil
}
