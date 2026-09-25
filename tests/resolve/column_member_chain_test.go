package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// resolvedWithLibraries resolves src over an index that holds the bundled
// standard libraries, so document-query vocabulary resolves.
func resolvedWithLibraries(t *testing.T, src string) *resolve.Resolver {
	t.Helper()
	p := parser.New(source.New("app.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("app.sysml", root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.ResolveDocument("app.sysml", root)
	return r
}

const columnChainDoc = `package Doc {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	attribute def Stat {
		attribute runs : Integer;
	}
	part def Sample;
	calc def Report :> Query {
		in root : Element;
		Project(
			source = Descendants(source = root, maxDepth = 1),
			columns = (Column(name = "N", expression = stat.runs ?? 0))
		)
	}
}`

// A feature chain naming members of the row element resolves inside a Column
// expression with no reference to find.
func TestResolveColumnMemberChainIsRowRelative(t *testing.T) {
	r := resolvedWithLibraries(t, columnChainDoc)
	for _, d := range r.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
}

// The same chain outside a Column expression is an ordinary reference and
// still reports its unresolved head.
func TestResolveMemberChainOutsideColumnStillReports(t *testing.T) {
	doc := strings.Replace(columnChainDoc,
		"columns = (Column(name = \"N\", expression = stat.runs ?? 0))",
		"columns = (Column(name = \"N\", expression = 0))",
		1) + `
package Other {
	private import ScalarValues::*;
	attribute probe = stat.runs;
}`
	r := resolvedWithLibraries(t, doc)
	var stat bool
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, "stat") {
			stat = true
		}
	}
	if !stat {
		t.Fatalf("expected an unresolved diagnostic for stat, got %v", r.Diagnostics)
	}
}
