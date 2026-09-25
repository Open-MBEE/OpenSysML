package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// sequenceModel is a tool-computed action answering multi-valued parameters: speeds is
// unbounded, few bounded, one single-valued.
const sequenceModel = `package test {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	private import ISQ::*;

	action def Profile {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in k : Real                  { @ToolVariable { name = "k"; } }
		out speeds : SpeedValue[0..*] { @ToolVariable { name = "speeds"; } }
	}
	action def RunProfile {
		out speeds : SpeedValue[0..*];
		action s : Profile { in k = 1.0; }
		bind speeds = s.speeds;
	}

	action def Few {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		out few : Real[2..4] { @ToolVariable { name = "few"; } }
	}
	action def RunFew {
		out few : Real[2..4];
		action s : Few {}
		bind few = s.few;
	}

	action def One {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		out one : Real { @ToolVariable { name = "one"; } }
	}
	action def RunOne {
		out one : Real;
		action s : One {}
		bind one = s.one;
	}
}`

func toolSeq(items ...ToolValue) ToolValue {
	if items == nil {
		items = make([]ToolValue, 0)
	}
	return ToolValue{Items: items}
}

// A sequence answered to a multi-valued parameter binds every item, each converted to the
// parameter's declared unit as a scalar answer is.
func TestToolOutputSequenceBindsEveryItem(t *testing.T) {
	ctx, scope := analysisFixture(t, sequenceModel)
	ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
		"speeds": {Unit: "km/h", Items: []ToolValue{
			{Value: toolReal(36)}, {Value: toolReal(72)},
		}},
	}})
	out, err := ctx.ExecuteAction(calcNamed(t, scope, "RunProfile"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := FormatValue(out["speeds"]); got != "[10.0 [SI::'m/s'], 20.0 [SI::'m/s']]" {
		t.Fatalf("speeds = %s, want the items converted to m/s", got)
	}
}

// An empty sequence binds nothing; the multi-valued parameter holds no value.
func TestToolOutputEmptySequence(t *testing.T) {
	ctx, scope := analysisFixture(t, sequenceModel)
	ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
		"speeds": {Items: make([]ToolValue, 0)},
	}})
	out, err := ctx.ExecuteAction(calcNamed(t, scope, "RunProfile"))
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := FormatValue(out["speeds"]); got != "[]" {
		t.Fatalf("speeds = %s, want []", got)
	}
}

// A bounded multiplicity admits what it spans and refuses what it does not, the refusal a
// malformed answer naming the bound.
func TestToolOutputSequenceHonorsTheBounds(t *testing.T) {
	run := func(t *testing.T, n int) error {
		ctx, scope := analysisFixture(t, sequenceModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"few": toolSeq(func() []ToolValue {
				items := make([]ToolValue, n)
				for i := range items {
					items[i] = ToolValue{Value: toolReal(float64(i))}
				}
				return items
			}()...),
		}})
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunFew"))
		return err
	}
	if err := run(t, 3); err != nil {
		t.Errorf("three items in [2..4]: %v", err)
	}
	for n, want := range map[int]string{5: "upper bound 4", 1: "lower bound 2"} {
		err := run(t, n)
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMalformed || !strings.Contains(failure.Detail, want) {
			t.Errorf("%d items = %v, want malformed naming %q", n, err, want)
		}
	}
}

// The two sides must agree on cardinality: a scalar is not a sequence, and a sequence is
// not one value.
func TestToolOutputCardinalityMustAgree(t *testing.T) {
	t.Run("scalar to a sequence", func(t *testing.T) {
		ctx, scope := analysisFixture(t, sequenceModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"few": {Value: toolReal(4)},
		}})
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunFew"))
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMalformed || !strings.Contains(failure.Detail, "holds a sequence") {
			t.Fatalf("ExecuteAction = %v, want malformed naming the sequence", err)
		}
	})
	t.Run("sequence to one value", func(t *testing.T) {
		ctx, scope := analysisFixture(t, sequenceModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"one": toolSeq(ToolValue{Value: toolReal(1)}, ToolValue{Value: toolReal(2)}),
		}})
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunOne"))
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMalformed || !strings.Contains(failure.Detail, "at most one") {
			t.Fatalf("ExecuteAction = %v, want malformed naming the single value", err)
		}
	})
}

// An item that cannot convert is the malformed element, named by its index.
func TestToolOutputSequenceBadItemNamesTheElement(t *testing.T) {
	ctx, scope := analysisFixture(t, sequenceModel)
	ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
		"speeds": {Unit: "km/h", Items: []ToolValue{
			{Value: toolReal(36)}, {Value: semantics.Value{Kind: semantics.ValBool, Bool: true}},
		}},
	}})
	_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunProfile"))
	var failure *ToolError
	if !errors.As(err, &failure) || failure.Kind != ToolMalformed || !strings.HasPrefix(failure.Detail, "speeds: element 1: ") {
		t.Fatalf("ExecuteAction = %v, want malformed naming element 1", err)
	}
}

// A sequence's elements are counted against the collection-element budget like any
// materialization: over the ceiling the run fails the element limit, not a malformed answer.
func TestToolOutputSequenceCountsAgainstTheElementBudget(t *testing.T) {
	ctx, scope := analysisFixture(t, sequenceModel)
	if err := ctx.SetBudgets(Budgets{
		MaxSteps: DefaultMaxSteps, MaxActionSteps: DefaultMaxActionSteps,
		MaxStateEvents: DefaultMaxStateEvents, MaxDoSteps: DefaultMaxDoSteps,
		MaxElements: 2, MaxCalcDepth: DefaultMaxCalcDepth, MaxSweepRuns: DefaultMaxSweepRuns,
	}); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
		"speeds": toolSeq(ToolValue{Value: toolReal(1)}, ToolValue{Value: toolReal(2)}, ToolValue{Value: toolReal(3)}),
	}})
	_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunProfile"))
	var failure *ToolError
	if errors.As(err, &failure) {
		t.Fatalf("ExecuteAction = %v, want the element limit, not a malformed answer", err)
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("ExecuteAction = %v, want ErrElementLimitExceeded", err)
	}
}

// An empty sequence answering a unit keeps the unit: it is measured in the parameter's
// coherent unit, the same as the elements a non-empty answer would carry.
func TestToolOutputEmptySequenceKeepsItsUnit(t *testing.T) {
	t.Run("measured", func(t *testing.T) {
		ctx, scope := analysisFixture(t, sequenceModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"speeds": {Unit: "km/h", Items: make([]ToolValue, 0)},
		}})
		out, err := ctx.ExecuteAction(calcNamed(t, scope, "RunProfile"))
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		unit, ok := out["speeds"].Sequence().ElementUnit()
		if !ok || unit.String() != "SI::'m/s'" {
			t.Fatalf("element unit = %v (%v), want SI::'m/s'", unit, ok)
		}
	})
	t.Run("measured but not a quantity", func(t *testing.T) {
		ctx, scope := analysisFixture(t, sequenceModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"few": {Unit: "K", Items: make([]ToolValue, 0)},
		}})
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunFew"))
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMalformed || !strings.Contains(failure.Detail, "is not a quantity to be measured in") {
			t.Fatalf("ExecuteAction = %v, want malformed naming a non-quantity parameter", err)
		}
	})
	t.Run("measured against another dimension", func(t *testing.T) {
		ctx, scope := analysisFixture(t, sequenceModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"speeds": {Unit: "K", Items: make([]ToolValue, 0)},
		}})
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunProfile"))
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMalformed || !strings.Contains(failure.Detail, "does not measure") {
			t.Fatalf("ExecuteAction = %v, want malformed naming the incommensurable unit", err)
		}
	})
	t.Run("an unknown unit", func(t *testing.T) {
		ctx, scope := analysisFixture(t, sequenceModel)
		ctx.SetToolRunner(&recordingRunner{answer: map[string]ToolValue{
			"speeds": {Unit: "furlongs/fortnight", Items: make([]ToolValue, 0)},
		}})
		_, err := ctx.ExecuteAction(calcNamed(t, scope, "RunProfile"))
		var failure *ToolError
		if !errors.As(err, &failure) || failure.Kind != ToolMalformed {
			t.Fatalf("ExecuteAction = %v, want malformed naming the unit", err)
		}
	})
}
