package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// listedModel names its documents and views as only quoting can: a quote, a
// space, `::` inside a segment and a line break the notation escapes as \n.
const listedModel = `package 'My Pkg' {
	private import DocumentQueries::*;
	private import Views::*;
	private import StandardViewDefinitions::*;

	part def Widget { part cog : Cog; }
	part def Cog { part tooth : Tooth; }
	part def Tooth;
	part def Gear { part rim : Tooth; }

	part def 'Doc "A"' :> Document { attribute redefines title = "Title A"; }
	part def 'a::b doc' :> Document { attribute redefines title = "Title B"; }
	part def 'line\nbreak' :> Document { attribute redefines title = "Title C"; }

	view 'v one' { expose Widget; }
	view 'x::y' : GridView { expose Cog; }
	view 'nl\nview' : InterconnectionView { expose Gear; }
	view geo : GeometryView { expose Widget; }
}
`

const geoReason = `(unsupported: 'My Pkg'::geo: geometry rendering (view def GeometryView) is not supported)`

// listItem is one decoded entry of -list-form json.
type listItem struct {
	Category  string `json:"category"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Supported *bool  `json:"supported"`
	Reason    string `json:"reason"`
	File      string `json:"file"`
	Line      int    `json:"line"`
}

// listRun runs the binary with args as given, the model files among them.
func listRun(t *testing.T, binary string, args ...string) runOutcome {
	t.Helper()
	return runBinary(t, binary, "", args)
}

func listJSON(t *testing.T, got runOutcome) []listItem {
	t.Helper()
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	var items []listItem
	if err := json.Unmarshal([]byte(got.stdout), &items); err != nil {
		t.Fatalf("decode %q: %v", got.stdout, err)
	}
	return items
}

func wantLines(t *testing.T, got runOutcome, want ...string) {
	t.Helper()
	if got.status != 0 {
		t.Fatalf("exit = %d\n%s", got.status, got.output())
	}
	lines := strings.Split(strings.TrimSuffix(got.stdout, "\n"), "\n")
	if !slices.Equal(lines, want) {
		t.Errorf("listed\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

// TestListTargets checks each -list target names what it lists, in qualified-name
// order, one aligned line per item.
func TestListTargets(t *testing.T) {
	binary := buildCLI(t)
	documents := []string{
		`document  'My Pkg'::'Doc "A"'`,
		`document  'My Pkg'::'a::b doc'`,
		`document  'My Pkg'::'line\nbreak'`,
	}
	wantLines(t, check(t, binary, listedModel, "-list", "documents"), documents...)
	wantLines(t, check(t, binary, listedModel, "-list", "views"),
		`view  geometry         'My Pkg'::geo  `+geoReason,
		`view  interconnection  'My Pkg'::'nl\nview'`,
		`view  tree             'My Pkg'::'v one'`,
		`view  table            'My Pkg'::'x::y'`,
	)
	wantLines(t, check(t, binary, listedModel, "-list", "diagrams"),
		`view  interconnection  'My Pkg'::'nl\nview'`,
		`view  tree             'My Pkg'::'v one'`,
	)
	wantLines(t, check(t, binary, listedModel, "-list", "all"),
		`document  'My Pkg'::'Doc "A"'`,
		`document  'My Pkg'::'a::b doc'`,
		`document  'My Pkg'::'line\nbreak'`,
		`view      geometry         'My Pkg'::geo  `+geoReason,
		`view      interconnection  'My Pkg'::'nl\nview'`,
		`view      tree             'My Pkg'::'v one'`,
		`view      table            'My Pkg'::'x::y'`,
	)

	pseudo := check(t, binary, listedModel, "-list", "pseudo-views")
	if pseudo.status != 0 || !strings.Contains(pseudo.stdout, "pseudo-view  tree             #tree\n") ||
		!strings.Contains(pseudo.stdout, "pseudo-view  interconnection  #interconnection\n") {
		t.Errorf("-list pseudo-views: exit = %d\n%s", pseudo.status, pseudo.output())
	}
	without := listRun(t, binary, "-list", "pseudo-views")
	if without.status != 0 || without.stdout != pseudo.stdout {
		t.Errorf("-list pseudo-views without a model: exit = %d\n%s\nwant\n%s", without.status, without.output(), pseudo.stdout)
	}
}

// TestListForms checks the tsv and json forms carry the kind, support, file,
// line, name and reason of each item, leaving out what an item has none of.
func TestListForms(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	path := writeModel(t, dir, "model.sysml", listedModel)

	tsv := listRun(t, binary, path, "-list", "all", "-list-form", "tsv")
	if tsv.status != 0 {
		t.Fatalf("exit = %d\n%s", tsv.status, tsv.output())
	}
	rows := strings.Split(strings.TrimSuffix(tsv.stdout, "\n"), "\n")
	want := []string{
		"category\tkind\tsupported\tfile\tline\tname\treason",
		"document\t\t\t" + path + "\t11\t'My Pkg'::'Doc \"A\"'\t",
		"document\t\t\t" + path + "\t12\t'My Pkg'::'a::b doc'\t",
		"document\t\t\t" + path + "\t13\t'My Pkg'::'line\\nbreak'\t",
		"view\tgeometry\tfalse\t" + path + "\t18\t'My Pkg'::geo\t'My Pkg'::geo: geometry rendering (view def GeometryView) is not supported",
		"view\tinterconnection\ttrue\t" + path + "\t17\t'My Pkg'::'nl\\nview'\t",
		"view\ttree\ttrue\t" + path + "\t15\t'My Pkg'::'v one'\t",
		"view\ttable\ttrue\t" + path + "\t16\t'My Pkg'::'x::y'\t",
	}
	if !slices.Equal(rows, want) {
		t.Errorf("tsv\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}

	items := listJSON(t, listRun(t, binary, path, "-list", "all", "-list-form", "json"))
	if len(items) != 7 {
		t.Fatalf("json lists %d items, want 7: %+v", len(items), items)
	}
	doc := items[2]
	if doc.Category != "document" || doc.Name != `'My Pkg'::'line\nbreak'` || doc.File != path || doc.Line != 13 ||
		doc.Kind != "" || doc.Supported != nil || doc.Reason != "" {
		t.Errorf("document item %+v", doc)
	}
	geo := items[3]
	if geo.Category != "view" || geo.Kind != "geometry" || geo.Supported == nil || *geo.Supported ||
		!strings.Contains(geo.Reason, "is not supported") || geo.Line != 18 {
		t.Errorf("unsupported view item %+v", geo)
	}
	tree := items[5]
	if tree.Kind != "tree" || tree.Supported == nil || !*tree.Supported || tree.Reason != "" {
		t.Errorf("supported view item %+v", tree)
	}
	raw := listRun(t, binary, path, "-list", "documents", "-list-form", "json").stdout
	for _, absent := range []string{`"kind"`, `"supported"`, `"reason"`} {
		if strings.Contains(raw, absent) {
			t.Errorf("a document item writes the empty field %s:\n%s", absent, raw)
		}
	}
}

// TestListAcrossFiles checks each item names the file it was declared in.
func TestListAcrossFiles(t *testing.T) {
	binary := buildCLI(t)
	got := checkFiles(t, binary, map[string]string{
		"a.sysml": "package A {\n\tprivate import DocumentQueries::*;\n\tpart def Report :> Document { attribute redefines title = \"R\"; }\n}\n",
		"b.sysml": "package B {\n\tpart def P;\n\n\tview pv { expose P; }\n}\n",
	}, "-list", "all", "-list-form", "json")
	items := listJSON(t, got)
	if len(items) != 2 {
		t.Fatalf("listed %+v", items)
	}
	if items[0].Name != "A::Report" || filepath.Base(items[0].File) != "a.sysml" || items[0].Line != 3 {
		t.Errorf("document item %+v", items[0])
	}
	if items[1].Name != "B::pv" || filepath.Base(items[1].File) != "b.sysml" || items[1].Line != 4 {
		t.Errorf("view item %+v", items[1])
	}
}

// TestListKindFilter checks -list-kind keeps the views of the kinds it names and
// is refused where there is nothing to filter or a kind is unknown.
func TestListKindFilter(t *testing.T) {
	binary := buildCLI(t)
	wantLines(t, check(t, binary, listedModel, "-list", "views", "-list-kind", "table,geometry"),
		`view  geometry  'My Pkg'::geo  `+geoReason,
		`view  table     'My Pkg'::'x::y'`,
	)
	wantLines(t, check(t, binary, listedModel, "-list", "diagrams", "-list-kind", "tree,table"),
		`view  tree  'My Pkg'::'v one'`,
	)
	wantLines(t, check(t, binary, listedModel, "-list", "all", "-list-kind", "interconnection"),
		`document  'My Pkg'::'Doc "A"'`,
		`document  'My Pkg'::'a::b doc'`,
		`document  'My Pkg'::'line\nbreak'`,
		`view      interconnection  'My Pkg'::'nl\nview'`,
	)
	if got := check(t, binary, listedModel, "-list", "views", "-list-kind", "state"); got.status != 0 || got.stdout != "" {
		t.Errorf("no view of the kind: exit = %d stdout = %q", got.status, got.stdout)
	}

	wantReport(t, check(t, binary, listedModel, "-list", "views", "-list-kind", "bogus"),
		2, `unknown view kind "bogus"`, "tree, interconnection, state, action")
	wantReport(t, check(t, binary, listedModel, "-list", "documents", "-list-kind", "tree"),
		2, "-list documents has none to filter")
	wantReport(t, check(t, binary, listedModel, "-list", "pseudo-views", "-list-kind", "tree"),
		2, "-list pseudo-views has none to filter")
}

// TestListRefusals checks -list refuses what it cannot list and what it cannot
// be combined with.
func TestListRefusals(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, check(t, binary, listedModel, "-list", "bogus"), 2, `unknown listing "bogus"`, "documents, views, diagrams, pseudo-views, all")
	wantReport(t, check(t, binary, listedModel, "-list", ""), 2, "-list is empty")
	wantReport(t, check(t, binary, listedModel, "-list", "views", "-list-form", "xml"), 2, `unknown listing form "xml"`, "text, tsv, json")
	wantReport(t, check(t, binary, listedModel, "-list-form", "json"), 2, "name what to list")
	wantReport(t, check(t, binary, listedModel, "-list-kind", "tree"), 2, "name what to list")
	wantReport(t, listRun(t, binary, "-list", "views"), 2, "no model to list")

	for _, args := range [][]string{
		{"-render", "'My Pkg'::'v one'"},
		{"-render-all", "out"},
		{"-render-document", "'My Pkg'::'Doc \"A\"'"},
		{"-render-documents", "out"},
	} {
		wantReport(t, check(t, binary, listedModel, append([]string{"-list", "views"}, args...)...), 2, "ask for one per run")
	}
	for _, args := range [][]string{{"-convert", "ttl"}, {"-migrate", "sysml"}} {
		wantReport(t, check(t, binary, listedModel, append([]string{"-list", "views"}, args...)...), 2, "ask for one per run")
	}
	wantReport(t, check(t, binary, listedModel, "-list", "views", "-query", "parts"), 2, "cannot be combined with -query")
	wantReport(t, check(t, binary, listedModel, "-list", "views", "-o", "out.txt"), 2, "cannot be combined with -output")
	wantReport(t, check(t, binary, listedModel, "-list", "views", "-constraint", "C"), 2, "check it in its own run")
	wantReport(t, check(t, binary, listedModel, "-list", "views", "-validate"), 2, "check it in its own run")
}

// TestListEmptyModel checks a model declaring nothing to list writes nothing in
// every form and succeeds.
func TestListEmptyModel(t *testing.T) {
	binary := buildCLI(t)
	for _, what := range []string{"documents", "views", "diagrams", "all"} {
		for _, form := range []string{"text", "tsv", "json"} {
			got := check(t, binary, "package Empty { part def P; }\n", "-list", what, "-list-form", form)
			if got.status != 0 || got.stdout != "" {
				t.Errorf("-list %s -list-form %s: exit = %d stdout = %q", what, form, got.status, got.stdout)
			}
		}
	}
}

// TestListModelThatDoesNotAnalyse checks a model with errors stops the run with
// status 2 and its diagnostics, as -render-documents does.
func TestListModelThatDoesNotAnalyse(t *testing.T) {
	binary := buildCLI(t)
	got := check(t, binary, "package Broken { part p : Missing; }\n", "-list", "documents")
	wantReport(t, got, 2, "Missing", "did not analyse cleanly; nothing was listed")
	if got.stdout != "" {
		t.Errorf("stdout = %q, want nothing", got.stdout)
	}
}

// TestListNamesRoundTrip checks every name -list prints, passed back unchanged,
// resolves through -render-document or -render to the element it names.
func TestListNamesRoundTrip(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	path := writeModel(t, dir, "model.sysml", listedModel)

	titles := map[string]string{
		`'My Pkg'::'Doc "A"'`:     "# Title A",
		`'My Pkg'::'a::b doc'`:    "# Title B",
		`'My Pkg'::'line\nbreak'`: "# Title C",
	}
	exposed := map[string]string{
		`'My Pkg'::'v one'`:    "part def 'My Pkg'::Widget",
		`'My Pkg'::'x::y'`:     "'My Pkg'::Cog",
		`'My Pkg'::'nl\nview'`: "part def 'My Pkg'::Gear",
	}
	items := listJSON(t, listRun(t, binary, path, "-list", "all", "-list-form", "json"))
	resolved := 0
	for _, item := range items {
		switch {
		case item.Category == "document":
			got := listRun(t, binary, path, "-render-document", item.Name)
			if got.status != 0 || !strings.Contains(got.stdout, titles[item.Name]+"\n") || titles[item.Name] == "" {
				t.Errorf("-render-document %s: exit = %d\n%s", item.Name, got.status, got.output())
			}
			resolved++
		case *item.Supported:
			got := listRun(t, binary, path, "-render", item.Name, "-render-form", "text")
			if got.status != 0 || !strings.HasPrefix(got.stdout, item.Name+" - ") || exposed[item.Name] == "" ||
				!strings.Contains(got.stdout, exposed[item.Name]) {
				t.Errorf("-render %s: exit = %d\n%s", item.Name, got.status, got.output())
			}
			resolved++
		default:
			got := listRun(t, binary, path, "-render", item.Name, "-render-form", "text")
			if got.status == 0 || !strings.Contains(got.stderr, item.Reason) {
				t.Errorf("-render %s of an unsupported view: exit = %d\n%s", item.Name, got.status, got.output())
			}
		}
	}
	if resolved != len(titles)+len(exposed) {
		t.Errorf("resolved %d names, want %d", resolved, len(titles)+len(exposed))
	}

	text := listRun(t, binary, path, "-list", "all")
	if strings.Count(text.stdout, "\n") != len(items) {
		t.Errorf("text listing is not one line per item:\n%s", text.stdout)
	}
}
