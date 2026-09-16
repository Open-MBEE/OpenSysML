package fuml

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
	action, fresh, err := build(em, budgets)
	if err != nil {
		return nil, err
	}
	inputs, err := defaultInputs(em.Activity)
	if err != nil {
		return nil, err
	}
	policy, err := runtime.ExplorePolicy(budget)
	if err != nil {
		return nil, err
	}
	run := func(ctx *runtime.Context) (runtime.Outcome, error) {
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

// build parses the emitted model, resolves its action definition and prepares
// a fresh context per exploration job.
func build(em *Emitted, budgets runtime.Budgets) (*symbols.Symbol, func(int) (*runtime.Context, error), error) {
	src := source.New(em.Name, []byte(em.Text))
	p := parser.New(src)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		return nil, nil, fmt.Errorf("%s: parse: %s", em.Name, d.Message)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(em.Name, file)
	idx.ExpandWildcardImports()
	matches := idx.LookupQualified(em.Qualified)
	if len(matches) != 1 {
		return nil, nil, fmt.Errorf("%s: %d symbols named %s, want 1", em.Name, len(matches), em.Qualified)
	}
	action := matches[0]
	text := source.TextOf(map[string]*source.SourceFile{em.Name: src}, nil)
	var mu sync.Mutex
	models := map[int]*runtime.Model{}
	fresh := func(job int) (*runtime.Context, error) {
		mu.Lock()
		defer mu.Unlock()
		model, ok := models[job]
		if !ok {
			resolver := resolve.New(idx)
			sem := semantics.NewModel(resolver)
			sem.SetSourceText(text)
			model = runtime.NewModel(sem, resolver)
			models[job] = model
		}
		ctx := runtime.NewContext(model, budgets.MaxSteps)
		if err := ctx.SetBudgets(budgets); err != nil {
			return nil, err
		}
		return ctx, nil
	}
	return action, fresh, nil
}

// defaultInputs is what the implementation passes for each input parameter when
// a test runs an activity: the type's default value (0, false, "", 0.0).
func defaultInputs(a *Activity) (map[string]runtime.Value, error) {
	inputs := map[string]runtime.Value{}
	for _, p := range a.Inputs() {
		switch p.Type.Name {
		case "Integer":
			inputs[p.Name] = runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt}}
		case "Boolean":
			inputs[p.Name] = runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool}}
		case "Real":
			inputs[p.Name] = runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal}}
		case "String":
			inputs[p.Name] = runtime.NewStringValue("")
		default:
			return nil, &TranslateError{a.Name, "parameter " + p.Name, "has no default value of type " + p.Type.String()}
		}
	}
	return inputs, nil
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
		reached[renderOutputs(a, o.Outcome.Outputs)] = true
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
	var lines []string
	for _, p := range a.Outputs() {
		var values []string
		for _, v := range byName[p.Name].Values {
			values = append(values, renderExpectedValue(v))
		}
		lines = append(lines, renderLine(p, values))
	}
	return strings.Join(lines, "\n")
}

// renderOutputs spells a run's output parameters as renderExpected does.
func renderOutputs(a *Activity, outputs map[string]runtime.Value) string {
	var lines []string
	for _, p := range a.Outputs() {
		var values []string
		if v, ok := outputs[p.Name]; ok {
			values = renderRuntimeValues(v)
		}
		lines = append(lines, renderLine(p, values))
	}
	return strings.Join(lines, "\n")
}

// renderLine spells one parameter's values: absent as `-`, a multi-valued
// unordered parameter's sorted so that runs agreeing as multisets render alike.
func renderLine(p *Parameter, values []string) string {
	if len(values) == 0 {
		return p.Name + " = -"
	}
	if !p.Multiplicity.Ordered && p.Multiplicity.Upper != 1 {
		sort.Strings(values)
	}
	return p.Name + " = " + strings.Join(values, ", ")
}

// renderExpectedValue spells a recorded primitive value canonically.
func renderExpectedValue(v ExpectedValue) string {
	switch v.Kind {
	case "Integer", "Boolean", "String":
		var raw any
		if err := json.Unmarshal(v.Value, &raw); err == nil {
			switch r := raw.(type) {
			case float64:
				return strconv.FormatInt(int64(r), 10)
			case bool:
				return strconv.FormatBool(r)
			case string:
				return strconv.Quote(r)
			}
		}
	case "Real":
		var s string
		if err := json.Unmarshal(v.Value, &s); err == nil {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return renderReal(f)
			}
		}
	}
	return v.Kind + string(v.Value)
}

// renderRuntimeValues spells a run's value, a sequence as its elements.
func renderRuntimeValues(v runtime.Value) []string {
	if seq := v.Sequence(); seq != nil {
		var out []string
		for _, e := range seq.Elements() {
			out = append(out, renderRuntimeValues(e)...)
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
	}
	return []string{runtime.FormatValue(v)}
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
