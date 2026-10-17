package explore_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

// The expected-outcome schema of the runtime's conformance cases, as far as
// exploring a case over its admissible set reads it.

// ExpectedValue is a value a conformance case expects.
type ExpectedValue struct {
	Type       string          `json:"type"`
	Value      interface{}     `json:"value"`
	Unit       string          `json:"unit,omitempty"`
	Im         *float64        `json:"im,omitempty"`
	Text       string          `json:"text,omitempty"`
	Elements   []ExpectedValue `json:"elements,omitempty"`
	Dimensions []int64         `json:"dimensions,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// ExpectedEvent is an event a state case injects.
type ExpectedEvent struct {
	Signal  string                   `json:"signal,omitempty"`
	Call    string                   `json:"call,omitempty"`
	Args    map[string]ExpectedValue `json:"args,omitempty"`
	Value   *ExpectedValue           `json:"value,omitempty"`
	Results map[string]ExpectedValue `json:"results,omitempty"`
}

// AdmittedOutcome is one outcome a case admits.
type AdmittedOutcome struct {
	Outputs          map[string]ExpectedValue `json:"outputs,omitempty"`
	FinalState       string                   `json:"finalState,omitempty"`
	Terminated       bool                     `json:"terminated,omitempty"`
	StateVisits      []string                 `json:"stateVisits,omitempty"`
	Error            string                   `json:"error,omitempty"`
	Probability      *float64                 `json:"probability,omitempty"`
	ProbabilityRange *[2]float64              `json:"probabilityRange,omitempty"`
}

// ExpectedExploreBudget is the exploreBudget of a case.
type ExpectedExploreBudget struct {
	Runs  *int `json:"runs,omitempty"`
	Depth *int `json:"depth,omitempty"`
}

// budget is the case's budget with the defaults filled in.
func (b *ExpectedExploreBudget) budget() runtime.ExploreBudget {
	budget := runtime.DefaultExploreBudget
	if b == nil {
		return budget
	}
	if b.Runs != nil {
		budget.Runs = *b.Runs
	}
	if b.Depth != nil {
		budget.Depth = *b.Depth
	}
	return budget
}

// ExpectedOutcome is the part of a case's expectation that exploring it reads.
type ExpectedOutcome struct {
	Type          string                 `json:"type"`
	Libraries     bool                   `json:"libraries,omitempty"`
	Outcomes      []AdmittedOutcome      `json:"outcomes,omitempty"`
	Events        []ExpectedEvent        `json:"events,omitempty"`
	Evaluate      string                 `json:"evaluate,omitempty"`
	ExploreBudget *ExpectedExploreBudget `json:"exploreBudget,omitempty"`
}

func isConformanceCase(fileName string) bool {
	return fixtures.IsCase(fileName)
}

func loadKnownFailures(t *testing.T, conformanceDir string) map[string]bool {
	failures, err := fixtures.KnownFailures(conformanceDir)
	if err != nil {
		t.Logf("warning: failed to read known_failures.txt: %v", err)
		return nil
	}
	return failures
}

// conformanceRun is the case's behavior as a run an exploration repeats.
func conformanceRun(t *testing.T, idx *symbols.Index, path string, expected ExpectedOutcome) func(*runtime.Context) (runtime.Outcome, error) {
	t.Helper()
	rootScope := idx.DocumentRoot(path)
	switch expected.Type {
	case "action":
		actionSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefAction, ast.UsageAction)
		return func(ctx *runtime.Context) (runtime.Outcome, error) {
			outputs, err := ctx.ExecuteAction(actionSym)
			if err != nil {
				return runtime.Outcome{}, err
			}
			return ctx.ActionOutcome(outputs), nil
		}
	case "state":
		stateSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefState, ast.UsageState)
		events := queuedEvents(t, expected.Events)
		return func(ctx *runtime.Context) (runtime.Outcome, error) {
			exec, err := ctx.CreateStateExecutor(stateSym)
			if err != nil {
				return runtime.Outcome{}, err
			}
			for _, event := range events {
				if err := exec.Enqueue(event); err != nil {
					exec.Release()
					return runtime.Outcome{}, err
				}
			}
			if err := exec.RunToCompletion(); err != nil {
				exec.Release()
				return runtime.Outcome{}, err
			}
			return exec.Outcome(), nil
		}
	default:
		t.Fatalf("outcomes apply to action and state cases, not %q", expected.Type)
		return nil
	}
}

func expectedToRuntimeValue(t *testing.T, ev ExpectedValue) runtime.Value {
	switch ev.Type {
	case "Integer":
		n, ok := expectedInteger(ev.Value)
		if !ok {
			t.Fatalf("invalid Integer value %v (%T)", ev.Value, ev.Value)
		}
		return runtime.Value{Kind: runtime.ValConst, Const: n}
	case "Real":
		if v, ok := ev.Value.(float64); ok {
			return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: v}}
		}
		t.Fatalf("invalid Real value type: %T", ev.Value)
	case "Rational":
		if r, ok := expectedRational(ev.Value); ok {
			return runtime.Value{Kind: runtime.ValConst, Const: r}
		}
		t.Fatalf("invalid Rational value %v (%T)", ev.Value, ev.Value)
	case "Boolean":
		if v, ok := ev.Value.(bool); ok {
			return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: v}}
		}
		t.Fatalf("invalid Boolean value type: %T", ev.Value)
	case "String":
		if v, ok := ev.Value.(string); ok {
			return runtime.NewStringValue(v)
		}
		t.Fatalf("invalid String value type: %T", ev.Value)
	case "Null":
		return runtime.Value{Kind: runtime.ValNull}
	case "Infinity":
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}
	case "Variant":
		t.Fatalf("a variant is named by the model, so it cannot be built from a case value")
	case "EnumLiteral":
		t.Fatalf("an enumeration literal is declared by the model, so it cannot be built from a case value")
	case "MeasurementRef":
		t.Fatalf("a measurement reference names a unit the model declares, so it cannot be built from a case value")
	case "CoordinateFrame", "CoordinateTransformation":
		t.Fatalf("a %s is declared by the model, so it cannot be built from a case value", ev.Type)
	case "Function":
		t.Fatalf("a function is a calc the model declares, so it cannot be built from a case value")
	case "Metaobject":
		t.Fatalf("a metaobject denotes an element the model declares, so it cannot be built from a case value")
	case "Complex":
		v, ok := ev.Value.(float64)
		if !ok || ev.Im == nil {
			t.Fatalf("invalid Complex value: %v (im %v)", ev.Value, ev.Im)
		}
		return runtime.NewComplex(complex(v, *ev.Im))
	case "Quantity":
		v, ok := ev.Value.(float64)
		if !ok {
			t.Fatalf("invalid Quantity value type: %T", ev.Value)
		}
		return runtime.NewQuantityValue(&runtime.Quantity{
			Num:  semantics.Value{Kind: semantics.ValReal, Real: v},
			Unit: runtime.Unit{Text: ev.Unit},
		})
	default:
		t.Fatalf("unknown type: %s", ev.Type)
	}
	return runtime.Value{}
}
func expectedRational(v any) (semantics.Value, bool) {
	s, ok := v.(string)
	if !ok {
		return semantics.Value{}, false
	}
	r, err := semantics.ParseRationalText(s, semantics.DefaultMaxIntegerBits)
	return r, err == nil
}
func expectedInteger(v any) (semantics.Value, bool) {
	switch v := v.(type) {
	case float64:
		return semantics.IntegerOfReal(v)
	case string:
		return semantics.ParseInteger(v)
	}
	return semantics.Value{}, false
}
func queuedEvents(t *testing.T, events []ExpectedEvent) []runtime.QueuedEvent {
	t.Helper()
	queued := make([]runtime.QueuedEvent, 0, len(events))
	for _, event := range events {
		args := make(map[string]runtime.Value, len(event.Args))
		for name, val := range event.Args {
			args[name] = expectedToRuntimeValue(t, val)
		}
		q := runtime.QueuedEvent{Signal: event.Signal, Call: event.Call, Args: args}
		if event.Value != nil {
			value := expectedToRuntimeValue(t, *event.Value)
			q.Value = &value
		}
		queued = append(queued, q)
	}
	return queued
}
