package runtime

import (
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Outcome is what one run came to, in the observables a conformance case
// compares; a run that failed is an outcome of its own, Err.
type Outcome struct {
	Outputs     map[string]Value
	FinalState  string
	StateVisits []string
	Err         error
	// ctx is the run's context, where the objects the outputs hold live.
	ctx *Context
}

// OutcomeOutput is one output of an outcome as it renders: its name and its
// value spelled canonically, an object by its type and what it holds.
type OutcomeOutput struct {
	Name string
	Text string
}

// Context is the context of the run that came to this outcome, nil for an
// outcome built without one.
func (o Outcome) Context() *Context { return o.ctx }

// String renders the outcome canonically: two runs agreeing on their observables
// render alike, so the rendering is the outcome's identity. An object is spelled
// by its type and what its features hold, never by the id its run gave it.
func (o Outcome) String() string {
	if o.Err != nil {
		return "error: " + o.Err.Error()
	}
	var parts []string
	if o.FinalState != "" {
		parts = append(parts, "finalState "+o.FinalState)
	}
	if len(o.StateVisits) > 0 {
		parts = append(parts, "visits "+strings.Join(o.StateVisits, ", "))
	}
	for _, out := range o.RenderedOutputs() {
		parts = append(parts, out.Name+" = "+out.Text)
	}
	if len(parts) == 0 {
		return "no outputs"
	}
	return strings.Join(parts, "; ")
}

// RenderedOutputs spells the outputs in name order; an object two outputs hold
// is spelled once, the second mention being its number in the outcome. A graph
// is opened to the bounds the wire serializes one to.
func (o Outcome) RenderedOutputs() []OutcomeOutput {
	return o.outputsSpelled(func(name string) string { return name }, true)
}

// identity is the outcome's identity: its rendering with every name quoted, so a
// name spelling a delimiter cannot make two outcomes one or one outcome two, and
// every object opened, so two graphs are one identity only when they are alike.
func (o Outcome) identity() string {
	if o.Err != nil {
		return "error: " + strconv.Quote(o.Err.Error())
	}
	parts := []string{"finalState " + strconv.Quote(o.FinalState)}
	for _, visit := range o.StateVisits {
		parts = append(parts, "visit "+strconv.Quote(visit))
	}
	for _, out := range o.outputsSpelled(strconv.Quote, false) {
		parts = append(parts, out.Name+" = "+out.Text)
	}
	return strings.Join(parts, "; ")
}

// outputsSpelled spells the outputs in name order, every name and type name
// written by names, the object graph to the wire's bounds if bounded.
func (o Outcome) outputsSpelled(names func(string) string, bounded bool) []OutcomeOutput {
	sorted := make([]string, 0, len(o.Outputs))
	for name := range o.Outputs {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	speller := &objectSpeller{ctx: o.ctx, names: names, bounded: bounded, numbers: make(map[int64]int)}
	outputs := make([]OutcomeOutput, 0, len(sorted))
	for _, name := range sorted {
		outputs = append(outputs, OutcomeOutput{Name: names(name), Text: speller.value(o.Outputs[name], 0)})
	}
	return outputs
}

// ActionOutcome is the outcome of an action run: the values its features hold.
func (ctx *Context) ActionOutcome(outputs map[string]Value) Outcome {
	return Outcome{Outputs: outputs, ctx: ctx}
}

// Outcome is the outcome of the performance so far: the configuration the
// machine rests in, the states it entered and the values it holds.
func (e *StateExecutor) Outcome() Outcome {
	return Outcome{
		FinalState:  e.FinalStateName(),
		StateVisits: slices.Clone(e.stateVisits),
		Outputs:     e.StateData(),
		ctx:         e.ctx,
	}
}

// AnalysisOutcome is the outcome of a case's run: its outputs, and each verdict
// as a value named by the objective or assertion it decided.
func (ctx *Context) AnalysisOutcome(r AnalysisResult) Outcome {
	outputs := make(map[string]Value, len(r.Outputs)+len(r.Verdicts))
	for _, out := range r.Outputs {
		outputs[out.Name] = out.Value
	}
	for _, v := range r.Verdicts {
		text := v.Status.String()
		if v.Detail != "" {
			text += ": " + v.Detail
		}
		outputs[v.Kind+" "+v.Name] = NewStringValue(text)
	}
	return Outcome{Outputs: outputs, ctx: ctx}
}

// VerifiedOutcome is the outcome of a verification case's run: the case's
// outcome, and what each body answered as a value named by the case that ran.
func (ctx *Context) VerifiedOutcome(result AnalysisResult, verdicts []VerificationVerdict) Outcome {
	outcome := ctx.AnalysisOutcome(result)
	for _, v := range verdicts {
		text := string(v.Kind)
		if v.Detail != "" {
			text += ": " + v.Detail
		}
		outcome.Outputs["verdict "+v.Case] = NewStringValue(text)
	}
	return outcome
}

// A bounded speller opens an object graph to these bounds, as the wire
// serializes one: an object past them is named but not opened.
const (
	maxSpelledObjectDepth = 8
	maxSpelledObjects     = 1000
)

// objectSpeller spells the objects one outcome's outputs hold, numbering each by
// first mention so an object held twice, or reached again around a cycle, is
// spelled once and named by its number after.
type objectSpeller struct {
	ctx     *Context
	names   func(string) string
	bounded bool
	numbers map[int64]int
}

// value spells a value, opening the objects it holds to depth.
func (s *objectSpeller) value(v Value, depth int) string {
	switch v.Kind {
	case ValInstance:
		return s.object(v.Instance, depth)
	case ValVariant:
		variant := v.Variant()
		if variant == nil {
			return FormatValue(v)
		}
		if v.Instance != 0 {
			return s.names(variant.Name) + " " + s.object(v.Instance, depth)
		}
		return s.names(variant.Name)
	case ValSequence:
		if v.Sequence() == nil {
			return "[]"
		}
		return "[" + s.elements(v.Sequence().Elements(), depth) + "]"
	case ValSet:
		if v.Set() == nil {
			return "Set{}"
		}
		return "Set{" + s.elements(v.Set().Elements(), depth) + "}"
	case ValArray:
		return v.Array().Format(func(element Value) string { return s.value(element, depth) })
	}
	return FormatValue(v)
}

func (s *objectSpeller) elements(elements []Value, depth int) string {
	parts := make([]string, len(elements))
	for i, element := range elements {
		parts[i] = s.value(element, depth)
	}
	return strings.Join(parts, ", ")
}

// object spells an object as `Type#n{feature = value, …}` at its first mention
// and `#n` after; one no context resolves keeps the id its run gave it.
func (s *objectSpeller) object(id int64, depth int) string {
	if s.ctx == nil {
		return FormatValue(Value{Kind: ValInstance, Instance: id})
	}
	if s.ctx.HoldsNoValue(Value{Kind: ValInstance, Instance: id}) {
		return UnsetText
	}
	if n, seen := s.numbers[id]; seen {
		return "#" + strconv.Itoa(n)
	}
	n := len(s.numbers) + 1
	s.numbers[id] = n
	inst, ok := s.ctx.Instance(id)
	if !ok {
		return "<unknown object>#" + strconv.Itoa(n)
	}
	name := s.names(s.typeName(inst)) + "#" + strconv.Itoa(n)
	if s.bounded && (depth >= maxSpelledObjectDepth || n > maxSpelledObjects) {
		return name + "{…}"
	}
	return name + "{" + s.features(inst, depth+1) + "}"
}

// features spells what every feature of inst holds, in name order; reading one
// materializes what it holds, as every surface reads an object.
func (s *objectSpeller) features(inst *Instance, depth int) string {
	names := make([]string, 0, len(inst.FeatureValues))
	for name := range inst.FeatureValues {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, s.names(name)+" = "+s.feature(inst, name, depth))
	}
	return strings.Join(parts, ", ")
}

func (s *objectSpeller) feature(inst *Instance, name string, depth int) string {
	fv, err := inst.GetFeatureValue(s.ctx, name)
	if err != nil {
		return "<error: " + err.Error() + ">"
	}
	if !fv.Feature.Scalar() {
		if fv.Values.Kind == ValInvalid {
			return "[]"
		}
		return s.value(fv.Values, depth)
	}
	if !fv.Materialized || fv.Value.Kind == ValInvalid {
		return UnsetText
	}
	return s.value(fv.Value, depth)
}

// typeName is the qualified name of the object's type, as the wire names it.
func (s *objectSpeller) typeName(inst *Instance) string {
	if inst.Type == nil {
		return "object"
	}
	if fqn := s.ctx.fqnOf(inst.Type); fqn != "" {
		return fqn
	}
	return inst.Type.Name
}
