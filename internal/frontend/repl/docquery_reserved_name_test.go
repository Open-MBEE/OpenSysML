package repl

import (
	"strings"
	"testing"

	_ "github.com/Open-MBEE/OpenSysML/internal/doc/queryexec/elementids"
)

// reservedNameQueryModel names an allocation end with a reserved word, `function`, which the
// textual notation must quote but a qualifiedName spells bare, as the pilot and the RDF writer do.
const reservedNameQueryModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	part def Fn;
	part def Pt;
	allocation def FunctionToPart { end 'function' : Fn; end target : Pt; }
	calc def Typings :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, relationships = true), type = "FeatureTyping", exact = true),
			properties = ("elementId", "typedFeature")
		)
	}
}`

// TestRunQueryTableSpellsAReservedWordNameAsTheQualifiedNameDoes: the table spells the end
// `function` as `Observatory::FunctionToPart::function`, the qualifiedName the conversion states,
// not the notation's `'function'`.
func TestRunQueryTableSpellsAReservedWordNameAsTheQualifiedNameDoes(t *testing.T) {
	s := NewSession()
	if res := s.Submit(reservedNameQueryModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	typings := s.RunDocumentQuery("Typings root=Observatory")
	if !typings.Holds() || typings.Table == nil {
		t.Fatalf("Typings verdict = %s: %v", typings.Status, typings.Lines)
	}
	checked := false
	for _, row := range typings.Table.Rows {
		if strings.Trim(cellText(row.Cells, 0), `"`) != "Observatory__FunctionToPart__function_ft0" {
			continue
		}
		checked = true
		if got := cellText(row.Cells, 1); got != "Observatory::FunctionToPart::function" {
			t.Errorf("typedFeature of the end's typing = %q, want the bare qualifiedName the conversion states", got)
		}
	}
	if !checked {
		t.Fatalf("no row for the end function's FeatureTyping; rows %v", typings.Table.Rows)
	}
}
