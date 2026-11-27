package queryexec

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The excluded holder's unmaterialized attribute is the only numeric operand
// source, so its evidence read has to evaluate a default. A spent caller budget
// must not turn the valid empty result into an operand error.
const shadowEvidenceBudgetFixture = `
private import ScalarValues::*;
part def Excluded { attribute annotations : Integer = ScalarFunctions::max(5, 1); }
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

func TestQueryShadowEvidenceDoesNotInheritASpentStepBudget(t *testing.T) {
	fixture := loadExecutionFixture(t, shadowEvidenceBudgetFixture)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	excluded, err := ctx.Instantiate(fixture.symbol(t, "excluded"))
	if err != nil {
		t.Fatalf("Instantiate excluded: %v", err)
	}
	annotations := excluded.FeatureValues["annotations"]
	if annotations == nil || annotations.Materialized {
		t.Fatalf("annotations before the evaluation = %+v, want an unmaterialized default", annotations)
	}
	textHolder, err := ctx.Instantiate(fixture.symbol(t, "textHolder"))
	if err != nil {
		t.Fatalf("Instantiate textHolder: %v", err)
	}

	budgets := ctx.Budgets()
	budgets.MaxSteps = 12
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	expr := parser.New(source.New("<spend>", []byte(strings.Repeat("textHolder.annotations+", 20)+"textHolder.annotations"))).ParseExpression()
	if _, err := ctx.EvalWithScope(expr, fixture.symbol(t, "textHolder").Scope); !errors.Is(err, runtime.ErrStepLimitExceeded) {
		t.Fatalf("spending evaluation = %v, want %v", err, runtime.ErrStepLimitExceeded)
	}

	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "excluded", Object: excluded}, {Label: "textHolder", Object: textHolder}},
	}
	bindings := Bindings{
		"lazy": {ObjectValue(excluded, "excluded")},
		"text": {ObjectValue(textHolder, "textHolder")},
	}
	result, err := Execute(fixture.program(t, "NameThenFilter"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("execute NameThenFilter after a spent budget: %v", err)
	}
	if got := rowNames(result); len(got) != 0 {
		t.Fatalf("rows = %v, want none", got)
	}
	if annotations.Materialized {
		t.Fatal("reading evidence materialized the excluded row's annotation default")
	}
}
