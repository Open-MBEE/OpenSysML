package repl

import (
	"strings"
	"testing"

	_ "github.com/Open-MBEE/OpenSysML/internal/doc/queryexec/elementids"
)

// reservedNameQueryModel names an allocation end `function`, a reserved word a qualifiedName
// spells bare, beside names only an unrestricted name spells, which it keeps quoted.
const reservedNameQueryModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	part def Fn;
	part def Pt;
	allocation def FunctionToPart { end 'function' : Fn; end target : Pt; }
	part def Holder {
		part 'two words' : Fn;
		part 'it\'s' : Fn;
		part 'a::b' : Pt;
		part '@3' : Pt;
		part : Pt;
	}
	calc def Typings :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, relationships = true), type = "FeatureTyping", exact = true),
			properties = ("elementId", "typedFeature")
		)
	}
}`

// TestRunQueryTableSpellsAReservedWordNameAsTheQualifiedNameDoes: `function` reads bare, `'two words'`
// keeps its quotes (KerML 1.1 §8.3.2.1.2 escapedName) and an unnamed part reads by its elementId.
func TestRunQueryTableSpellsAReservedWordNameAsTheQualifiedNameDoes(t *testing.T) {
	s := NewSession()
	if res := s.Submit(reservedNameQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	typings := s.RunDocumentQuery("Typings root=Observatory")
	if !typings.Holds() || typings.Table == nil {
		t.Fatalf("Typings verdict = %s: %v", typings.Status, typings.Lines)
	}
	want := map[string]string{
		"Observatory__FunctionToPart__function_ft0": "Observatory::FunctionToPart::function",
		"Observatory__Holder__two_20words_ft0":      "Observatory::Holder::'two words'",
		"Observatory__Holder___27a__b_27_ft0":       "Observatory::Holder::'a::b'",
		"Observatory__Holder___27_403_27_ft0":       "Observatory::Holder::'@3'",
		"Observatory__Holder___404_ft0":             "Observatory__Holder___404",
	}
	got := map[string]string{}
	for _, row := range typings.Table.Rows {
		got[strings.Trim(cellText(row.Cells, 0), `"`)] = cellText(row.Cells, 1)
	}
	for typing, spelling := range want {
		switch typed, ok := got[typing]; {
		case !ok:
			t.Errorf("no row for %s; rows %v", typing, typings.Table.Rows)
		case typed != spelling:
			t.Errorf("typedFeature of %s = %q, want %q", typing, typed, spelling)
		}
	}
}
