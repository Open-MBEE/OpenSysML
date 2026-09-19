package fuml

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Execution is what running an emitted activity under explore scheduling found,
// held against the reference implementation's record of the same activity.
type Execution struct {
	// Expected are the output parameters as the implementation left them, one
	// per line in parameter order; an absent one is `-`.
	Expected []string
	// Reached are the distinct output states the runs came to, sorted; each is
	// the parameters rendered as Expected is, joined by `; `.
	Reached []string
	// Disagreements are the reached output states that differ from Expected.
	Disagreements []string
	// Errors are the distinct errors runs ended in, sorted; any makes the test fail.
	Errors []string
	// Fired is the advisory comparison of which value-producing action nodes the
	// implementation fired against which produced a value here; "" when it agrees.
	Fired string
	// Runs is how many linearizations were run; Status is how exploration ended.
	Runs     int
	Status   string
	Complete bool
}

// Passed reports whether every run ended with the implementation's outputs.
// Exploration within budget need not be complete: each run is a legal schedule,
// and every one must agree.
func (x *Execution) Passed() bool {
	return len(x.Disagreements) == 0 && len(x.Errors) == 0 && len(x.Reached) > 0
}

// Reasons names why the execution failed, one line each; none for a pass.
func (x *Execution) Reasons() []string {
	var reasons []string
	for _, e := range x.Errors {
		reasons = append(reasons, "run error: "+e)
	}
	for _, d := range x.Disagreements {
		reasons = append(reasons, "outputs differ: "+d)
	}
	if len(x.Reached) == 0 && len(x.Errors) == 0 {
		reasons = append(reasons, "no run completed")
	}
	return reasons
}

// Execute runs an emitted activity under explore scheduling with the
// implementation's default input values, and compares what every run left in
// the output parameters with the implementation's record.
func Execute(stop context.Context, em *Emitted, x *ExpectedActivity, budget runtime.ExploreBudget, jobs int) (*Execution, error) {
	budgets, err := runBudgets()
	if err != nil {
		return nil, err
	}
	action, classes, fresh, err := build(em, budgets)
	if err != nil {
		return nil, err
	}
	if _, err := defaultInputs(nil, em, classes); err != nil {
		return nil, err
	}
	policy, err := runtime.ExplorePolicy(budget)
	if err != nil {
		return nil, err
	}
	run := func(ctx *runtime.Context) (runtime.Outcome, error) {
		inputs, err := defaultInputs(ctx, em, classes)
		if err != nil {
			return runtime.Outcome{}, err
		}
		outputs, err := ctx.ExecuteActionWithInputs(action, inputs)
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
	exploration, err := runtime.ExploreWith(stop, policy, jobs, fresh, run)
	if err != nil {
		return nil, err
	}
	return compare(em, exploration, x), nil
}

// runBudgets is the budgets bounding each run: the environment's where set,
// the runtime's defaults otherwise.
func runBudgets() (runtime.Budgets, error) {
	return runtime.BudgetsFromEnv()
}

// classLookup resolves a class of the model to the definition the emitted model
// declares for it.
type classLookup func(c *Class) (*symbols.Symbol, error)

// build parses the emitted model, resolves its action definition and the
// classes' definitions, and prepares a fresh context per exploration job.
func build(em *Emitted, budgets runtime.Budgets) (*symbols.Symbol, classLookup, func(int) (*runtime.Context, error), error) {
	src := source.New(em.Name, []byte(em.Text))
	p := parser.New(src)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		return nil, nil, nil, fmt.Errorf("%s: parse: %s", em.Name, d.Message)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(em.Name, file)
	idx.ExpandWildcardImports()
	lookup := func(qualified string) (*symbols.Symbol, error) {
		matches := idx.LookupQualified(qualified)
		if len(matches) != 1 {
			return nil, fmt.Errorf("%s: %d symbols named %s, want 1", em.Name, len(matches), qualified)
		}
		return matches[0], nil
	}
	action, err := lookup(em.Qualified)
	if err != nil {
		return nil, nil, nil, err
	}
	classes := func(c *Class) (*symbols.Symbol, error) { return lookup(Package + "::" + c.Name) }
	text := source.TextOf(map[string]*source.SourceFile{em.Name: src}, nil)
	var mu sync.Mutex
	models := map[int]*runtime.Model{}
	fresh := func(job int) (*runtime.Context, error) {
		mu.Lock()
		defer mu.Unlock()
		model, ok := models[job]
		if !ok {
			resolver := resolve.New(idx)
			sem := passes.NewTypedModel(resolver)
			sem.SetSourceText(text)
			model = runtime.NewModel(sem, resolver)
			model.SetExpressionParser(parser.ParseOneExpression)
			models[job] = model
		}
		ctx := runtime.NewContext(model, budgets.MaxSteps)
		if err := ctx.SetBudgets(budgets); err != nil {
			return nil, err
		}
		return ctx, nil
	}
	return action, classes, fresh, nil
}

// defaultInputs is what the implementation passes for each input parameter when
// a test runs an activity: the type's default value (0, false, "", 0.0), and for
// a class a fresh object of it whose every attribute holds its type's default.
// The objects live in ctx; with none, only whether the defaults exist is checked.
func defaultInputs(ctx *runtime.Context, em *Emitted, classes classLookup) (map[string]runtime.Value, error) {
	a := em.Activity
	inputs := map[string]runtime.Value{}
	for _, p := range a.Inputs() {
		v, err := defaultValue(ctx, em, classes, p.Type, "parameter "+p.Name, nil)
		if err != nil {
			return nil, err
		}
		inputs[p.Name] = v
	}
	return inputs, nil
}

// defaultValue is the implementation's default value of a type; an untyped
// attribute defaults as a String does. making holds the classes whose object
// is under construction, so a class holding one of itself is refused.
func defaultValue(ctx *runtime.Context, em *Emitted, classes classLookup, t TypeRef, where string, making map[*Class]bool) (runtime.Value, error) {
	a := em.Activity
	switch t.Name {
	case "Integer":
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt}}, nil
	case "Boolean":
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool}}, nil
	case "Real":
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal}}, nil
	case "String":
		return runtime.NewStringValue(""), nil
	}
	if t.Zero() {
		return runtime.NewStringValue(""), nil
	}
	c := a.Model.ClassOf(t)
	if c == nil {
		return runtime.Value{}, &TranslateError{a.Name, where, "has no default value of type " + t.String()}
	}
	if making[c] {
		return runtime.Value{}, &TranslateError{a.Name, where, "defaults a " + c.Name + " holding a " + c.Name + " without end"}
	}
	making = withClass(making, c)
	attrs := c.AllAttributes()
	defaults := make([]runtime.Value, len(attrs))
	for i, attr := range attrs {
		at := where + ", attribute " + c.Name + "." + attr.Name
		if attr.Association != nil {
			return runtime.Value{}, &TranslateError{a.Name, at, "an association end is not translated by the pilot emitter"}
		}
		v, err := defaultValue(ctx, em, classes, attr.Type, at, making)
		if err != nil {
			return runtime.Value{}, err
		}
		defaults[i] = v
	}
	if ctx == nil {
		return runtime.Value{}, nil
	}
	sym, err := classes(c)
	if err != nil {
		return runtime.Value{}, err
	}
	inst, err := ctx.InstantiateRead(sym, func(inst *runtime.Instance) error {
		for i, attr := range attrs {
			if err := inst.SetFeatureValue(ctx, attr.Name, defaults[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return runtime.Value{}, err
	}
	return runtime.Value{Kind: runtime.ValInstance, Instance: inst.ID}, nil
}

// withClass is making with c added, leaving making as it was.
func withClass(making map[*Class]bool, c *Class) map[*Class]bool {
	out := make(map[*Class]bool, len(making)+1)
	for k := range making {
		out[k] = true
	}
	out[c] = true
	return out
}

// producingNodes names the activity's own action nodes that hold a value at an
// output pin in a run's outputs.
func producingNodes(em *Emitted, outputs map[string]runtime.Value) []string {
	var nodes []string
	for name, keys := range em.Produces {
		for _, key := range keys {
			if v, ok := outputs[key]; ok && v.Kind != runtime.ValNull {
				nodes = append(nodes, name)
				break
			}
		}
	}
	return nodes
}

// compare holds the exploration's outcomes against the implementation's record;
// only the outcomes the exploration kept count, never a discarded speculative run.
func compare(em *Emitted, x *runtime.Exploration, expected *ExpectedActivity) *Execution {
	a := em.Activity
	want := renderExpected(a, expected)
	reached := map[string]bool{}
	errs := map[string]bool{}
	produced := map[string]bool{}
	for _, o := range x.Outcomes {
		if o.Outcome.Err != nil {
			errs[o.Outcome.Err.Error()] = true
			continue
		}
		reached[renderOutputs(a, o.Outcome.Context(), o.Outcome.Outputs)] = true
		for _, n := range producingNodes(em, o.Outcome.Outputs) {
			produced[n] = true
		}
	}
	ex := &Execution{
		Expected: strings.Split(want, "\n"),
		Errors:   sortedKeys(errs),
		Fired:    firedAgreement(a, expected, produced),
		Runs:     x.Runs,
		Status:   x.Status(),
		Complete: x.Complete(),
	}
	if want == "" {
		ex.Expected = nil
	}
	for _, r := range sortedKeys(reached) {
		line := strings.ReplaceAll(r, "\n", "; ")
		ex.Reached = append(ex.Reached, line)
		if r != want {
			ex.Disagreements = append(ex.Disagreements, line)
		}
	}
	return ex
}

// renderExpected spells the implementation's outputs one parameter per line,
// in the activity's parameter order.
func renderExpected(a *Activity, x *ExpectedActivity) string {
	byName := map[string]ExpectedOutput{}
	if x != nil {
		for _, o := range x.Outputs {
			byName[o.Parameter] = o
		}
	}
	r := &renderer{model: a.Model, numbers: map[string]int{}}
	var lines []string
	for _, p := range a.Outputs() {
		lines = append(lines, renderLine(p.Name, p.Multiplicity, r.expectedAll(p.Multiplicity, byName[p.Name].Values)))
	}
	return strings.Join(lines, "\n")
}

// renderOutputs spells a run's output parameters as renderExpected does; the
// objects they hold live in ctx.
func renderOutputs(a *Activity, ctx *runtime.Context, outputs map[string]runtime.Value) string {
	r := &renderer{model: a.Model, ctx: ctx, numbers: map[string]int{}}
	var lines []string
	for _, p := range a.Outputs() {
		var values []string
		if v, ok := outputs[p.Name]; ok {
			values = r.runtimeAll(p.Multiplicity, v)
		}
		lines = append(lines, renderLine(p.Name, p.Multiplicity, values))
	}
	return strings.Join(lines, "\n")
}

// renderLine spells one feature's values: absent as `-`, a multi-valued
// unordered feature's sorted so that runs agreeing as multisets render alike.
func renderLine(name string, m Multiplicity, values []string) string {
	if len(values) == 0 {
		return name + " = -"
	}
	if unordered(m) {
		sort.Strings(values)
	}
	return name + " = " + strings.Join(values, ", ")
}

// unordered reports whether a feature's values form a multiset rather than a list.
func unordered(m Multiplicity) bool {
	return !m.Ordered && m.Upper != 1
}

// renderer spells the implementation's values and the runtime's alike: a
// primitive canonically, an object as `Type#n{feature = values; …}` at its first
// mention and `#n` after, numbering objects by first mention so that neither
// side's object ids show. An object's features are those its class declares and
// inherits, in name order.
type renderer struct {
	model   *Model
	ctx     *runtime.Context
	numbers map[string]int
}

// expectedAll spells recorded values, an unordered feature's in canonical order.
func (r *renderer) expectedAll(m Multiplicity, values []ExpectedValue) []string {
	return r.canonical(m, len(values), func(r *renderer, i int) []string {
		return []string{r.expected(values[i])}
	})
}

// runtimeAll spells a run's value, an unordered sequence's elements in canonical order.
func (r *renderer) runtimeAll(m Multiplicity, v runtime.Value) []string {
	seq := v.Sequence()
	if seq == nil {
		return r.runtime(v)
	}
	elements := seq.Elements()
	return r.canonical(m, len(elements), func(r *renderer, i int) []string {
		return r.runtime(elements[i])
	})
}

// canonical spells n values, an unordered feature's in the order of their spellings
// from the numbers held so far, so equal multisets of objects number alike.
func (r *renderer) canonical(m Multiplicity, n int, spell func(*renderer, int) []string) []string {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	if unordered(m) && n > 1 {
		keys := make([]string, n)
		for i := range keys {
			scratch := &renderer{model: r.model, ctx: r.ctx, numbers: maps.Clone(r.numbers)}
			keys[i] = strings.Join(spell(scratch, i), ", ")
		}
		sort.SliceStable(order, func(a, b int) bool { return keys[order[a]] < keys[order[b]] })
	}
	var out []string
	for _, i := range order {
		out = append(out, spell(r, i)...)
	}
	return out
}

// expected spells a recorded value.
func (r *renderer) expected(v ExpectedValue) string {
	switch v.Kind {
	case "Integer", "Boolean", "String":
		var raw any
		if err := json.Unmarshal(v.Value, &raw); err == nil {
			switch raw := raw.(type) {
			case float64:
				return strconv.FormatInt(int64(raw), 10)
			case bool:
				return strconv.FormatBool(raw)
			case string:
				return strconv.Quote(raw)
			}
		}
	case "Real":
		var s string
		if err := json.Unmarshal(v.Value, &s); err == nil {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return renderReal(f)
			}
		}
	case "Reference":
		if v.Referent != nil {
			return r.expected(*v.Referent)
		}
	case "Object":
		return r.expectedObject(v)
	}
	return v.Kind + string(v.Value)
}

// expectedObject spells a recorded object by the class its first type names.
func (r *renderer) expectedObject(v ExpectedValue) string {
	if n, seen := r.numbers[v.ID]; seen {
		return "#" + strconv.Itoa(n)
	}
	n := len(r.numbers) + 1
	r.numbers[v.ID] = n
	typeName := strings.Join(v.Types, "&")
	held := map[string][]ExpectedValue{}
	for _, f := range v.Features {
		held[f.Feature] = f.Values
	}
	var c *Class
	if len(v.Types) == 1 {
		c = r.model.ClassOf(TypeRef{Name: v.Types[0]})
	}
	if c == nil {
		return typeName + "#" + strconv.Itoa(n) + "{?}"
	}
	var lines []string
	for _, attr := range sortedAttributes(c) {
		lines = append(lines, renderLine(attr.Name, attr.Multiplicity, r.expectedAll(attr.Multiplicity, held[attr.Name])))
	}
	return typeName + "#" + strconv.Itoa(n) + "{" + strings.Join(lines, "; ") + "}"
}

// runtime spells a run's value, a sequence as its elements.
func (r *renderer) runtime(v runtime.Value) []string {
	if seq := v.Sequence(); seq != nil {
		var out []string
		for _, e := range seq.Elements() {
			out = append(out, r.runtime(e)...)
		}
		return out
	}
	switch v.Kind {
	case runtime.ValNull:
		return nil
	case runtime.ValString:
		return []string{strconv.Quote(v.Str())}
	case runtime.ValConst:
		switch v.Const.Kind {
		case semantics.ValInt:
			return []string{strconv.FormatInt(v.Const.Int, 10)}
		case semantics.ValBool:
			return []string{strconv.FormatBool(v.Const.Bool)}
		case semantics.ValReal:
			return []string{renderReal(v.Const.Real)}
		}
	case runtime.ValInstance:
		// A valueless feature of a value type is materialized as an object that
		// is no value, which every surface of the runtime reads as unset.
		if r.ctx.HoldsNoValue(v) {
			return nil
		}
		return []string{r.runtimeObject(v.Instance)}
	}
	return []string{runtime.FormatValue(v)}
}

// runtimeObject spells a run's object by the class its definition translates.
func (r *renderer) runtimeObject(id int64) string {
	key := "#" + strconv.FormatInt(id, 10)
	if n, seen := r.numbers[key]; seen {
		return "#" + strconv.Itoa(n)
	}
	n := len(r.numbers) + 1
	r.numbers[key] = n
	inst, ok := r.ctx.Instance(id)
	if !ok || inst.Type == nil {
		return "<unknown object>#" + strconv.Itoa(n)
	}
	typeName := inst.Type.Name
	c := r.model.ClassOf(TypeRef{Name: typeName})
	if c == nil {
		return typeName + "#" + strconv.Itoa(n) + "{?}"
	}
	var lines []string
	for _, attr := range sortedAttributes(c) {
		fv, err := inst.GetFeatureValue(r.ctx, attr.Name)
		if err != nil {
			lines = append(lines, attr.Name+" = <error: "+err.Error()+">")
			continue
		}
		var values []string
		switch {
		case !fv.Feature.Scalar():
			if fv.Values.Kind != runtime.ValInvalid {
				values = r.runtimeAll(attr.Multiplicity, fv.Values)
			}
		case fv.Materialized && fv.Value.Kind != runtime.ValInvalid:
			values = r.runtimeAll(attr.Multiplicity, fv.Value)
		}
		lines = append(lines, renderLine(attr.Name, attr.Multiplicity, values))
	}
	return typeName + "#" + strconv.Itoa(n) + "{" + strings.Join(lines, "; ") + "}"
}

// sortedAttributes is the class's own and inherited attributes in name order.
func sortedAttributes(c *Class) []*Property {
	attrs := append([]*Property(nil), c.AllAttributes()...)
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Name < attrs[j].Name })
	return attrs
}

// renderReal spells a real so that the implementation's and the runtime's agree
// when they are the same number.
func renderReal(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Sprint(f)
	}
	return strconv.FormatFloat(f, 'g', 15, 64)
}

// firedAgreement compares, advisorily, the set of value-producing action nodes
// the implementation fired directly in the activity with the set that produced
// a value in some run here. The runtime does not expose a per-run firing
// sequence for nested flows, so the order is not compared.
func firedAgreement(a *Activity, x *ExpectedActivity, produced map[string]bool) string {
	if x == nil {
		return "no record"
	}
	fired := map[string]bool{}
	for _, e := range x.Events {
		if e.Kind == "Fire" && e.Activity == a.Name {
			if n := a.NodeNamed(e.Action); n != nil && n.Owner == nil && len(n.Outputs()) > 0 {
				fired[e.Action] = true
			}
		}
	}
	var only, missing []string
	for n := range fired {
		if !produced[n] {
			missing = append(missing, n)
		}
	}
	for n := range produced {
		if !fired[n] {
			only = append(only, n)
		}
	}
	sort.Strings(only)
	sort.Strings(missing)
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "fired by the implementation only: "+strings.Join(missing, ", "))
	}
	if len(only) > 0 {
		parts = append(parts, "produced a value here only: "+strings.Join(only, ", "))
	}
	return strings.Join(parts, "; ")
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
