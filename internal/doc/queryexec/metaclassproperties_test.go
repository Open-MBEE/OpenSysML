package queryexec

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const metaclassPropertyModel = `
part def Subsystem;
package Scope { part telescope : Subsystem { part mirror; ref part spare; } }
calc def PartUsages :> Query {
	in root : Element;
	WhereType(source = Descendants(source = root), type = "PartUsage", exact = true)
}
calc def Composites :> Query {
	in root : Element;
	Project(source = PartUsages(root = root), properties = ("name", "isComposite", "isPortion"))
}
calc def OnlyComposites :> Query {
	in root : Element;
	WhereFeature(source = PartUsages(root = root), feature = "isComposite", operator = "=", value = "true")
}
calc def Identified :> Query {
	in root : Element;
	Project(source = PartUsages(root = root), properties = ("name", "elementId"))
}
calc def Misspelt :> Query {
	in root : Element;
	Project(source = PartUsages(root = root), properties = ("name", "isCompost"))
}
calc def Crossings :> Query {
	in root : Element;
	Project(source = PartUsages(root = root), properties = ("name", "crossFeature"))
}
`

func metaclassPropertyCells(t *testing.T, result *RowSet) []string {
	t.Helper()
	var rows []string
	for _, row := range result.Rows() {
		var cells []string
		for _, cell := range row.Cells() {
			var texts []string
			for _, value := range cell.Values() {
				if boolean, ok := value.Boolean(); ok {
					texts = append(texts, fmt.Sprint(boolean))
					continue
				}
				texts = append(texts, differValueText(t, value))
			}
			cells = append(cells, strings.Join(texts, ","))
		}
		rows = append(rows, strings.Join(cells, " "))
	}
	return rows
}

// TestExecuteProjectsMetamodelTableProperties: Project reads any property the
// metamodel gives the row's metaclass, through the reflective evaluator.
func TestExecuteProjectsMetamodelTableProperties(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	result, err := fixture.execute(t, "Composites", bindings, Options{})
	if err != nil {
		t.Fatalf("Composites: %v", err)
	}
	got := strings.Join(metaclassPropertyCells(t, result), " | ")
	want := "telescope false false | mirror true false | spare false false"
	if got != want {
		t.Fatalf("Composites rows = %q, want %q", got, want)
	}
}

// TestExecuteWhereFeatureFiltersOnMetamodelTableProperty: WhereFeature compares
// a metaclass property as it does a declared one.
func TestExecuteWhereFeatureFiltersOnMetamodelTableProperty(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	result, err := fixture.execute(t, "OnlyComposites", bindings, Options{})
	if err != nil {
		t.Fatalf("OnlyComposites: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("OnlyComposites returned %d rows, want the one composite part", len(rows))
	}
	if sym, _ := rows[0].Element().Element(); sym.Name != "mirror" {
		t.Fatalf("OnlyComposites kept %s, want mirror", sym.Name)
	}
}

// TestExecuteProjectsElementIDThroughTheContext: elementId is what the context
// reads for the element; absent without a reader, and a reader's refusal fails
// the read with its reason.
func TestExecuteProjectsElementIDThroughTheContext(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	context := Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}
	absent, err := Execute(fixture.program(t, "Identified"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("Identified without a reader: %v", err)
	}
	if got := strings.Join(metaclassPropertyCells(t, absent), " | "); got != "telescope  | mirror  | spare " {
		t.Fatalf("without a reader the cells are %q, want elementId absent", got)
	}
	context.ElementID = func(sym *symbols.Symbol) (string, error) {
		return "id:" + symbols.FQNOf(sym), nil
	}
	identified, err := Execute(fixture.program(t, "Identified"), context, bindings, Options{})
	if err != nil {
		t.Fatalf("Identified: %v", err)
	}
	want := "telescope id:Observatory::Scope::telescope | mirror id:Observatory::Scope::telescope::mirror | spare id:Observatory::Scope::telescope::spare"
	if got := strings.Join(metaclassPropertyCells(t, identified), " | "); got != want {
		t.Fatalf("cells = %q, want %q", got, want)
	}
	refusal := errors.New("the model does not convert")
	context.ElementID = func(*symbols.Symbol) (string, error) { return "", refusal }
	_, err = Execute(fixture.program(t, "Identified"), context, bindings, Options{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorUnevaluableFeature || !errors.Is(err, refusal) {
		t.Fatalf("a refused conversion gives %v, want an unevaluable-feature error carrying the refusal", err)
	}
}

// TestExecuteUnknownPropertyOffersTheMetaclassProperties: a property no row
// carries is unknown, and the message offers the nearest the rows' metaclass has.
func TestExecuteUnknownPropertyOffersTheMetaclassProperties(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	_, err := fixture.execute(t, "Misspelt", bindings, Options{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorUnknownProperty {
		t.Fatalf("Misspelt gives %v, want an unknown-property error", err)
	}
	if !strings.Contains(err.Error(), "did you mean isComposite") {
		t.Fatalf("message %q does not offer isComposite", err.Error())
	}
}

// TestExecuteUnservedPropertyNamesTheGap: a metaclass property the evaluator does
// not derive is a typed path error naming the defining class and the element.
func TestExecuteUnservedPropertyNamesTheGap(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	_, err := fixture.execute(t, "Crossings", bindings, Options{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorUnservedProperty {
		t.Fatalf("Crossings gives %v, want an unserved-property error", err)
	}
	if typed.Expected != "Feature::crossFeature" || typed.Target != "Observatory::Scope::telescope" {
		t.Fatalf("error names %s of %s, want Feature::crossFeature of Observatory::Scope::telescope", typed.Expected, typed.Target)
	}
	want := fmt.Sprintf("query %s reads Feature::crossFeature of Observatory::Scope::telescope, a metamodel property OpenSysML does not derive yet", typed.Query)
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("message %q, want prefix %q", err.Error(), want)
	}
	if typed.Outcome == nil || typed.Outcome.Path == nil {
		t.Fatalf("the error carries no path-error record")
	}
}

// TestExecuteRedefinedMetaclassPropertyStaysExposed: Usage::definition stays
// readable on a PartUsage, whose partDefinition redefines it.
func TestExecuteRedefinedMetaclassPropertyStaysExposed(t *testing.T) {
	fixture := loadExecutionFixture(t, metaclassPropertyModel+`
calc def Definitions :> Query {
	in root : Element;
	Project(source = PartUsages(root = root), properties = ("name", "definition"))
}
`)
	bindings := Bindings{"root": {ElementValue(fixture.symbol(t, "Scope"))}}
	_, err := fixture.execute(t, "Definitions", bindings, Options{})
	var typed *Error
	if errors.As(err, &typed) && typed.Kind == ErrorUnknownProperty {
		t.Fatalf("definition is unknown on a PartUsage: %v", err)
	}
	if err != nil && (!errors.As(err, &typed) || typed.Kind != ErrorUnservedProperty || typed.Expected != "Usage::definition") {
		t.Fatalf("Definitions: %v, want rows or the unserved Usage::definition", err)
	}
}
