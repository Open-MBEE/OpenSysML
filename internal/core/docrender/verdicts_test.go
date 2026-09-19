package docrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// verdictFixtureDocument evaluates the verdict report: over a session holding
// the car when held is set, over the model as declared otherwise.
func verdictFixtureDocument(t *testing.T, held bool) *docir.Document {
	t.Helper()
	fixture := loadRenderFixture(t, filepath.Join("testdata", "verdict_report.sysml"))
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	context := queryexec.Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}
	if held {
		inst, err := ctx.Instantiate(fixture.symbol(t, "Garage::car"))
		if err != nil {
			t.Fatalf("Instantiate car: %v", err)
		}
		context.Runtime = ctx
		context.Roots = []queryexec.Root{{Label: "car", Object: inst}}
	}
	report := symbols.PreferDeclared(fixture.index.LookupQualified("Garage::CarReport"))
	if len(report) != 1 {
		t.Fatalf("lookup Garage::CarReport: got %d symbols", len(report))
	}
	plan, err := docplan.Compile(fixture.index, fixture.model, fixture.resolver, report[0])
	if err != nil {
		t.Fatalf("compile document: %v", err)
	}
	document, err := docir.Evaluate(plan, context, queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate document: %v", err)
	}
	return document
}

// TestMarkdownVerdictReportGolden locks the Markdown of a document over
// verdicts: one row per assertion on the car and the objects it holds, with the
// verdict and its reason, and a list of the verdicts that do not hold.
func TestMarkdownVerdictReportGolden(t *testing.T) {
	got, err := Markdown(verdictFixtureDocument(t, true), MarkdownOptions{})
	if err != nil {
		t.Fatalf("render document: %v", err)
	}
	golden := filepath.Join("testdata", "verdict_report.golden.md")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("rendered Markdown differs from %s (run with -update after intentional changes)\ngot:\n%s", golden, got)
	}
}

// TestMarkdownVerdictReportDeclared checks that the same document with no
// session checks the car as declared: the same assertions, by qualified path.
func TestMarkdownVerdictReportDeclared(t *testing.T) {
	got, err := Markdown(verdictFixtureDocument(t, false), MarkdownOptions{})
	if err != nil {
		t.Fatalf("render document: %v", err)
	}
	for _, want := range []string{
		"| Garage::car | massOk | holds |  |\n",
		"| Garage::car | fits | undecided | ",
		"| Garage::car.engine | powerLow | violated | ",
		"| Garage::car.wheels\\[1\\] | pressureOk | violated | ",
		"- assert constraint fits on Garage::car: undecided\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("declared rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "| car |") {
		t.Errorf("a session holding nothing rendered a session label:\n%s", got)
	}
}

// TestHTMLVerdictReport checks the HTML of a document over verdicts: a row
// carries the verdict, the carrier's path and identity and the assertion; a
// verdict-valued item is marked as such.
func TestHTMLVerdictReport(t *testing.T) {
	got, err := HTML(verdictFixtureDocument(t, true), HTMLOptions{Fragment: true})
	if err != nil {
		t.Fatalf("render document as HTML: %v", err)
	}
	for _, want := range []string{
		`<tr class="sysml-row" data-object="#1" data-verdict="holds" data-path="car" data-element="Garage::Car::massOk" data-element-kind="constraintUsage">`,
		`<tr class="sysml-row" data-object="#1" data-verdict="undecided" data-path="car" data-element="Garage::Car::fits" data-element-kind="constraintUsage">`,
		`<tr class="sysml-row" data-object="#2" data-verdict="violated" data-path="car.engine" data-element="Garage::Engine::powerLow" data-element-kind="constraintUsage">`,
		`data-object="#2" data-verdict="holds" data-path="car.engine" data-element="Garage::" data-element-kind="satisfyRequirementUsage">`,
		`<tr class="sysml-row" data-object="#2" data-verdict="holds" data-path="car.engine" data-element="Garage::checkEngine" data-element-kind="verificationCaseUsage">`,
		`<td class="sysml-cell" data-column="verdict" data-value-kind="string"><span class="sysml-value" data-value-kind="string">violated</span></td>`,
		`<li class="sysml-item" data-object="#2" data-verdict="violated" data-path="car.engine" data-element="Garage::Engine::powerLow" data-element-kind="constraintUsage">assert constraint powerLow on car.engine: violated</li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
}
