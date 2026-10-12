package repl

import (
	"slices"
	"strings"
	"testing"
)

const listingModel = `package 'My Pkg' {
	private import DocumentQueries::*;
	private import Views::*;
	private import StandardViewDefinitions::*;

	part def Widget { part cog : Cog; }
	part def Cog;
	part def 'Doc "A"' :> Document { attribute redefines title = "A"; }
	part def 'line\nbreak' :> Document { attribute redefines title = "B"; }

	view 'v one' { expose Widget; }
	view 'x::y' : GridView { expose Widget; }
	view states : StateTransitionView { expose Widget; }
	view geo : GeometryView { expose Widget; }
}
`

func listingSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(listingModel); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	return s
}

// %documents lists the documents by the names %render-document reads, in the
// text form -list writes.
func TestDocumentsCommand(t *testing.T) {
	s := listingSession(t)
	got := run(t, s, "%documents")
	want := strings.Join([]string{
		`document  'My Pkg'::'Doc "A"'`,
		`document  'My Pkg'::'line\nbreak'`,
	}, "\n")
	if got != want {
		t.Errorf("%%documents =\n%s\nwant\n%s", got, want)
	}
	if got != strings.Join(ListText(s.ListDocuments()), "\n") {
		t.Errorf("%%documents does not write the listing's text form:\n%s", got)
	}
	if !strings.Contains(run(t, s, `%render-document 'My Pkg'::'line\nbreak'`), "# B") {
		t.Error("a listed document name does not resolve through %render-document")
	}
	wants(t, run(t, NewSession(), "%documents"), "(no documents)")
}

// %views lists every view, the graph-shaped ones under diagrams, and those of
// the kinds named; an unknown kind is refused with the kinds there are.
func TestViewsCommand(t *testing.T) {
	s := listingSession(t)
	all := run(t, s, "%views")
	want := strings.Join([]string{
		`view  geometry  'My Pkg'::geo  (unsupported: 'My Pkg'::geo: geometry rendering (view def GeometryView) is not supported)`,
		`view  state     'My Pkg'::states`,
		`view  tree      'My Pkg'::'v one'`,
		`view  table     'My Pkg'::'x::y'`,
	}, "\n")
	if all != want {
		t.Errorf("%%views =\n%s\nwant\n%s", all, want)
	}
	if all != strings.Join(ListText(s.ListViews(false, nil)), "\n") {
		t.Errorf("%%views does not write the listing's text form:\n%s", all)
	}
	if got := run(t, s, "%views diagrams"); got != "view  state  'My Pkg'::states\nview  tree   'My Pkg'::'v one'" {
		t.Errorf("%%views diagrams =\n%s", got)
	}
	if got := run(t, s, "%views table geometry"); !strings.Contains(got, "'My Pkg'::'x::y'") ||
		!strings.Contains(got, "'My Pkg'::geo") || strings.Contains(got, "'v one'") {
		t.Errorf("%%views table geometry =\n%s", got)
	}
	if got := run(t, s, "%views diagrams table"); got != "(no views)" {
		t.Errorf("%%views diagrams table =\n%s", got)
	}
	wants(t, run(t, s, "%views bogus"), `unknown view kind "bogus"`, "diagrams or tree, interconnection", viewsUsage)
	if !strings.Contains(run(t, s, `%render 'My Pkg'::'x::y'`), "table rendering") {
		t.Error("a listed view name does not resolve through %render")
	}
}

// %documents and %views are listed in %help's session group and completed, the
// %views kinds with them.
func TestListingCommandsInHelpAndCompletion(t *testing.T) {
	s := NewSession()
	help := run(t, s, "%help")
	session := help[strings.Index(help, groupSession):]
	if next := strings.Index(session[len(groupSession):], ":\n"); next >= 0 {
		session = session[:len(groupSession)+next]
	}
	wants(t, session, "%documents", "%views [diagrams] [<kind>...]")
	for prefix, want := range map[string]string{"%docu": "%documents", "%vie": "%views"} {
		if comp := s.Complete(prefix, len(prefix)); !slices.Contains(comp.Candidates, want) {
			t.Errorf("completing %q: want %q in %v", prefix, want, comp.Candidates)
		}
	}
	line := "%views "
	comp := s.Complete(line, len(line))
	for _, want := range []string{"diagrams", "tree", "state", "action", "table"} {
		if !slices.Contains(comp.Candidates, want) {
			t.Errorf("completing %q: want %q in %v", line, want, comp.Candidates)
		}
	}
	line = "%views st"
	if comp := s.Complete(line, len(line)); !slices.Equal(comp.Candidates, []string{"state"}) {
		t.Errorf("completing %q: %v, want [state]", line, comp.Candidates)
	}
}
