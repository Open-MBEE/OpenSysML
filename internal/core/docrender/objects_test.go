package docrender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// objectFixtureDocument evaluates the object report over a session holding
// the objects named, each under its own name.
func objectFixtureDocument(t *testing.T, held ...string) *docir.Document {
	t.Helper()
	fixture := loadRenderFixture(t, filepath.Join("testdata", "object_report.sysml"))
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	roots := make([]queryexec.Root, 0, len(held))
	for _, name := range held {
		inst, err := ctx.Instantiate(fixture.symbol(t, "Garage::"+name))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", name, err)
		}
		roots = append(roots, queryexec.Root{Label: name, Object: inst})
	}
	report := symbols.PreferDeclared(fixture.index.LookupQualified("Garage::CarReport"))
	if len(report) != 1 {
		t.Fatalf("lookup Garage::CarReport: got %d symbols", len(report))
	}
	plan, err := docplan.Compile(fixture.index, fixture.model, fixture.resolver, report[0])
	if err != nil {
		t.Fatalf("compile document: %v", err)
	}
	document, err := docir.Evaluate(plan,
		queryexec.Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx, Roots: roots},
		queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate document: %v", err)
	}
	return document
}

// TestMarkdownObjectReportGolden locks the Markdown of a document over
// objects: rows by path, object-valued cells by path, and a list of what the
// session holds.
func TestMarkdownObjectReportGolden(t *testing.T) {
	got, err := Markdown(objectFixtureDocument(t, "car", "spare"), MarkdownOptions{})
	if err != nil {
		t.Fatalf("render document: %v", err)
	}
	golden := filepath.Join("testdata", "object_report.golden.md")
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

// TestMarkdownObjectReportDeclared checks that the same document over a session
// holding nothing renders the declared model: the bound usage's own row, which
// owns no parts of its own and holds no objects, and no wheels.
func TestMarkdownObjectReportDeclared(t *testing.T) {
	got, err := Markdown(objectFixtureDocument(t), MarkdownOptions{})
	if err != nil {
		t.Fatalf("render document: %v", err)
	}
	for _, want := range []string{
		"| name | qualifiedName | pressure |\n| --- | --- | --- |\n\n",
		"| name | engine | wheels |\n| --- | --- | --- |\n| car |  |  |\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("declared rendering does not contain %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "wheels\\[") || strings.Contains(got, "\n- ") {
		t.Errorf("a session holding nothing rendered objects:\n%s", got)
	}
}

// TestHTMLObjectReport checks the HTML of a document over objects: rows and
// items carry the object's identity next to the element it stands for, an
// object-valued cell is marked as such, and scalar cells stay scalar.
func TestHTMLObjectReport(t *testing.T) {
	got, err := HTML(objectFixtureDocument(t, "car", "spare"), HTMLOptions{Fragment: true})
	if err != nil {
		t.Fatalf("render document as HTML: %v", err)
	}
	for _, want := range []string{
		`<tr class="sysml-row" data-object="#1" data-element="Garage::car" data-element-kind="partUsage">`,
		`<tr class="sysml-row" data-object="#3" data-element="Garage::Car::engine" data-element-kind="partUsage">`,
		`<tr class="sysml-row" data-object="#4" data-element="Garage::Car::wheels" data-element-kind="partUsage">`,
		`<td class="sysml-cell" data-column="name" data-value-kind="string"><span class="sysml-value" data-value-kind="string">wheels[1]</span></td>`,
		`<td class="sysml-cell" data-column="pressure" data-value-kind="integer"><span class="sysml-value" data-value-kind="integer">30</span></td>`,
		`<td class="sysml-cell" data-column="engine" data-value-kind="object"><span class="sysml-value sysml-object" data-value-kind="object" data-object="#3" data-element="Garage::Car::engine" data-element-kind="partUsage">car.engine</span></td>`,
		`<span class="sysml-value sysml-object" data-value-kind="object" data-object="#4" data-element="Garage::Car::wheels" data-element-kind="partUsage">car.wheels[1]</span>`,
		`<li class="sysml-item" data-object="#2" data-element="Garage::spare" data-element-kind="partUsage">spare 20</li>`,
		`<li class="sysml-item" data-object="#4" data-element="Garage::Car::wheels" data-element-kind="partUsage">car.wheels[1] 30</li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendering does not contain %q\n%s", want, got)
		}
	}
}
