package queryexec

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// exhaustedEvidenceQuery makes the excluded holder's only numeric value cost
// more than the preview budget, leaving the text holder as the surviving row.
func exhaustedEvidenceQuery(terms int, projected bool) string {
	read := `source = Union(source = lazy, other = text)`
	if projected {
		read = `
			source = Project(
				source = WhereName(
					source = Union(source = lazy, other = text),
					operator = "=", value = "textHolder"
				),
				properties = ("annotations")
			)`
	} else {
		read = `
			source = WhereName(
				` + read + `,
				operator = "=", value = "textHolder"
			)`
	}
	return fmt.Sprintf(`
private import ScalarValues::*;
part def Excluded {
	attribute base : Integer = 1;
	attribute annotations : Integer = base%s;
}
part def TextHolder { attribute annotations : String = "text"; }
part excluded : Excluded;
part textHolder : TextHolder;
calc def NameThenFilter :> Query {
	in lazy : Element;
	in text : Element;
	WhereFeature(
		%s,
		'feature' = "annotations", operator = ">", value = "3"
	)
}
`, strings.Repeat("+1", terms), read)
}

func executeExhaustedEvidence(t *testing.T, body string, budget int64) (result *RowSet, err error) {
	t.Helper()
	fixture := loadExecutionFixture(t, body)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), budget)
	excluded, err := ctx.Instantiate(fixture.symbol(t, "excluded"))
	if err != nil {
		t.Fatalf("Instantiate excluded: %v", err)
	}
	textHolder, err := ctx.Instantiate(fixture.symbol(t, "textHolder"))
	if err != nil {
		t.Fatalf("Instantiate textHolder: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "excluded", Object: excluded}, {Label: "textHolder", Object: textHolder}},
	}
	bindings := Bindings{
		"lazy": {ObjectValue(excluded, "excluded")},
		"text": {ObjectValue(textHolder, "textHolder")},
	}
	return Execute(fixture.program(t, "NameThenFilter"), context, bindings, Options{})
}

// A read that exhausts the preview budget must report the budget, not blame the
// operator on the incompatible value that survived the earlier name filter.
func TestQueryReportsAnExhaustedEvidenceBudget(t *testing.T) {
	_, err := executeExhaustedEvidence(t, exhaustedEvidenceQuery(40, false), 80)
	if !errors.Is(err, runtime.ErrStepLimitExceeded) {
		t.Fatalf("an exhausted evidence read = %v, want %v", err, runtime.ErrStepLimitExceeded)
	}
}

// A projected column takes the same evidence budget and must preserve its
// typed refusal instead of turning it into an invalid comparison.
func TestQueryProjectedEvidenceReportsAnExhaustedBudget(t *testing.T) {
	_, err := executeExhaustedEvidence(t, exhaustedEvidenceQuery(40, true), 80)
	if !errors.Is(err, runtime.ErrStepLimitExceeded) {
		t.Fatalf("an exhausted projected evidence read = %v, want %v", err, runtime.ErrStepLimitExceeded)
	}
}

// An operand the comparison itself cannot parse stays an invalid operand even
// when an evidence preview elsewhere ran out of budget.
func TestQueryKeepsAnInvalidOperandWhenEvidenceIsUnavailable(t *testing.T) {
	body := strings.Replace(exhaustedEvidenceQuery(40, false), `value = "3"`, `value = "abc"`, 1)
	_, err := executeExhaustedEvidence(t, body, 80)
	failure := executionError(t, err, ErrorInvalidArgument)
	if failure.Parameter != "value" {
		t.Fatalf("invalid operand = %+v, want the value parameter", failure)
	}
}

// Evidence from another starting row settles the operand, so an exhausted
// preview elsewhere must not fail an otherwise valid comparison.
func TestQueryComparableEvidenceHidesAnExhaustedPreview(t *testing.T) {
	body := strings.Replace(exhaustedEvidenceQuery(40, false), `
part textHolder : TextHolder;`, `
part numericHolder : NumericHolder;
part textHolder : TextHolder;`, 1)
	body = strings.Replace(body, "part def TextHolder", "part def NumericHolder { attribute annotations : Integer = 5; }\npart def TextHolder", 1)
	body = strings.Replace(body, "source = Union(source = lazy, other = text)", "source = Union(source = Union(source = lazy, other = numeric), other = text)", 1)
	body = strings.Replace(body, "\tin text : Element;", "\tin numeric : Element;\n\tin text : Element;", 1)
	fixture := loadExecutionFixture(t, body)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), 80)
	excluded, err := ctx.Instantiate(fixture.symbol(t, "excluded"))
	if err != nil {
		t.Fatalf("Instantiate excluded: %v", err)
	}
	numeric, err := ctx.Instantiate(fixture.symbol(t, "numericHolder"))
	if err != nil {
		t.Fatalf("Instantiate numericHolder: %v", err)
	}
	textHolder, err := ctx.Instantiate(fixture.symbol(t, "textHolder"))
	if err != nil {
		t.Fatalf("Instantiate textHolder: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "excluded", Object: excluded}, {Label: "numericHolder", Object: numeric}, {Label: "textHolder", Object: textHolder}},
	}
	bindings := Bindings{
		"lazy":    {ObjectValue(excluded, "excluded")},
		"numeric": {ObjectValue(numeric, "numericHolder")},
		"text":    {ObjectValue(textHolder, "textHolder")},
	}
	result, err := Execute(fixture.program(t, "NameThenFilter"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("a comparable shadow after an exhausted preview: %v", err)
	}
	if got := rowNames(result); len(got) != 0 {
		t.Fatalf("rows = %v, want none", got)
	}
}

// Cancellation reached during evidence collection ends the query instead of
// being mistaken for an absent value or an invalid operand.
func TestQueryEvidenceInterruptIsNotSuppressed(t *testing.T) {
	fixture := loadExecutionFixture(t, exhaustedEvidenceQuery(40, false))
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	excluded, err := ctx.Instantiate(fixture.symbol(t, "excluded"))
	if err != nil {
		t.Fatalf("Instantiate excluded: %v", err)
	}
	textHolder, err := ctx.Instantiate(fixture.symbol(t, "textHolder"))
	if err != nil {
		t.Fatalf("Instantiate textHolder: %v", err)
	}
	flag := &atomic.Bool{}
	flag.Store(true)
	ctx.SetInterrupt(flag)
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "excluded", Object: excluded}, {Label: "textHolder", Object: textHolder}},
	}
	bindings := Bindings{
		"lazy": {ObjectValue(excluded, "excluded")},
		"text": {ObjectValue(textHolder, "textHolder")},
	}
	_, err = Execute(fixture.program(t, "NameThenFilter"), context, bindings, Options{})
	if !errors.Is(err, runtime.ErrInterrupted) {
		t.Fatalf("an interrupted evidence read = %v, want %v", err, runtime.ErrInterrupted)
	}
}

// drawSettingsEvidenceQuery supplies the only numeric evidence from a lazy
// random default, so a preview must draw under the settings in force now.
const drawSettingsEvidenceQuery = `
private import ScalarValues::*;
private import RandomFunctions::*;
part def Excluded { attribute annotations : Integer = uniformInteger(4, 5); }
part def TextHolder { attribute annotations : String = "text"; }
part excluded : Excluded;
part textHolder : TextHolder;
calc def NameThenFilter :> Query {
	in lazy : Element;
	in text : Element;
	WhereFeature(
		source = WhereName(
			source = Union(source = lazy, other = text),
			operator = "=", value = "textHolder"
		),
		'feature' = "annotations", operator = ">", value = "3"
	)
}
`

func executeDrawSettingsEvidence(t *testing.T, change func(*runtime.Context)) (result *RowSet, err error) {
	t.Helper()
	fixture := loadExecutionFixture(t, drawSettingsEvidenceQuery)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	excluded, err := ctx.Instantiate(fixture.symbol(t, "excluded"))
	if err != nil {
		t.Fatalf("Instantiate excluded: %v", err)
	}
	if annotations := excluded.FeatureValues["annotations"]; annotations == nil || annotations.Materialized {
		t.Fatalf("annotations before the draw setting = %+v, want an unmaterialized default", annotations)
	}
	textHolder, err := ctx.Instantiate(fixture.symbol(t, "textHolder"))
	if err != nil {
		t.Fatalf("Instantiate textHolder: %v", err)
	}
	change(ctx)
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "excluded", Object: excluded}, {Label: "textHolder", Object: textHolder}},
	}
	bindings := Bindings{
		"lazy": {ObjectValue(excluded, "excluded")},
		"text": {ObjectValue(textHolder, "textHolder")},
	}
	return Execute(fixture.program(t, "NameThenFilter"), context, bindings, Options{})
}

// Changing the seed between a held session's commands changes the draw source a
// top-level evidence preview uses, as it does an ordinary lazy feature read.
func TestQueryEvidenceUsesANewModelSeed(t *testing.T) {
	result, err := executeDrawSettingsEvidence(t, func(ctx *runtime.Context) { ctx.SetModelSeed(7) })
	if err != nil {
		t.Fatalf("query after setting a seed: %v", err)
	}
	if got := rowNames(result); len(got) != 0 {
		t.Fatalf("rows = %v, want none", got)
	}
}

// The same holds for a draw policy changed after the last run: an unseeded
// random-only scheduler may not decide what the new run would draw.
func TestQueryEvidenceUsesANewDrawPolicy(t *testing.T) {
	result, err := executeDrawSettingsEvidence(t, func(ctx *runtime.Context) { ctx.SetDrawPolicy(runtime.DrawMax) })
	if err != nil {
		t.Fatalf("query after setting the draw policy: %v", err)
	}
	if got := rowNames(result); len(got) != 0 {
		t.Fatalf("rows = %v, want none", got)
	}
}

// dottedStepCapQuery reaches two held sensors in one evidence preview; each
// sensor's numeric default fits the budget, but both together do not.
func dottedStepCapQuery(terms int) string {
	return fmt.Sprintf(`
private import ScalarValues::*;
part def NumericSensor {
	attribute base : Integer = 1;
	attribute annotations : Integer = base%s;
}
part def PlainSensor { attribute annotations : String = "text"; }
part def Holder { part sensor : NumericSensor[2]; }
part def TextHolder { part sensor : PlainSensor[1]; }
part numericHolder : Holder;
part textHolder : TextHolder;
calc def DottedFilter :> Query {
	in numeric : Element;
	in text : Element;
	WhereFeature(
		source = WhereName(
			source = Union(source = numeric, other = text),
			operator = "=", value = "textHolder"
		),
		'feature' = "sensor.annotations", operator = ">", value = "3"
	)
}
`, strings.Repeat("+1", terms))
}

func executeDottedStepCap(t *testing.T, budget int64) (*RowSet, error) {
	t.Helper()
	fixture := loadExecutionFixture(t, dottedStepCapQuery(20))
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), budget)
	numericHolder, err := ctx.Instantiate(fixture.symbol(t, "numericHolder"))
	if err != nil {
		t.Fatalf("Instantiate numericHolder: %v", err)
	}
	textHolder, err := ctx.Instantiate(fixture.symbol(t, "textHolder"))
	if err != nil {
		t.Fatalf("Instantiate textHolder: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "numericHolder", Object: numericHolder}, {Label: "textHolder", Object: textHolder}},
	}
	bindings := Bindings{
		"numeric": {ObjectValue(numericHolder, "numericHolder")},
		"text":    {ObjectValue(textHolder, "textHolder")},
	}
	return Execute(fixture.program(t, "DottedFilter"), context, bindings, Options{})
}

// Nested feature probes on a dotted path share the preview's step counter: two
// reads that each fit cannot each spend the whole allowance.
func TestQueryDottedEvidenceSharesItsStepCap(t *testing.T) {
	_, err := executeDottedStepCap(t, 60)
	if !errors.Is(err, runtime.ErrStepLimitExceeded) {
		t.Fatalf("a dotted preview over two reads under a 60-step cap = %v, want %v", err, runtime.ErrStepLimitExceeded)
	}
}

// With enough steps for the whole path, the same dotted comparison is valid and
// returns no rows rather than a budget failure.
func TestQueryDottedEvidenceWithinItsStepCap(t *testing.T) {
	result, err := executeDottedStepCap(t, 160)
	if err != nil {
		t.Fatalf("a dotted preview within its step cap: %v", err)
	}
	if got := rowNames(result); len(got) != 0 {
		t.Fatalf("rows = %v, want none", got)
	}
}
