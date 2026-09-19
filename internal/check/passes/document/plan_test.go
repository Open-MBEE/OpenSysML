package document_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/document"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func documentPlanDiagnostics(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	index := libs.NewModelIndex()
	name := "documents.sysml"
	p := parser.New(source.New(name, []byte(`
package Fixture {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	`+body+`
}`)))
	root := p.ParseFile()
	index.AddDocument(name, root)
	index.ExpandWildcardImports()
	return passes.Analyze(name, root, parserDiagnostics(p), index)
}

func TestDocumentPlanPassAcceptsValidDocument(t *testing.T) {
	diagnostics := documentPlanDiagnostics(t, `
part telescope;
calc def Names :> Query {
	in root : Element;
	OwnedElements(source = root)
}
part def Report :> Document {
	attribute redefines title = "Report";
	part intro : Paragraph {
		attribute redefines text = "Overview.";
	}
	part body : Section {
		attribute redefines title = "Body";
		part items : List {
			calc names : Names {
				in root = telescope;
			}
		}
	}
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("valid document reported diagnostics: %v", diagnostics)
	}
}

func TestDocumentPlanPassReportsMissingTitle(t *testing.T) {
	diagnostics := documentPlanDiagnostics(t, `
part def Report :> Document {
	part intro : Paragraph {
		attribute redefines text = "Overview.";
	}
}
`)
	var found bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "document-plan-missing-title" {
			found = true
			if diagnostic.Source != "document-plan" || diagnostic.Span.Len == 0 {
				t.Fatalf("missing-title diagnostic lacks typed source location: %+v", diagnostic)
			}
		}
	}
	if !found {
		t.Fatalf("missing missing-title diagnostic: %v", diagnostics)
	}
}

func TestDocumentPlanPassIsElementScoped(t *testing.T) {
	_ = document.PlanPass{}.Run(nil, "", nil)

	diagnostics := documentPlanDiagnostics(t, `
part broken : MissingType;
part def BrokenReport :> Document {
	attribute redefines title = "Broken";
	part stray : MissingType;
}
part def Report :> Document {
	part intro : Paragraph {
		attribute redefines text = "Overview.";
	}
}
`)
	var missingTitle, lowerTier bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "document-plan-missing-title" {
			missingTitle = true
		}
		if diagnostic.Source != "document-plan" && diagnostic.Blocking() {
			lowerTier = true
		}
		if diagnostic.Source == "document-plan" && strings.Contains(diagnostic.Message, "BrokenReport") {
			t.Fatalf("broken document produced cascading planner diagnostic: %v", diagnostics)
		}
	}
	if !lowerTier {
		t.Fatalf("fixture produced no lower-tier error: %v", diagnostics)
	}
	if !missingTitle {
		t.Fatalf("unrelated lower-tier error suppressed missing-title diagnostic: %v", diagnostics)
	}
}

func TestDocumentPlanPassReportsBindingErrors(t *testing.T) {
	diagnostics := documentPlanDiagnostics(t, `
part telescope;
calc def Names :> Query {
	in root : Element;
	OwnedElements(source = root)
}
part def Report :> Document {
	attribute redefines title = "Report";
	part items : List {
		calc names : Names {
			in root = telescope;
			in bogus = telescope;
		}
	}
}
`)
	var found bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "document-plan-unknown-parameter" {
			found = true
			if diagnostic.Span.Len == 0 {
				t.Fatalf("unknown-parameter diagnostic lacks source location: %+v", diagnostic)
			}
		}
	}
	if !found {
		t.Fatalf("missing unknown-parameter diagnostic: %v", diagnostics)
	}
}
