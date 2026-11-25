package querytranslate

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
)

<<<<<<< HEAD
// An explicit built-in keeps its Boolean kind despite a shadowing model feature,
// so a string operand is a malformed filter.
=======
// An explicit built-in spelling keeps its Boolean kind under a shadowing
// feature, so a text operand is a malformed filter.
>>>>>>> 9b6b9d8457 (fix(query): scope built-in deferral to the terminal property and its shadow)
func TestOSLCExplicitActivityBuiltinRejectsTextOperand(t *testing.T) {
	table, index, model := shadowFixtureTable(t)
	q, err := ParseParameters(`oslc.where=sysx:@isActive=%22ready%22&oslc.select=sysx:@isActive`, table)
	if err != nil {
		t.Fatalf("ParseParameters: %v", err)
	}
	_, err = Translate(q, Context{Index: index, Model: model, Table: table}, "OSLCBuiltin")
	var refused *Error
	if !errors.As(err, &refused) || refused.Kind != ErrUnparsableValue {
		t.Fatalf("Translate = %v, want an unparsable Boolean operand", err)
	}
	// The same model's bare name reads its string feature and stays filterable.
	bare, err := ParseParameters(`oslc.where=sysx:isActive=%22ready%22`, table)
	if err != nil {
		t.Fatalf("ParseParameters bare: %v", err)
	}
	if _, err := Translate(bare, Context{Index: index, Model: model, Table: table}, "OSLCFeature"); err != nil {
		t.Fatalf("Translate bare: %v", err)
	}
	// A caller that canonicalizes the spelling still records the built-in
	// selection; validation must not defer to the shadowing feature then.
	selected := Query{
		Where:    Comparison{Property: query.PropertyIsActive, Operator: Equal, Values: []string{"ready"}},
		Builtins: map[string]bool{query.PropertyIsActive: true},
	}
	_, err = Translate(selected, Context{Index: index, Model: model, Table: table}, "SelectedBuiltin")
	if !errors.As(err, &refused) || refused.Kind != ErrUnparsableValue {
		t.Fatalf("Translate with selected built-in = %v, want an unparsable Boolean operand", err)
	}
}
