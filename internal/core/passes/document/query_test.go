package document_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/document"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

func documentQueryDiagnostics(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	index := libs.NewModelIndex()
	name := "queries.sysml"
	p := parser.New(source.New(name, []byte(`
package Fixture {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	`+body+`
}`)))
	root := p.ParseFile()
	index.AddDocument(name, root)
	index.ExpandWildcardImports()
	return passes.Analyze(name, root, parserDiagnostics(p), index)
}

func TestDocumentQueryPassAcceptsComposableQuery(t *testing.T) {
	diagnostics := documentQueryDiagnostics(t, `
calc def InterfacesFor :> Query {
	in subsystem : Element;
	Descendants(source = subsystem, maxDepth = 2)
}
calc def APSInterfaces :> Query {
	in subsystem : Element;
	InterfacesFor(subsystem = subsystem)
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("valid document queries reported diagnostics: %v", diagnostics)
	}
}

func TestDocumentQueryPassReportsCompositionCycle(t *testing.T) {
	diagnostics := documentQueryDiagnostics(t, `
calc def CycleA :> Query { in subsystem : Element; CycleB(subsystem = subsystem) }
calc def CycleB :> Query { in subsystem : Element; CycleA(subsystem = subsystem) }
`)
	var found bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "document-query-composition-cycle" {
			found = true
			if diagnostic.Source != "document-query" || diagnostic.Span.Len == 0 {
				t.Fatalf("cycle diagnostic lacks typed source location: %+v", diagnostic)
			}
		}
	}
	if !found {
		t.Fatalf("missing composition-cycle diagnostic: %v", diagnostics)
	}
}

func TestDocumentQueryPassIsElementScoped(t *testing.T) {
	_ = document.QueryPass{}.Run(nil, "", nil)

	diagnostics := documentQueryDiagnostics(t, `
part broken : MissingType;
calc def BrokenQuery :> Query { MissingOperation() }
calc def CycleA :> Query { in subsystem : Element; CycleB(subsystem = subsystem) }
calc def CycleB :> Query { in subsystem : Element; CycleA(subsystem = subsystem) }
`)
	var cycle, lowerTier bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "document-query-composition-cycle" {
			cycle = true
		}
		if diagnostic.Source != "document-query" && diagnostic.Blocking() {
			lowerTier = true
		}
		if diagnostic.Source == "document-query" && strings.Contains(diagnostic.Message, "BrokenQuery") {
			t.Fatalf("broken query produced cascading planner diagnostic: %v", diagnostics)
		}
	}
	if !lowerTier {
		t.Fatalf("fixture produced no lower-tier error: %v", diagnostics)
	}
	if !cycle {
		t.Fatalf("unrelated lower-tier error suppressed composition cycle: %v", diagnostics)
	}
}

func TestDocumentQueryPassAcceptsInheritedResultExpression(t *testing.T) {
	diagnostics := documentQueryDiagnostics(t, `
calc def Base :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Specialized :> Base {
	in redefines source;
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("inherited query result reported diagnostics: %v", diagnostics)
	}
}

func TestDocumentQueryPassReportsDependencyErrorInDeclaringDocument(t *testing.T) {
	index := libs.NewModelIndex()
	dependencyName := "dependency.sysml"
	dependencyParser := parser.New(source.New(dependencyName, []byte(`
package Shared {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	calc def Leaf :> Query {
		in source : Element;
		OwnedElements(source = source)
	}
	calc def Broken :> Query {
		in source : Element;
		Leaf(source)
	}
}`)))
	dependencyRoot := dependencyParser.ParseFile()
	callerName := "caller.sysml"
	callerParser := parser.New(source.New(callerName, []byte(`
package Caller {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import Shared::*;
	calc def Entry :> Query {
		in source : Element;
		Broken(source = source)
	}
}`)))
	callerRoot := callerParser.ParseFile()
	index.AddDocument(dependencyName, dependencyRoot)
	index.AddDocument(callerName, callerRoot)
	index.ExpandWildcardImports()

	callerDiagnostics := passes.Analyze(callerName, callerRoot, parserDiagnostics(callerParser), index)
	for _, diagnostic := range callerDiagnostics {
		if diagnostic.Source == "document-query" {
			t.Fatalf("caller received dependency diagnostic: %v", callerDiagnostics)
		}
	}
	dependencyDiagnostics := passes.Analyze(
		dependencyName,
		dependencyRoot,
		parserDiagnostics(dependencyParser),
		index,
	)
	for _, diagnostic := range dependencyDiagnostics {
		if diagnostic.Code == "document-query-positional-query-arguments" {
			return
		}
	}
	t.Fatalf("dependency received no planner diagnostic: %v", dependencyDiagnostics)
}
