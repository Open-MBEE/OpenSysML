package querytranslate

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const shadowedBuiltinFixture = `package Shadow {
	private import ScalarValues::*;
	part def Device {
		attribute isActive : String = "ready";
	}
}`

func shadowFixtureTable(t *testing.T) (*Table, *symbols.Index, *semantics.Model) {
	t.Helper()
	index := libs.NewModelIndex()
	p := parser.New(source.New("shadowed.sysml", []byte(shadowedBuiltinFixture)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse fixture: %v", p.Diagnostics)
	}
	index.AddDocument("shadowed.sysml", root)
	index.ExpandWildcardImports()
	model := semantics.NewModel(resolve.New(index))
	if index.LookupUnique("Shadow::Device::isActive") == nil {
		t.Fatal("the fixture declares no isActive feature")
	}
	return NewTable(index, model), index, model
}

// A bare name a model feature may shadow is left for the row reader to
// classify; the explicit "@" spelling keeps the evaluator's own kind.
func TestTableDefersKindOfAShadowedBuiltin(t *testing.T) {
	table, index, model := shadowFixtureTable(t)
	if got := table.kind("isActive"); got != kindUnknown {
		t.Errorf("kind(isActive) = %d, want kindUnknown for a declared feature", got)
	}
	if got := table.kind("@isActive"); got != kindBoolean {
		t.Errorf("kind(@isActive) = %d, want kindBoolean", got)
	}
	plainIndex := libs.NewModelIndex()
	if got := NewTable(plainIndex, semantics.NewModel(resolve.New(plainIndex))).kind("isActive"); got != kindBoolean {
		t.Errorf("kind(isActive) without a declared feature = %d, want kindBoolean", got)
	}

	// Projection and filtering of the string feature both translate; the same
	// text on the explicit built-in is still refused before a row is read.
	q := Query{
		Scope:  []string{"Shadow"},
		Select: []Selection{{Property: "isActive"}},
		Where:  Comparison{Property: "isActive", Operator: Equal, Values: []string{"ready"}},
	}
	ctx := Context{Index: index, Model: model, Table: table}
	plan, err := Translate(q, ctx, "ShadowQuery")
	if err != nil {
		t.Fatalf("Translate bare isActive: %v", err)
	}
	if len(plan.Columns) != 1 || plan.Columns[0] != "isActive" {
		t.Fatalf("columns = %v, want isActive", plan.Columns)
	}
	q.Where = Comparison{Property: "@isActive", Operator: Equal, Values: []string{"ready"}}
	_, err = Translate(q, ctx, "ShadowQuery")
	if err == nil || !strings.Contains(err.Error(), "needs true or false") {
		t.Fatalf("Translate @isActive with text = %v, want a Boolean-value refusal", err)
	}
}
