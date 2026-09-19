package docplan

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const planningImports = `
private import DocumentQueries::*;
private import KerML::Root::Element;
private import ScalarValues::*;
`

const fixtureDoc = "document-planning.sysml"

type planningFixture struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
}

func loadPlanningFixture(t *testing.T, body string) planningFixture {
	t.Helper()
	return loadPlanningSource(t, "package Observatory {"+planningImports+body+"}")
}

func loadPlanningFixtureFile(t *testing.T, path string) planningFixture {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return loadPlanningSource(t, string(content))
}

func loadPlanningSource(t *testing.T, content string) planningFixture {
	t.Helper()
	index := libs.NewModelIndex()
	p := parser.New(source.New(fixtureDoc, []byte(content)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument(fixtureDoc, root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	return planningFixture{
		index:    index,
		model:    semantics.NewModel(resolver),
		resolver: resolver,
	}
}

func (f planningFixture) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified("Observatory::" + name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

func (f planningFixture) compile(t *testing.T, name string) (*Plan, error) {
	t.Helper()
	return Compile(f.index, f.model, f.resolver, f.symbol(t, name))
}

func (f planningFixture) mustCompile(t *testing.T, name string) *Plan {
	t.Helper()
	plan, err := f.compile(t, name)
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return plan
}

func planningError(t *testing.T, err error) *Error {
	t.Helper()
	var planning *Error
	if !errors.As(err, &planning) {
		t.Fatalf("error = %v, want *docplan.Error", err)
	}
	return planning
}

func TestCompileTelescopeDocument(t *testing.T) {
	fixture := loadPlanningFixtureFile(t, "testdata/telescope_document.sysml")
	plan := fixture.mustCompile(t, "MassReport")
	if plan.Name() != "Observatory::MassReport" {
		t.Fatalf("name = %q", plan.Name())
	}
	if plan.Title() != "Telescope Mass Report" {
		t.Fatalf("title = %q", plan.Title())
	}
	if !plan.Origin().Located() || plan.Origin().Doc != fixtureDoc {
		t.Fatalf("origin = %+v", plan.Origin())
	}
	content := plan.Content()
	if len(content) != 2 {
		t.Fatalf("content = %d nodes, want 2", len(content))
	}

	intro := content[0]
	if intro.Kind() != ContentParagraph || intro.Name() != "intro" {
		t.Fatalf("intro = %s %q", intro.Kind(), intro.Name())
	}
	if intro.Text() != "Mass rollup for the telescope assembly." || intro.Query() != nil {
		t.Fatalf("intro text = %q query = %v", intro.Text(), intro.Query())
	}
	if !intro.Origin().Located() {
		t.Fatalf("intro origin = %+v", intro.Origin())
	}

	breakdown := content[1]
	if breakdown.Kind() != ContentSection || breakdown.Title() != "Subsystem Masses" {
		t.Fatalf("breakdown = %s %q", breakdown.Kind(), breakdown.Title())
	}
	children := breakdown.Children()
	if len(children) != 3 {
		t.Fatalf("breakdown children = %d, want 3", len(children))
	}

	masses := children[0]
	if masses.Kind() != ContentTable || masses.Caption() != "All subsystems by mass" {
		t.Fatalf("masses = %s %q", masses.Kind(), masses.Caption())
	}
	query := masses.Query()
	if query == nil || query.Entry() != "Observatory::SubsystemTable" {
		t.Fatalf("masses query = %+v", query)
	}
	if query.Program() == nil || query.Program().Entry() != "Observatory::SubsystemTable" {
		t.Fatalf("masses program = %+v", query.Program())
	}
	bindings := query.Bindings()
	if len(bindings) != 1 || bindings[0].Parameter() != "root" {
		t.Fatalf("masses bindings = %+v", bindings)
	}
	values := bindings[0].Values()
	if len(values) != 1 {
		t.Fatalf("masses binding values = %d", len(values))
	}
	element, ok := values[0].Element()
	if !ok || symbols.FQNOf(element) != "Observatory::telescope" {
		t.Fatalf("masses binding element = %v", element)
	}
	if !values[0].Origin().Located() {
		t.Fatalf("masses binding origin = %+v", values[0].Origin())
	}

	heavy := children[1]
	if heavy.Kind() != ContentSection || heavy.Title() != "Heavy Subsystems" {
		t.Fatalf("heavy = %s %q", heavy.Kind(), heavy.Title())
	}
	heavyChildren := heavy.Children()
	if len(heavyChildren) != 2 {
		t.Fatalf("heavy children = %d, want 2", len(heavyChildren))
	}
	summary := heavyChildren[0]
	if summary.Kind() != ContentParagraph || summary.Text() != "" || summary.Query() == nil {
		t.Fatalf("summary = %s %q %v", summary.Kind(), summary.Text(), summary.Query())
	}
	if summary.Query().Entry() != "Observatory::HeavySubsystemNames" {
		t.Fatalf("summary query = %q", summary.Query().Entry())
	}
	summaryBindings := summary.Query().Bindings()
	if len(summaryBindings) != 2 || summaryBindings[1].Parameter() != "threshold" {
		t.Fatalf("summary bindings = %+v", summaryBindings)
	}
	threshold, ok := summaryBindings[1].Values()[0].String()
	if !ok || threshold != "10" {
		t.Fatalf("summary threshold = %q", threshold)
	}
	heavyItems := heavyChildren[1]
	if heavyItems.Kind() != ContentList || heavyItems.Style() != ListNumber {
		t.Fatalf("heavyItems = %s %q", heavyItems.Kind(), heavyItems.Style())
	}

	missing := children[2]
	if missing.Kind() != ContentSection || len(missing.Children()) != 2 {
		t.Fatalf("missing = %s %d", missing.Kind(), len(missing.Children()))
	}
	if missing.Children()[1].Style() != ListBullet {
		t.Fatalf("missing list style = %q", missing.Children()[1].Style())
	}
}

func TestCompiledPlanIsImmutable(t *testing.T) {
	fixture := loadPlanningFixtureFile(t, "testdata/telescope_document.sysml")
	plan := fixture.mustCompile(t, "MassReport")
	content := plan.Content()
	content[0] = Content{}
	if plan.Content()[0].Kind() != ContentParagraph {
		t.Fatal("mutating returned content changed the plan")
	}
	section := plan.Content()[1]
	children := section.Children()
	children[0] = Content{}
	if section.Children()[0].Kind() != ContentTable {
		t.Fatal("mutating returned children changed the plan")
	}
	query := section.Children()[0].Query()
	bindings := query.Bindings()
	bindings[0] = Binding{}
	if query.Bindings()[0].Parameter() != "root" {
		t.Fatal("mutating returned bindings changed the plan")
	}
}

func TestIsDocumentDefinition(t *testing.T) {
	fixture := loadPlanningFixtureFile(t, "testdata/telescope_document.sysml")
	if !IsDocumentDefinition(fixture.index, fixture.model, fixture.symbol(t, "MassReport")) {
		t.Fatal("MassReport should be a document definition")
	}
	if IsDocumentDefinition(fixture.index, fixture.model, fixture.symbol(t, "Subsystem")) {
		t.Fatal("Subsystem should not be a document definition")
	}
	if IsDocumentDefinition(fixture.index, fixture.model, fixture.symbol(t, "SubsystemTable")) {
		t.Fatal("SubsystemTable should not be a document definition")
	}
}

func TestCompileRejectsNonDocument(t *testing.T) {
	fixture := loadPlanningFixtureFile(t, "testdata/telescope_document.sysml")
	_, err := fixture.compile(t, "Subsystem")
	planning := planningError(t, err)
	if planning.Kind != ErrorNotDocumentDefinition {
		t.Fatalf("kind = %s", planning.Kind)
	}
}

func TestCompileRequiresContext(t *testing.T) {
	_, err := Compile(nil, nil, nil, nil)
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidContext {
		t.Fatalf("kind = %s", planning.Kind)
	}
}

func TestCompileReportsMissingTitle(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Untitled :> Document {
			part intro : Paragraph {
				attribute redefines text = "text";
			}
		}
	`)
	_, err := fixture.compile(t, "Untitled")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingTitle {
		t.Fatalf("kind = %s", planning.Kind)
	}
	if !planning.Origin.Located() {
		t.Fatalf("origin = %+v", planning.Origin)
	}
}

func TestCompileReportsMissingSectionTitle(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part body : Section {
				part intro : Paragraph {
					attribute redefines text = "text";
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingTitle || planning.Content != "Observatory::Report::body" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileReportsInvalidContent(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Widget;
		part def Report :> Document {
			attribute redefines title = "Report";
			part stray : Widget;
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidContent || planning.Content != "Observatory::Report::stray" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileReportsNestedDocument(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Inner :> Document {
			attribute redefines title = "Inner";
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part nested : Inner;
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorNestedDocument {
		t.Fatalf("kind = %s", planning.Kind)
	}
}

func TestCompileReportsParagraphWithoutContent(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part empty : Paragraph;
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingText {
		t.Fatalf("kind = %s", planning.Kind)
	}
}

func TestCompileReportsParagraphWithTextAndQuery(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part both : Paragraph {
				attribute redefines text = "text";
				calc names : Names {
					in root = telescope;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorConflictingText {
		t.Fatalf("kind = %s", planning.Kind)
	}
}

func TestCompileReportsTableWithoutQuery(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = "Report";
			part empty : Table;
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingQuery {
		t.Fatalf("kind = %s", planning.Kind)
	}
}

func TestCompileReportsUnknownQuery(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def NotAQuery {
			return result : Element[0..*] ordered;
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part table : Table {
				calc rows : NotAQuery;
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorUnknownQuery {
		t.Fatalf("kind = %s", planning.Kind)
	}
}

func TestCompileWrapsQueryPlanningFailure(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Broken :> Query {
			in root : Element;
			OwnedElements(source = missing)
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part table : Table {
				calc rows : Broken {
					in root = Report;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorQueryPlanning {
		t.Fatalf("kind = %s", planning.Kind)
	}
	var inner *queryplan.Error
	if !errors.As(err, &inner) || inner.Kind != queryplan.ErrorUnknownParameter {
		t.Fatalf("inner = %v", planning.Err)
	}
}

func TestCompileReportsUnknownParameter(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part list : List {
				calc items : Names {
					in root = telescope;
					in bogus = telescope;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorUnknownParameter || planning.Parameter != "bogus" {
		t.Fatalf("error = %+v", planning)
	}
	if !planning.Origin.Located() {
		t.Fatalf("origin = %+v", planning.Origin)
	}
}

func TestCompileReportsMissingBinding(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part list : List {
				calc items : Names;
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingBinding || planning.Parameter != "root" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileLeavesDefaultedParametersToTheExecutor(t *testing.T) {
	fixture := loadPlanningFixtureFile(t, "../../doc/docrender/testdata/defaulted_queries.sysml")
	plan := fixture.mustCompile(t, "DefaultedReport")
	content := plan.Content()
	if len(content) != 3 {
		t.Fatalf("content = %d nodes, want 3", len(content))
	}
	heavy := content[0].Query()
	if heavy == nil || len(heavy.Bindings()) != 0 {
		t.Fatalf("heavy query = %+v, want no explicit bindings", heavy)
	}
	params := map[string]queryplan.Parameter{}
	for _, definition := range heavy.Program().Definitions() {
		if definition.Name() != heavy.Entry() {
			continue
		}
		for _, param := range definition.Parameters() {
			params[param.Name] = param
		}
	}
	if !params["root"].HasDefault || !params["threshold"].HasDefault {
		t.Fatalf("parameters = %+v, want retained defaults", params)
	}
	light := content[1].Query()
	if light == nil || len(light.Bindings()) != 0 {
		t.Fatalf("light query = %+v, want no explicit bindings", light)
	}
	explicit := content[2].Query()
	if explicit == nil || len(explicit.Bindings()) != 2 {
		t.Fatalf("explicit query = %+v, want two explicit bindings", explicit)
	}
}

func TestCompileReportsBindingTypeMismatch(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part list : List {
				calc items : Names {
					in root = "telescope";
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorBindingType || planning.Parameter != "root" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileReportsBindingMultiplicityMismatch(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Names :> Query {
			in root : Element[1];
			OwnedElements(source = root)
		}
		part telescope;
		part instruments;
		part def Report :> Document {
			attribute redefines title = "Report";
			part list : List {
				calc items : Names {
					in root = (telescope, instruments);
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorBindingMultiplicity || planning.Parameter != "root" {
		t.Fatalf("error = %+v", planning)
	}
}

// `()` and `null` are the one null expression, binding no value, and a sequence
// is flat: a `null` element adds nothing and a nested sequence its own values.
// Within a parameter's multiplicity that is a binding, outside it a mismatch.
func TestCompileBindsNullToNoValues(t *testing.T) {
	cases := map[string]struct {
		binding string
		want    []string
	}{
		"parens":    {"()", nil},
		"null":      {"null", nil},
		"all null":  {"(null, null)", nil},
		"mixed":     {`("a", null, "b")`, []string{"a", "b"}},
		"nested":    {`("a", ("b", null, "c"))`, []string{"a", "b", "c"}},
		"one value": {`"a"`, []string{"a"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				calc def Names :> Query {
					in root : Element;
					in tags : String[0..*];
					OwnedElements(source = root)
				}
				part telescope;
				part def Report :> Document {
					attribute redefines title = "Report";
					part list : List {
						calc items : Names {
							in root = telescope;
							in tags = `+tc.binding+`;
						}
					}
				}
			`)
			plan, err := fixture.compile(t, "Report")
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			bindings := plan.Content()[0].Query().Bindings()
			if len(bindings) != 2 || bindings[1].Parameter() != "tags" {
				t.Fatalf("bindings = %+v", bindings)
			}
			var got []string
			for _, value := range bindings[1].Values() {
				text, ok := value.String()
				if !ok {
					t.Fatalf("tags value %+v is not a string", value)
				}
				got = append(got, text)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("tags = %q, want %q", got, tc.want)
			}
		})
	}
	for name, binding := range map[string]string{"parens": "()", "all null": "(null, null)"} {
		t.Run(name+" required", func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				calc def Names :> Query {
					in root : Element[1];
					OwnedElements(source = root)
				}
				part def Report :> Document {
					attribute redefines title = "Report";
					part list : List {
						calc items : Names {
							in root = `+binding+`;
						}
					}
				}
			`)
			_, err := fixture.compile(t, "Report")
			planning := planningError(t, err)
			if planning.Kind != ErrorBindingMultiplicity || planning.Parameter != "root" || planning.Actual != "0" {
				t.Fatalf("error = %+v", planning)
			}
		})
	}
}

func TestCompileAcceptsSignedNumericBindings(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Names :> Query {
			in root : Element;
			in offset : Integer;
			in factor : Real;
			OwnedElements(source = root)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part list : List {
				calc items : Names {
					in root = telescope;
					in offset = -3;
					in factor = +2.5;
				}
			}
		}
	`)
	plan, err := fixture.compile(t, "Report")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	bindings := plan.Content()[0].Query().Bindings()
	if len(bindings) != 3 {
		t.Fatalf("bindings = %+v", bindings)
	}
	offset, ok := bindings[1].Values()[0].Integer()
	if !ok || offset != -3 {
		t.Fatalf("offset = %d %v", offset, ok)
	}
	if !bindings[1].Values()[0].Origin().Located() {
		t.Fatalf("offset origin = %+v", bindings[1].Values()[0].Origin())
	}
	factor, ok := bindings[2].Values()[0].Real()
	if !ok || factor != 2.5 {
		t.Fatalf("factor = %g %v", factor, ok)
	}
}

func TestCompileRejectsInvalidSignedBindings(t *testing.T) {
	cases := map[string]string{
		"overflow":   "in offset = -99999999999999999999;",
		"nonNumeric": "in offset = -telescope;",
	}
	for name, binding := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := loadPlanningFixture(t, `
				calc def Names :> Query {
					in root : Element;
					in offset : Integer;
					OwnedElements(source = root)
				}
				part telescope;
				part def Report :> Document {
					attribute redefines title = "Report";
					part list : List {
						calc items : Names {
							in root = telescope;
							`+binding+`
						}
					}
				}
			`)
			_, err := fixture.compile(t, "Report")
			planning := planningError(t, err)
			if planning.Kind != ErrorUnsupportedBinding || planning.Parameter != "offset" {
				t.Fatalf("error = %+v", planning)
			}
		})
	}
}

func TestCompileReportsInvalidListStyle(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part telescope;
		part def Report :> Document {
			attribute redefines title = "Report";
			part list : List {
				attribute redefines style = "roman";
				calc items : Names {
					in root = telescope;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidStyle || planning.Actual != "roman" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileInheritsContentAndAttributes(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def TemplateReport :> Document {
			attribute redefines title = "Template";
			part intro : Paragraph {
				attribute redefines text = "Shared overview.";
			}
			part items : List {
				attribute redefines style = "number";
				calc names : Names {
					in root = telescope;
				}
			}
		}
		part def DerivedReport :> TemplateReport {
			attribute redefines title = "Derived";
			part extra : Paragraph {
				attribute redefines text = "Derived-only text.";
			}
		}
	`)
	plan := fixture.mustCompile(t, "DerivedReport")
	if plan.Title() != "Derived" {
		t.Fatalf("title = %q", plan.Title())
	}
	content := plan.Content()
	if len(content) != 3 {
		t.Fatalf("content = %d nodes, want 3", len(content))
	}
	if content[0].Name() != "extra" || content[0].Text() != "Derived-only text." {
		t.Fatalf("local content = %q %q", content[0].Name(), content[0].Text())
	}
	if content[1].Name() != "intro" || content[1].Text() != "Shared overview." {
		t.Fatalf("inherited paragraph = %q %q", content[1].Name(), content[1].Text())
	}
	if content[2].Name() != "items" || content[2].Style() != ListNumber || content[2].Query() == nil {
		t.Fatalf("inherited list = %+v", content[2])
	}
}

func TestCompileInheritsQueryFromContentType(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def NameTable :> Table {
			attribute redefines caption = "Names";
			calc rows : Names {
				in root = telescope;
			}
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part names : NameTable;
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	table := plan.Content()[0]
	if table.Kind() != ContentTable || table.Caption() != "Names" {
		t.Fatalf("table = %s %q", table.Kind(), table.Caption())
	}
	if table.Query() == nil || table.Query().Entry() != "Observatory::Names" {
		t.Fatalf("table query = %+v", table.Query())
	}
}

func TestCompileAnonymousContentAndBindingRedefinition(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part : Paragraph {
				attribute redefines text = "Anonymous paragraph.";
			}
			part items : List {
				calc names : Names {
					in :>> root = telescope;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if len(content) != 2 {
		t.Fatalf("content = %d nodes, want 2", len(content))
	}
	if content[0].Kind() != ContentParagraph || content[0].Text() != "Anonymous paragraph." {
		t.Fatalf("anonymous paragraph = %s %q", content[0].Kind(), content[0].Text())
	}
	bindings := content[1].Query().Bindings()
	if len(bindings) != 1 || bindings[0].Parameter() != "root" {
		t.Fatalf("bindings = %+v", bindings)
	}
}

func TestCompileFollowsRedefinitionLineage(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def TemplateReport :> Document {
			attribute redefines title = "Template";
		}
		part def NameList :> List {
			calc names : Names {
				in root = telescope;
			}
		}
		part def DerivedReport :> TemplateReport {
			attribute redefines title;
			part items : NameList {
				calc redefines names {
					in root = telescope;
				}
			}
		}
	`)
	plan := fixture.mustCompile(t, "DerivedReport")
	if plan.Title() != "Template" {
		t.Fatalf("title = %q", plan.Title())
	}
	list := plan.Content()[0]
	if list.Query() == nil || list.Query().Entry() != "Observatory::Names" {
		t.Fatalf("list query = %+v", list.Query())
	}
}

func TestCompileRejectsUnresolvedRedefinedQueryType(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def NameList :> List {
			calc names : Names {
				in root = telescope;
			}
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part items : NameList {
				calc redefines names : Missing {
					in root = telescope;
				}
			}
		}
	`)
	_, err := fixture.compile(t, "Report")
	var planning *Error
	if !errors.As(err, &planning) || planning.Kind != ErrorUnknownQuery {
		t.Fatalf("error = %+v", err)
	}
}

func TestCompileReportsInheritedInvalidContent(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part telescope;
		calc def Names :> Query {
			in root : Element;
			OwnedElements(source = root)
		}
		part def BadSection :> Section {
			attribute redefines title = "Bad";
			calc stray : Names {
				in root = telescope;
			}
		}
		part def Report :> Document {
			attribute redefines title = "Report";
			part s : BadSection;
		}
	`)
	_, err := fixture.compile(t, "Report")
	var planning *Error
	if !errors.As(err, &planning) || planning.Kind != ErrorInvalidContent {
		t.Fatalf("error = %+v", err)
	}
	if planning.Content != "Observatory::BadSection::stray" {
		t.Fatalf("content = %q", planning.Content)
	}
}

func TestCompileReportsNonLiteralTitle(t *testing.T) {
	fixture := loadPlanningFixture(t, `
		part def Report :> Document {
			attribute redefines title = 42;
		}
	`)
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidAttribute || planning.Parameter != "title" {
		t.Fatalf("error = %+v", planning)
	}
}
