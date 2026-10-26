package migrate_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// migrateLayoutFixture migrates testdata/xmi/layout.xmi augmented by the MTIP
// export layout.layout.xml.
func migrateLayoutFixture(t *testing.T) *migrate.Result {
	t.Helper()
	return migrateLaidOut(t, "layout")
}

// migrateLaidOut migrates testdata/xmi/<name>.xmi augmented by the MTIP export
// <name>.layout.xml beside it.
func migrateLaidOut(t *testing.T, name string) *migrate.Result {
	return migrateLaidOutOptions(t, name, migrate.Options{})
}

func migrateLaidOutOptions(t *testing.T, name string, opts migrate.Options) *migrate.Result {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/" + name + ".xmi")
	if err != nil {
		t.Fatal(err)
	}
	layoutData, err := os.ReadFile("testdata/xmi/" + name + ".layout.xml")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := mtip.Parse(layoutData)
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	opts.Layout = layout
	opts.LayoutSource = name + ".layout.xml"
	r, err := migrate.MigrateOptions(name+".xmi", data, opts)
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	return r
}

func migrateStreamLaidOut(t *testing.T, name string, strict bool) *migrate.Result {
	t.Helper()
	archive := streamArchive(t, name)
	layoutData, err := os.ReadFile("testdata/xmi/" + name + ".layout.xml")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := mtip.Parse(layoutData)
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	r, err := migrate.MigrateOptions(name+".xmi", archive, migrate.Options{
		Layout: layout, LayoutSource: name + ".layout.xml", Strict: strict,
	})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	return r
}

// migrateStream migrates a fixture whose diagrams are laid out from their own
// symbol streams alone, with no export record.
func migrateStream(t *testing.T, name string) *migrate.Result {
	t.Helper()
	r, err := migrate.MigrateOptions(name+".xmi", streamArchive(t, name), migrate.Options{})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	return r
}

// streamArchive zips a fixture's XMI with the symbol stream its diagram names.
func streamArchive(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/xmi/" + name + ".xmi")
	if err != nil {
		t.Fatal(err)
	}
	return streamArchiveOf(t, name, data)
}

// streamArchiveOf zips the given XMI, standing in for a fixture's, with that fixture's streams.
func streamArchiveOf(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	type entry struct {
		name string
		data []byte
	}
	entries := []entry{{name: name + ".xmi", data: data}}
	for _, pattern := range []string{name + ".stream.xml", name + ".*.stream.xml"} {
		streams, err := filepath.Glob("testdata/xmi/" + pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range streams {
			stream, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			entries = append(entries, entry{name: "BINARY-" + strings.TrimSuffix(filepath.Base(path), ".stream.xml"), data: stream})
		}
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for _, entry := range entries {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

// The notation and report of the augmented migration are pinned, and the
// notation analyses clean — the DiagramLayout annotations must type-check.
func TestGoldenLayout(t *testing.T) {
	r := migrateLayoutFixture(t)
	wantLine(t, r.Notation, ":>> DiagramLayout::Layout::height = 80;")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about engine { x = 20; y = 10; width = 100; :>> DiagramLayout::Layout::height = 40; }")
	checkGolden(t, "testdata/xmi/layout.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/layout.layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "layout.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	strict := migrateLaidOutOptions(t, "layout", migrate.Options{Strict: true})
	for _, d := range errorsMode(t, "layout.sysml", strict.Notation, diag.ConformanceStrict) {
		t.Errorf("%v", d)
	}
	// The strict migration joins the same records to the same views; only
	// their geometry is left unwritten, which the summary says instead of
	// reporting the views as missing.
	l, sl := r.Report.Layout, strict.Report.Layout
	if sl.DiagramsJoined != l.DiagramsJoined || sl.DiagramsUnmatched != l.DiagramsUnmatched || sl.ViewsWithoutLayout != l.ViewsWithoutLayout || sl.GeometryOmitted != l.DiagramsJoined || sl.PlacementsWritten != 0 {
		t.Errorf("strict layout summary: %+v\nwant joined %d, unmatched %d, without %d, all geometry omitted", sl, l.DiagramsJoined, l.DiagramsUnmatched, l.ViewsWithoutLayout)
	}
	if l.GeometryOmitted != 0 {
		t.Errorf("default layout summary omits geometry: %+v", l)
	}
	for _, e := range strict.Report.Entries {
		if e.Kind == "Layout" && strings.Contains(e.Note, "does not write as a view") {
			t.Errorf("strict migration reports a joined view missing: %+v", e)
		}
	}
	if !strings.Contains(strict.Report.Summary(), "geometry the strict migration omits") {
		t.Errorf("strict summary = %q", strict.Report.Summary())
	}
}

func TestGoldenLayoutExposedMetadataAttribute(t *testing.T) {
	r := migrateStreamLaidOut(t, "layout_exposed_shadow", false)
	if l := r.Report.Layout; l == nil || l.DiagramsJoined != 1 || l.PlacementsWritten != 2 || l.RoutesWritten != 1 {
		t.Fatalf("layout summary: %+v", l)
	}
	checkGolden(t, "testdata/xmi/layout_exposed_shadow.layout.golden.sysml", r.Notation)
	for _, d := range errors(t, "layout_exposed_shadow.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	for _, want := range []string{
		"expose Structure::Scale::height;",
		"expose Structure::Scale::text;",
		"expose Structure::Scale::width;",
		":>> DiagramLayout::Layout::height = 40",
		":>> DiagramLayout::Route::points = (",
		":>> DiagramLayout::Canvas::height = 100",
		":>> DiagramLayout::Style::text = ",
		":>> DiagramLayout::Picture::width = ",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("layout output does not contain %q:\n%s", want, notation)
		}
	}
}

func TestGoldenStreamPictureQualifiesShadowedLayoutField(t *testing.T) {
	r := migrateStreamLaidOut(t, "stream_layout_shadow", false)
	if !strings.Contains(string(r.Notation), ":>> DiagramLayout::Picture::height = 40") {
		t.Errorf("stream picture does not qualify its shadowed height field:\n%s", r.Notation)
	}
	checkGolden(t, "testdata/xmi/stream_layout_shadow.layout.golden.sysml", r.Notation)
	for _, d := range errors(t, "stream_layout_shadow.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

func TestGoldenLayoutWildcardExposedMetadataAttribute(t *testing.T) {
	r := migrateLaidOut(t, "layout_wildcard_exposed_shadow")
	if l := r.Report.Layout; l == nil || l.DiagramsJoined != 1 || l.PlacementsWritten != 2 {
		t.Fatalf("layout summary: %+v", l)
	}
	checkGolden(t, "testdata/xmi/layout_wildcard_exposed_shadow.layout.golden.sysml", r.Notation)
	for _, d := range errors(t, "layout_wildcard_exposed_shadow.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	for _, want := range []string{
		"expose Structure::**;",
		":>> DiagramLayout::Layout::height = ",
		"x = 10;",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("layout output does not contain %q:\n%s", want, notation)
		}
	}
}

// Every edge kind a diagram draws is routed once its member is named and exposed, every
// connector record not routed is accounted for by kind and reason, and the notation analyses clean.
func TestGoldenEdgeLayout(t *testing.T) {
	r := migrateLaidOut(t, "diagram_edges")
	checkGolden(t, "testdata/xmi/diagram_edges.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/diagram_edges.layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "diagram_edges.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Routes != 24 || l.RoutesWritten != 17 || l.RoutesUnexposed != 6 || l.RoutesDangling != 1 {
		t.Errorf("routes: %+v", l)
	}
	// An edge of the graph the activity's rendering does not draw is exposed, not swallowed.
	if !strings.Contains(string(r.Notation), "expose 'set speed.value = target';") {
		t.Errorf("the activity view does not expose the binding its rendering does not draw:\n%s", r.Notation)
	}
	// A flow several edges carry is named for the shown one, though an unshown one is written first.
	if !strings.Contains(string(r.Notation), "Route about 'gain.result to set speed.value'") {
		t.Errorf("the shown twin of a flow written once is not routed:\n%s", r.Notation)
	}
	// A transition written once per trigger routes every transition it was written as.
	if !strings.Contains(string(r.Notation), "Route about 'Halted accept Go then Running', 'Halted accept Resume then Running'") {
		t.Errorf("the two-trigger transition is not routed as both of its transitions:\n%s", r.Notation)
	}
	// A relationship written once per client–supplier pair exposes every pair, each in
	// the body it was written in, though the tree rendering draws none of them.
	for _, want := range []string{
		"expose 'Engine to Component';\n",
		"expose 'Wheel to Component';\n",
		"expose Structure::Vehicle::'satisfy Speed Limit';\n",
		"expose Structure::Engine::'satisfy Speed Limit';\n",
	} {
		if !strings.Contains(string(r.Notation), want) {
			t.Errorf("a pair of a relationship written per pair is not exposed: %s\n%s", want, r.Notation)
		}
	}
	// An activity diagram showing none of its activity's graph is not a graph view of it:
	// it exposes nothing rather than drawing the whole activity.
	if !strings.Contains(string(r.Notation), "view 'Driving Sketch' {\n                    render Views::asTextualNotation;\n                }") {
		t.Errorf("the empty activity diagram is not an empty textual view:\n%s", r.Notation)
	}
	// A transition, drawn as an edge, positions no node; a declared final is placed, and
	// an initial node written as start is placed as the activity's start.
	if l.PlacementsUnexposed != 2 || strings.Contains(string(r.Notation), "Layout about halt") || !strings.Contains(string(r.Notation), "Layout about final {") || !strings.Contains(string(r.Notation), "Layout about Drive::start {") {
		t.Errorf("placements: %+v", l)
	}
	wantKinds := []migrate.RouteKind{
		{Kind: "Association", Reason: "written", Count: 1},
		{Kind: "AssociationClass", Reason: "not drawn", Count: 1},
		{Kind: "BindingConnector", Reason: "written", Count: 1},
		{Kind: "Connector", Reason: "unnamed", Count: 1},
		{Kind: "Connector", Reason: "written", Count: 3},
		{Kind: "ControlFlow", Reason: "unnamed", Count: 1},
		{Kind: "ControlFlow", Reason: "written", Count: 2},
		{Kind: "Dependency", Reason: "written", Count: 2},
		{Kind: "Generalization", Reason: "written", Count: 1},
		{Kind: "Include", Reason: "written", Count: 1},
		{Kind: "ObjectFlow", Reason: "not drawn", Count: 1},
		{Kind: "ObjectFlow", Reason: "written", Count: 1},
		{Kind: "Satisfy", Reason: "not exposed", Count: 1},
		{Kind: "Satisfy", Reason: "written", Count: 1},
		{Kind: "Transition", Reason: "dangling", Count: 1},
		{Kind: "Transition", Reason: "no v2 member", Count: 1},
		{Kind: "Transition", Reason: "written", Count: 3},
		{Kind: "Verify", Reason: "written", Count: 1},
	}
	if !reflect.DeepEqual(l.RoutesByKind, wantKinds) {
		t.Errorf("routes by kind:\n got %+v\nwant %+v", l.RoutesByKind, wantKinds)
	}
	// The view text is the same with or without the export: naming never reads it.
	plain := migrateFixtureFile(t, "diagram_edges")
	var stripped []string
	for _, line := range strings.Split(string(r.Notation), "\n") {
		if !strings.Contains(line, "DiagramLayout::") {
			stripped = append(stripped, line)
		}
	}
	if got := strings.Join(stripped, "\n"); got != string(plain.Notation) {
		t.Errorf("the laid-out notation differs from the plain one beyond its DiagramLayout annotations:\n%s", got)
	}
}

func TestGoldenRefusedVertexLayout(t *testing.T) {
	r := migrateStreamLaidOut(t, "refused_vertex_layout", false)
	checkGolden(t, "testdata/xmi/refused_vertex_layout.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/refused_vertex_layout.layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "refused_vertex_layout.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	for _, kind := range []string{"Style", "Note"} {
		for _, node := range []string{"::junction", "::start"} {
			found := false
			for _, line := range strings.Split(string(r.Notation), "\n") {
				if strings.Contains(line, "DiagramLayout::"+kind+" about ") && strings.Contains(line, node) {
					found = true
				}
			}
			if !found {
				t.Errorf("non-strict output has no %s about %s:\n%s", kind, node, r.Notation)
			}
		}
	}
	if l := r.Report.Layout; l == nil || l.Styles != 2 || l.StylesWritten != 2 || l.Notes != 2 || l.NotesAnchored != 2 || l.NotesFreed != 0 {
		t.Errorf("the styled initial pseudostate and the note anchored to it are not counted as written: %+v", l)
	}
}

func TestStrictRefusedVertexLayoutHasNoPseudostateReferences(t *testing.T) {
	const name = "refused_vertex_layout"
	r := migrateStreamLaidOut(t, name, true)
	for _, d := range errorsMode(t, name+".sysml", r.Notation, diag.ConformanceStrict) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	for _, line := range strings.Split(notation, "\n") {
		if !strings.Contains(line, "DiagramLayout::") {
			continue
		}
		target, ok := strings.CutPrefix(line, "metadata ")
		if !ok {
			continue
		}
		_, target, ok = strings.Cut(target, " about ")
		if !ok {
			continue
		}
		target, _, _ = strings.Cut(target, " {")
		if strings.HasSuffix(strings.TrimSpace(target), "::junction") ||
			strings.HasSuffix(strings.TrimSpace(target), "::choice") {
			t.Errorf("strict output has DiagramLayout metadata about a refused pseudostate:\n%s", line)
		}
	}
}

// A control node the migrator declares as a member is placed; the initial and flow final
// nodes, written as start and done, are placed as the activity's start and done.
func TestGoldenControlNodeLayout(t *testing.T) {
	r := migrateLaidOut(t, "control_nodes")
	checkGolden(t, "testdata/xmi/control_nodes.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/control_nodes.layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "control_nodes.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Placements != 8 || l.PlacementsWritten != 8 || l.PlacementsUnexposed != 0 {
		t.Errorf("placements: %+v", l)
	}
	if l.Routes != 8 || l.RoutesWritten != 8 {
		t.Errorf("routes: %+v", l)
	}
	for _, want := range []string{
		"Layout about Control::Controller::Run::start {", "Layout about 'fork' {", "Layout about 'join' {", "Layout about check {", "Layout about final {", "Layout about Control::Controller::Run::done {",
	} {
		if !strings.Contains(string(r.Notation), want) {
			t.Errorf("a declared control node is not placed: %s\n%s", want, r.Notation)
		}
	}
	s := session(t, r)
	rendering, err := s.ViewRendering("Control::Controller::Run::Run")
	if err != nil {
		t.Fatal(err)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{`xlabel="'fork'"`, `xlabel="final"`, "'start to fork'", "«initial»", "«final»"} {
		if strings.Contains(dot, unwanted) {
			t.Errorf("DOT of the activity view draws the name %s the migration made up:\n%s", unwanted, dot)
		}
	}
	for _, want := range []string{
		"// layout: neato -n2\n",
		`"n1" [fillcolor=black, label="", pos="100,217!", pin=true, width=1.6666666666666667, height=0.08333333333333333, fixedsize=true];`,
		`"n5" [shape=diamond, label="", xlabel="check", pos="100,70!", pin=true, width=0.2777777777777778, height=0.2777777777777778, fixedsize=true];`,
		`"n6" [shape=doublecircle, fillcolor=black, label="", pos="50,10!", pin=true, width=0.2777777777777778, height=0.2777777777777778, fixedsize=true];`,
		`"n7" [shape=circle, fillcolor=black, label="", pos="100,260!", pin=true, width=0.2777777777777778, height=0.2777777777777778, fixedsize=true];`,
		`"n8" [shape=doublecircle, fillcolor=black, label="", pos="150,10!", pin=true, width=0.2777777777777778, height=0.2777777777777778, fixedsize=true];`,
		`"n7" -> "n1" [style=dashed, arrowhead=vee, pos="e,100,220 100,250 100,250 100,230 100,230"];`,
		`"n5" -> "n8" [label="[false]", style=dashed, arrowhead=vee, pos="e,150,20 110,70 110,70 150,70 150,70 150,70 150,30 150,30", lp="118.5,45"];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT of the activity view lacks %q:\n%s", want, dot)
		}
	}
}

// The routes a migrated view carries reach the DOT form as pinned edge splines for each
// edge kind, run from source to target however the export listed the line's points, and
// labelled by name only where the edge has no text of its own and the name is the
// source's: one the migration made up, and marked, labels nothing. An object flow runs
// pin to pin, the pins named beside the action; a label sits inside the canvas, and two
// labels on one route, or a label and a parallel line, keep apart.
func TestMigratedRoutesRenderPinned(t *testing.T) {
	r := migrateLaidOut(t, "diagram_edges")
	s := session(t, r)
	everything := view.Options{Show: view.AllShown()}
	for view, wants := range map[string][]string{
		"Structure::Vehicle::Drive::Driving": {
			`"n4" -> "n1" [style=dashed, arrowhead=vee, pos="e,60,180 60,210 60,210 60,190 60,190"];`,
			`"n2" -> "n3" [label="finish", style=dashed, arrowhead=vee, pos="e,60,20 60,60 60,60 60,30 60,30", lp="87.5,40"];`,
			`"n1.0" [shape=box, label="", xlabel="result", fontsize=8, width=0.16666666666666666, height=0.16666666666666666, fixedsize=true, pos="116,160!", pin=true];`,
			`"n2.0" [shape=box, label="", xlabel="value", fontsize=8, width=0.16666666666666666, height=0.16666666666666666, fixedsize=true, pos="116,80!", pin=true];`,
			`"n1.0" -> "n2.0" [pos="e,110,80 110,160 110,160 140,160 140,160 140,160 140,80 140,80 140,80 120,80 120,80"];`,
		},
		"Behavior::Modes::Modes": {
			`"n1" -> "n2" [label="accept Go", arrowhead=vee, pos="e,200,110 110,110 110,110 190,110 190,110", lp="155,122"];`,
			`"n2" -> "n3" [label="accept Stop", arrowhead=vee, pos="e,250,40 250,90 250,90 250,50 250,50", lp="203,65"];`,
			`"n3" -> "n2" [label="accept Go", arrowhead=vee, pos="e,280,90 280,40 280,40 320,40 320,40 320,40 320,90 320,90 320,90 290,90 290,90", lp="281,65"];`,
			`"n3" -> "n2" [label="accept Resume", arrowhead=vee, pos="e,280,90 280,40 280,40 320,40 320,40 320,40 320,90 320,90 320,90 290,90 290,90", lp="265.5,45.5"];`,
		},
		"Structure::Vehicle::'Vehicle Internals'": {
			`[label="connection", arrowhead=none, penwidth=3, pos="200,100 200,100 120,100 120,100", lp="160,112"];`,
			`[label="connection", arrowhead=none, penwidth=3, pos="200,80 200,80 160,60 160,60 160,60 120,80 120,80", lp="192.5,44.5"];`,
			`[label="drive", arrowhead=none, penwidth=3, pos="200,90 200,90 120,90 120,90", lp="160,78"];`,
			`[label="binding", arrowhead=none, pos="200,10 200,10 120,10 120,10", lp="160,22"];`,
		},
	} {
		rendering, err := s.ViewRendering(view)
		if err != nil {
			t.Fatalf("render %s: %v", view, err)
		}
		dot, err := rendering.DOTWith(everything)
		if err != nil {
			t.Fatalf("DOT of %s: %v", view, err)
		}
		for _, want := range wants {
			if !strings.Contains(dot, want) {
				t.Errorf("DOT of %s lacks %q:\n%s", view, want, dot)
			}
		}
	}
}

// Zero options must migrate exactly as Migrate does: notation and text report
// byte-identical for every fixture.
func TestZeroOptionsMatchMigrate(t *testing.T) {
	for _, name := range append(append([]string{"vehicle"}, constructFixtures...), "layout") {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/xmi/" + name + ".xmi")
			if err != nil {
				t.Fatal(err)
			}
			plain, err := migrate.Migrate(name+".xmi", data)
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			opts, err := migrate.MigrateOptions(name+".xmi", data, migrate.Options{})
			if err != nil {
				t.Fatalf("MigrateOptions: %v", err)
			}
			if !bytes.Equal(plain.Notation, opts.Notation) {
				t.Error("notation differs")
			}
			var a, b bytes.Buffer
			if err := plain.Report.WriteText(&a); err != nil {
				t.Fatal(err)
			}
			if err := opts.Report.WriteText(&b); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a.Bytes(), b.Bytes()) {
				t.Error("text report differs")
			}
		})
	}
}

// A layout export whose diagram records match no diagram of the model was
// exported from a different project, and is an error rather than a report.
func TestLayoutProjectMismatch(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/layout.xmi")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := mtip.Parse([]byte(`<packet><metadata><mtipVersion>2022x</mtipVersion></metadata><data><id _dtype="dict"><cameo _dtype="str">_other</cameo></id><relationships _dtype="dict"><element _dtype="list"/></relationships><type _dtype="str">sysml.BlockDefinitionDiagram</type></data></packet>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = migrate.MigrateOptions("layout.xmi", data, migrate.Options{Layout: foreign, LayoutSource: "foreign.xml"})
	if err == nil {
		t.Fatal("expected the project mismatch error")
	}
	want := "foreign.xml: none of its 1 diagram records matches a diagram of layout.xmi; the layout file was exported from a different project"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

// The layout summary accounts for every diagram record and every shown element.
func TestLayoutSummary(t *testing.T) {
	r := migrateLayoutFixture(t)
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Diagrams != 3 || l.DiagramsJoined != 2 || l.DiagramsUnmatched != 1 {
		t.Errorf("diagrams: %+v", l)
	}
	if l.Placements != 7 || l.PlacementsWritten != 5 || l.PlacementsUnexposed != 1 || l.PlacementsDangling != 1 {
		t.Errorf("placements: %+v", l)
	}
	if l.Routes != 4 || l.RoutesWritten != 1 || l.RoutesUnexposed != 2 || l.RoutesDangling != 1 {
		t.Errorf("routes: %+v", l)
	}
	// The tree diagram draws no connection, so its connector's route is not
	// pinned; the control flow the interconnection diagram shows is not either.
	wantKinds := []migrate.RouteKind{
		{Kind: "Connector", Reason: "not exposed", Count: 1},
		{Kind: "Connector", Reason: "written", Count: 1},
		{Kind: "ControlFlow", Reason: "not drawn", Count: 1},
		{Kind: "Dependency", Reason: "dangling", Count: 1},
	}
	if !reflect.DeepEqual(l.RoutesByKind, wantKinds) {
		t.Errorf("routes by kind: %+v, want %+v", l.RoutesByKind, wantKinds)
	}
	if l.Malformed != 1 || l.Unsupported["fillColor"] != 1 {
		t.Errorf("malformed/unsupported: %+v", l)
	}
	if !strings.Contains(r.Report.Summary(), "laid out 2 of 2 diagrams from layout.layout.xml") {
		t.Errorf("summary: %s", r.Report.Summary())
	}
	var unmapped int
	for _, e := range r.Report.Entries {
		if e.Kind == "Layout" && e.Verdict == migrate.Unmapped {
			unmapped++
		}
	}
	if unmapped != 2 {
		t.Errorf("layout entries: %d, want one per unmatched record and one per malformed record", unmapped)
	}
}

// A record joining a diagram the migration does not write as a view — its host
// is written without a body — lays out nothing, and the summary counts it as
// unmatched rather than joined.
func TestLayoutDiagramWithoutWrittenView(t *testing.T) {
	xmi := `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="2.5.1" xmlns:xmi="http://www.omg.org/spec/XMI/20131001"
         xmlns:uml="http://www.omg.org/spec/UML/20161101"
         xmlns:sysml="http://www.omg.org/spec/SysML/20181001/SysML"
         xmlns:diagram="http://www.example.com/tool/diagram">
  <uml:Model xmi:type="uml:Model" xmi:id="_m" name="Model">
    <packagedElement xmi:type="uml:Package" xmi:id="_pkg" name="P">
      <packagedElement xmi:type="uml:Class" xmi:id="_blk_wheel" name="Wheel"/>
      <packagedElement xmi:type="uml:Class" xmi:id="_blk_vehicle" name="Vehicle">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_prop_w" name="wheel" type="_blk_wheel"/>
        <ownedConnector xmi:type="uml:Connector" xmi:id="_conn" name="drive">
          <end xmi:type="uml:ConnectorEnd" xmi:id="_ce_1" role="_prop_w"/>
          <end xmi:type="uml:ConnectorEnd" xmi:id="_ce_2" role="_prop_w"/>
        </ownedConnector>
      </packagedElement>
    </packagedElement>
    <xmi:Extension extender="Example UML Tool 1.0">
      <modelExtension>
        <ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag_unhosted" name="Drive Internals" ownerOfDiagram="_conn">
          <xmi:Extension extender="Example UML Tool 1.0">
            <diagramRepresentation>
              <diagram:DiagramRepresentationObject type="SysML Internal Block Diagram" umlType="Composite Structure Diagram">
                <diagramContents><usedElements>_blk_wheel</usedElements></diagramContents>
              </diagram:DiagramRepresentationObject>
            </diagramRepresentation>
          </xmi:Extension>
        </ownedDiagram>
      </modelExtension>
    </xmi:Extension>
  </uml:Model>
  <sysml:Block xmi:id="_s_wheel2" base_Class="_blk_wheel"/>
  <sysml:Block xmi:id="_s_wheel" base_Class="_blk_wheel"/>
</xmi:XMI>`
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID:          "_diag_unhosted",
		Name:        "Drive Internals",
		Placements:  []mtip.Placement{{ID: "_blk_wheel", X: 10, Y: 20, Width: 100, Height: 40}},
		Unsupported: map[string]int{},
	}}}
	r, err := migrate.MigrateOptions("unhosted.xmi", []byte(xmi), migrate.Options{Layout: layout, LayoutSource: "unhosted.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Diagrams != 1 || l.DiagramsJoined != 0 || l.DiagramsUnmatched != 1 {
		t.Errorf("diagrams: %+v", l)
	}
	var row *migrate.Entry
	for i := range r.Report.Entries {
		e := &r.Report.Entries[i]
		if e.Kind == "Layout" {
			row = e
		}
	}
	if row == nil || row.ID != "_diag_unhosted" || row.Note != "matches a diagram the migration does not write as a view" {
		t.Errorf("layout row: %+v", row)
	}
	if strings.Contains(string(r.Notation), "DiagramLayout::") {
		t.Errorf("notation lays out a view never written:\n%s", r.Notation)
	}
}

// A table diagram with a layout record keeps its Canvas and Layout annotations
// on the view usage, next to the Document its table definition becomes: the
// view exposes the Document, the query still reads the laid-out rows, and the
// join counts the diagram as laid out.
func TestLayoutOfTableDiagram(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/tables.xmi")
	if err != nil {
		t.Fatal(err)
	}
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID:   "_diag_pumps",
		Name: "Pump Table",
		Type: "sysml.InstanceTable",
		Placements: []mtip.Placement{
			{ID: "_inst_p1", X: 0, Y: 0, Width: 400, Height: 20},
			{ID: "_inst_p2", X: 0, Y: 20, Width: 400, Height: 20},
			{ID: "_inst_r1", X: 0, Y: 40, Width: 400, Height: 20},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := migrate.MigrateOptions("tables.xmi", data, migrate.Options{Layout: layout, LayoutSource: "tables.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	wantClean(t, "tables.sysml", r)
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Diagrams != 1 || l.DiagramsJoined != 1 || l.DiagramsUnmatched != 0 || l.PlacementsWritten != 3 {
		t.Errorf("layout summary: %+v", l)
	}
	notation := string(r.Notation)
	wantInOrder(t, "laid-out table view", notation,
		"view 'Pump Table' {",
		"expose p1;", "expose p2;", "expose r1;",
		"expose 'Pump Table Document';",
		`@DiagramLayout::Canvas { unit = "px"; width = 400; height = 60; }`,
		"metadata DiagramLayout::Layout about p1 { x = 0; y = 0; width = 400; height = 20; }",
		"metadata DiagramLayout::Layout about p2 { x = 0; y = 20; width = 400; height = 20; }",
		"metadata DiagramLayout::Layout about r1 { x = 0; y = 40; width = 400; height = 20; }",
		"render Views::asElementTable;",
		"calc def 'Pump Table Rows' :> DocumentQueries::Query {",
		"part def 'Pump Table Document' :> DocumentQueries::Document {")

	plain, err := migrate.Migrate("tables.xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	strip := func(s string) string {
		var kept []string
		for _, line := range strings.Split(s, "\n") {
			if !strings.Contains(line, "DiagramLayout::") {
				kept = append(kept, line)
			}
		}
		return strings.Join(kept, "\n")
	}
	if strip(notation) != string(plain.Notation) {
		t.Errorf("the layout changed more than the annotations:\n%s", notation)
	}
	s := session(t, r)
	wantInOrder(t, "laid-out Pump Table rows", rows(t, s, "Plant::Inventory::'Pump Table Rows'"),
		"returned 4 rows", "Plant::Inventory::r1", "Plant::Inventory::p1", "Plant::Spares::s1", "Plant::Spares::s2")
}

// edgeText is an edge's texts as the forms without a place at an end fold
// them: the tail end's name, the edge's own, the head end's name.
func edgeText(edge view.Edge) string {
	var texts []string
	for _, text := range []string{edge.FromLabel, edge.Label, edge.ToLabel} {
		if text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, " ")
}

// An association end's role symbol — the 10×10 box Cameo keeps at the line's end,
// inside the association's symbol — is the end's position on the line, which the
// association's route carries, pinned to the member-end usage the tree rendering
// draws as the composition edge: no Layout is written for the end property, so the
// rendering lists the part in its definition unplaced and routes the edge, rather
// than drawing a 10×10 node. A property drawn as its own box keeps its Layout.
func TestGoldenAssociationEndLayoutLiesOnTheEdge(t *testing.T) {
	r := migrateStream(t, "association_end_layout")
	l := r.Report.Layout
	if l == nil || l.StreamDiagrams != 1 || l.Placements != 6 || l.PlacementsWritten != 3 || l.PlacementsOnEdges != 3 || l.PlacementsUnexposed != 0 || l.RoutesWritten != 1 {
		t.Fatalf("layout summary: %+v", l)
	}
	checkGolden(t, "testdata/xmi/association_end_layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/association_end_layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "association_end_layout.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	for _, want := range []string{
		"metadata DiagramLayout::Layout about Vehicle { x = 40; y = 20; width = 120; height = 50; }",
		"metadata DiagramLayout::Layout about Engine { x = 40; y = 160; width = 120; height = 90; }",
		"metadata DiagramLayout::Layout about Blocks::Engine::piston { x = 55; y = 200; width = 90; height = 30; }",
		"metadata DiagramLayout::Route about EngineToVehicle { points = (100, 160, 100, 70); }",
		"3 of 6 shown elements positioned (3 lying on connections)",
	} {
		if !strings.Contains(notation, want) && !strings.Contains(report.String(), want) {
			t.Errorf("output does not contain %q:\n%s\n%s", want, notation, report.String())
		}
	}
	if strings.Contains(notation, "width = 10;") {
		t.Errorf("a Layout the size of an end symbol was written:\n%s", notation)
	}
	s := session(t, r)
	rendering, err := s.ViewRendering("Blocks::'Vehicle BDD'")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
	var drawn []string
	names := map[string]string{}
	var visit func([]*view.Node)
	visit = func(list []*view.Node) {
		for _, node := range list {
			names[node.ID] = node.Name
			g := ""
			if node.Geometry != nil {
				g = fmt.Sprintf(" at (%v, %v) size %vx%v", node.Geometry.X, node.Geometry.Y, node.Geometry.Width, node.Geometry.Height)
			}
			drawn = append(drawn, node.Kind+" "+node.Name+g)
			visit(node.Children)
		}
	}
	visit(rendering.Roots)
	wantDrawn := []string{
		"part def Model::Blocks::Vehicle at (40, 20) size 120x50",
		"part engine",
		"part piston at (55, 200) size 90x30",
		"part def Model::Blocks::Engine at (40, 160) size 120x90",
		"part piston at (55, 200) size 90x30",
	}
	if !slices.Equal(drawn, wantDrawn) {
		t.Errorf("nodes drawn = %q, want %q", drawn, wantDrawn)
	}
	var edges []string
	for _, edge := range rendering.Edges {
		edges = append(edges, fmt.Sprintf("%s %s %s: %s", names[edge.From], edge.Kind, names[edge.To], edgeText(edge)))
	}
	wantEdges := []string{
		"Model::Blocks::Vehicle composition Model::Blocks::Engine: engine",
		"Model::Blocks::Vehicle composition engine: ",
		"engine composition piston: ",
		"Model::Blocks::Engine composition piston: ",
	}
	if !slices.Equal(edges, wantEdges) {
		t.Errorf("edges = %q, want %q", edges, wantEdges)
	}
	if want := []view.Point{{X: 100, Y: 70}, {X: 100, Y: 160}}; !slices.Equal(rendering.Edges[0].Route, want) {
		t.Errorf("composition route = %v, want %v, from Vehicle's box to Engine's", rendering.Edges[0].Route, want)
	}
}

// An anonymous association whose two ends are both classifier-owned is one
// connection def drawn as a line between its end types: each diagram's line
// pins it once, with the points run from the def's first end's type to its
// second's however the stream drew the line, so the composition edge the tree
// draws from the owning block follows the line from that block's box, and
// the end symbols lie on the line rather than becoming 10x10 nodes.
func TestGoldenAssociationEndPairPinsItsConnectionDef(t *testing.T) {
	r := migrateStream(t, "association_end_pair")
	l := r.Report.Layout
	if l == nil || l.StreamDiagrams != 2 || l.Placements != 7 || l.PlacementsWritten != 4 || l.PlacementsOnEdges != 3 || l.Routes != 2 || l.RoutesWritten != 2 || l.RoutesUnexposed != 0 {
		t.Fatalf("layout summary: %+v", l)
	}
	checkGolden(t, "testdata/xmi/association_end_pair.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/association_end_pair.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "association_end_pair.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	wantInOrder(t, "pair view", notation,
		"view 'Pair BDD' {",
		"metadata DiagramLayout::Route about SensorToController { points = (100, 200, 100, 130, 100, 70); }",
		"view 'Sensor BDD' {",
		"metadata DiagramLayout::Route about SensorToController { points = (160, 225, 300, 225, 300, 70); }",
	)
	if strings.Contains(notation, "width = 10;") {
		t.Errorf("a Layout the size of an end symbol was written:\n%s", notation)
	}
	s := session(t, r)
	for name, want := range map[string]struct {
		edges []string
		route []view.Point
	}{
		"Blocks::'Pair BDD'": {
			edges: []string{
				"Model::Blocks::Controller composition Model::Blocks::Sensor: controller sensors",
				"Model::Blocks::Controller composition sensors: ",
				"sensors reference controller: ",
				"Model::Blocks::Sensor reference controller: ",
				"controller composition sensors: ",
			},
			route: []view.Point{{X: 100, Y: 70}, {X: 100, Y: 130}, {X: 100, Y: 200}},
		},
		"Blocks::'Sensor BDD'": {
			edges: []string{
				"Model::Blocks::Controller composition Model::Blocks::Sensor: controller sensors",
				"Model::Blocks::Controller composition sensors: ",
				"sensors reference controller: ",
				"Model::Blocks::Sensor reference controller: ",
				"controller composition sensors: ",
			},
			route: []view.Point{{X: 300, Y: 70}, {X: 300, Y: 225}, {X: 160, Y: 225}},
		},
	} {
		rendering, err := s.ViewRendering(name)
		if err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		if len(rendering.Notices) != 0 {
			t.Errorf("%s: notices = %v, want none", name, rendering.Notices)
		}
		names := map[string]string{}
		var visit func([]*view.Node)
		visit = func(list []*view.Node) {
			for _, node := range list {
				names[node.ID] = node.Name
				if node.Geometry != nil && node.Geometry.Width == 10 && node.Geometry.Height == 10 {
					t.Errorf("%s: %s %s drawn as a 10x10 node", name, node.Kind, node.Name)
				}
				visit(node.Children)
			}
		}
		visit(rendering.Roots)
		var edges []string
		for _, edge := range rendering.Edges {
			edges = append(edges, fmt.Sprintf("%s %s %s: %s", names[edge.From], edge.Kind, names[edge.To], edgeText(edge)))
		}
		if !slices.Equal(edges, want.edges) {
			t.Errorf("%s: edges = %q, want %q", name, edges, want.edges)
		}
		if !slices.Equal(rendering.Edges[0].Route, want.route) {
			t.Errorf("%s: composition route = %v, want %v, from Controller's box to Sensor's", name, rendering.Edges[0].Route, want.route)
		}
	}
}

// An export that places an end symbol the stream shows on the association's
// path keeps the end on the line: the export's bounds win, the stream's nesting holds.
func TestExportPlacementOfAnEndSymbolStaysOnTheEdge(t *testing.T) {
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID:   "_diag_bdd",
		Name: "Vehicle BDD",
		Type: "sysml.BlockDefinitionDiagram",
		Placements: []mtip.Placement{
			{ID: "_blk_vehicle", X: 40, Y: 20, Width: 120, Height: 50},
			{ID: "_prop_engine", X: 95, Y: 150, Width: 10, Height: 10},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := migrate.MigrateOptions("association_end_layout.xmi", streamArchive(t, "association_end_layout"), migrate.Options{Layout: layout, LayoutSource: "association_end_layout.xml"})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	l := r.Report.Layout
	if l == nil || l.DiagramsJoined != 1 || l.Placements != 5 || l.PlacementsWritten != 3 || l.PlacementsOnEdges != 2 || l.RoutesWritten != 1 {
		t.Fatalf("layout summary: %+v", l)
	}
	notation := string(r.Notation)
	wantInOrder(t, "joined view", notation,
		"metadata DiagramLayout::Layout about Vehicle { x = 40; y = 20; width = 120; height = 50; }",
		"metadata DiagramLayout::Layout about Blocks::Engine::piston { x = 55; y = 200; width = 90; height = 30; }",
		"metadata DiagramLayout::Route about EngineToVehicle { points = (100, 160, 100, 70); }",
	)
	if strings.Contains(notation, "width = 10;") {
		t.Errorf("a Layout the size of an end symbol was written:\n%s", notation)
	}
}

// An export's route of an association wins over the stream's and pins the
// connection def once; which element is at which end of the exported line is
// read from the stream, so the points run from the def's first end's type to
// its second's whichever way the export lists them.
func TestExportRouteOfAnAssociationPinsItsConnectionDef(t *testing.T) {
	above := []mtip.Placement{
		{ID: "_blk_controller", Type: "Class", X: 50, Y: 20, Width: 120, Height: 50},
		{ID: "_blk_sensor", Type: "Class", X: 50, Y: 200, Width: 120, Height: 50},
	}
	// The export has moved the sensor above the controller, across the stream's ends.
	// Without boxes of its own, the stream's, which the view draws, place the ends.
	below := []mtip.Placement{
		{ID: "_blk_controller", Type: "Class", X: 50, Y: 250, Width: 120, Height: 50},
		{ID: "_blk_sensor", Type: "Class", X: 50, Y: 20, Width: 120, Height: 50},
	}
	for name, c := range map[string]struct {
		boxes  []mtip.Placement
		points []float64
		want   string
	}{
		"drawn the stream's way":     {above, []float64{110, 70, 110, 200}, "(110, 200, 110, 70)"},
		"drawn against the stream's": {above, []float64{110, 200, 110, 70}, "(110, 200, 110, 70)"},
		"drawn between moved boxes":  {below, []float64{110, 250, 110, 70}, "(110, 70, 110, 250)"},
		"drawn without boxes":        {nil, []float64{110, 70, 110, 200}, "(110, 200, 110, 70)"},
	} {
		t.Run(name, func(t *testing.T) {
			layout := &mtip.Export{Diagrams: []mtip.Diagram{{
				ID:         "_diag_pair",
				Name:       "Pair BDD",
				Type:       "sysml.BlockDefinitionDiagram",
				Placements: c.boxes,
				Connectors: []mtip.Connector{
					{ID: "_assoc_pair", Type: "Association", Points: c.points},
				},
				Unsupported: map[string]int{},
			}}}
			r, err := migrate.MigrateOptions("association_end_pair.xmi", streamArchive(t, "association_end_pair"), migrate.Options{Layout: layout, LayoutSource: "association_end_pair.xml"})
			if err != nil {
				t.Fatalf("MigrateOptions: %v", err)
			}
			l := r.Report.Layout
			if l == nil || l.DiagramsJoined != 1 || l.Routes != 2 || l.RoutesWritten != 2 {
				t.Fatalf("layout summary: %+v", l)
			}
			wantInOrder(t, "joined pair view", string(r.Notation),
				"view 'Pair BDD' {",
				"metadata DiagramLayout::Route about SensorToController { points = "+c.want+"; }",
			)
		})
	}
}

// A view exposes a definition and a same-named usage, and each expose imports
// its name into the view: inside it the simple name denotes neither, so the
// writer qualifies both, the def keeps its own position and the association
// to it is drawn as the composition edge the tree renders.
func TestGoldenLayoutQualifiesNamesTheExposesHide(t *testing.T) {
	r := migrateLaidOut(t, "layout_same_named_usage")
	if l := r.Report.Layout; l == nil || l.DiagramsJoined != 1 || l.PlacementsWritten != 5 {
		t.Fatalf("layout summary: %+v", l)
	}
	checkGolden(t, "testdata/xmi/layout_same_named_usage.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/layout_same_named_usage.layout.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "layout_same_named_usage.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	notation := string(r.Notation)
	for _, want := range []string{
		"expose UI::GUI;",
		"expose UI::Console::GUI;",
		"expose Panel;",
		"metadata DiagramLayout::Layout about UI::GUI { x = 20; y = 200;",
		"metadata DiagramLayout::Layout about UI::Console::GUI { x = 110; y = 90;",
		"metadata DiagramLayout::Layout about Panel { x = 250; y = 200;",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("layout output does not contain %q:\n%s", want, notation)
		}
	}
	s := session(t, r)
	rendering, err := s.ViewRendering("UI::'Console BDD'")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none", rendering.Notices)
	}
	nodes, names := map[string]*view.Node{}, map[string]string{}
	var visit func([]*view.Node)
	visit = func(list []*view.Node) {
		for _, node := range list {
			nodes[node.Name], names[node.ID] = node, node.Name
			visit(node.Children)
		}
	}
	visit(rendering.Roots)
	gui := nodes["Model::UI::GUI"]
	if gui == nil || gui.Geometry == nil || *gui.Geometry != (view.Geometry{X: 20, Y: 200, Width: 80, Height: 37, HasSize: true}) {
		t.Fatalf("GUI def node = %+v, want one at its own position; drawn: %v", gui, slices.Sorted(maps.Keys(nodes)))
	}
	var edges []string
	for _, edge := range rendering.Edges {
		edges = append(edges, fmt.Sprintf("%s %s %s: %s", names[edge.From], edge.Kind, names[edge.To], edgeText(edge)))
	}
	want := []string{
		"Model::UI::Console composition Model::UI::GUI: GUI",
		"Model::UI::Console composition Model::UI::Panel: panel",
		"Model::UI::Console composition GUI: ",
		"Model::UI::Console composition panel: ",
	}
	if !reflect.DeepEqual(edges, want) {
		t.Errorf("edges = %q, want %q", edges, want)
	}
}

func TestGoldenPrivateLayoutAlias(t *testing.T) {
	r := migrateLaidOut(t, "private_layout_alias")
	if l := r.Report.Layout; l == nil || l.DiagramsJoined != 1 || l.PlacementsWritten != 2 || l.PlacementsUnexposed != 0 {
		t.Fatalf("layout summary: %+v", l)
	}
	if diagnostics := errors(t, "private_layout_alias.sysml", r.Notation); len(diagnostics) > 0 {
		for _, d := range diagnostics {
			t.Error(d)
		}
		t.FailNow()
	}
	assertPrivateLayoutAliasTarget(t, "private_layout_alias.sysml", r.Notation, []string{"Model", "A"}, "A BDD", "x (A)", "Model::A::x", "Model::B::x")
	checkGolden(t, "testdata/xmi/private_layout_alias.layout.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/private_layout_alias.layout.golden.report.txt", report.Bytes())
	notation := string(r.Notation)
	for _, want := range []string{
		"private alias 'x (A)' for x;",
		"metadata MigrationMetadata::SynthesizedName about 'x (A)';",
		"metadata DiagramLayout::Layout about 'x (A)'",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("layout output does not contain %q:\n%s", want, notation)
		}
	}
}

func TestPrivateLayoutAliasMarkerResolvesToLibraryWithShadowedName(t *testing.T) {
	const docName = "private_layout_alias_scope.sysml"
	r := migratePrivateLayoutAliasWithMetadataMember(t)
	notation := string(r.Notation)
	if !strings.Contains(notation, "metadata $::MigrationMetadata::SynthesizedName about 'x (A)';") {
		t.Errorf("metadata marker is not rooted past the local name:\n%s", notation)
	}
	for _, d := range errors(t, docName, r.Notation) {
		t.Errorf("%v", d)
	}
	assertPrivateLayoutAliasTarget(t, docName, r.Notation, []string{"Model", "A"}, "A BDD", "x (A)", "Model::A::x", "Model::B::x")

	ws := model.NewWorkspace()
	ws.Open(docName, r.Notation, 1)
	doc := ws.Document(docName)
	if doc == nil {
		t.Fatalf("workspace did not open %s", docName)
	}
	owner, ok := ws.ResolveQualifiedInDoc(docName, doc.Scope, ast.QualifiedNameOf("Model", "A"))
	if !ok || owner.Scope == nil {
		t.Fatal("owner Model::A is unresolved or has no scope")
	}
	if _, ok := owner.Scope.LookupLocal("MigrationMetadata"); !ok {
		t.Fatal("test model has no owner-local MigrationMetadata")
	}
	var markerFacts []*symbols.AnnotationFacts
	var libraryType *symbols.Symbol
	var libraryTypeOK bool
	if err := ws.Read(func(reading *model.Reading) error {
		reading.Query(docName, func(resolver *resolve.Resolver, semantic *semantics.Model) {
			for _, member := range owner.Scope.AllMembers() {
				if facts := semantic.AboutAnnotationFactsOf(member); facts != nil {
					markerFacts = append(markerFacts, facts)
				}
			}
			libraryType, libraryTypeOK = resolver.ResolveQualified(doc.Scope, ast.QualifiedNameOf("MigrationMetadata", "SynthesizedName"))
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !libraryTypeOK || libraryType.Kind != symbols.SymbolMetadataDef {
		t.Fatalf("library MigrationMetadata::SynthesizedName = %v, resolved = %t", libraryType, libraryTypeOK)
	}
	libraryFQN := symbols.FQNOf(libraryType)
	for _, facts := range markerFacts {
		if facts.TypeFQN == libraryFQN {
			return
		}
	}
	t.Fatalf("about marker facts = %+v, want library type %s", markerFacts, libraryFQN)
}

func migratePrivateLayoutAliasWithMetadataMember(t *testing.T) *migrate.Result {
	t.Helper()
	const sourcePath = "testdata/xmi/private_layout_alias.xmi"
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	const member = `      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_b" name="b" type="_b" aggregation="composite"/>`
	const replacement = `      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_migration_metadata" name="MigrationMetadata" type="_b" aggregation="composite"/>
` + member
	if !bytes.Contains(sourceData, []byte(member)) {
		t.Fatalf("%s does not contain the expected owner member", sourcePath)
	}
	sourceData = bytes.Replace(sourceData, []byte(member), []byte(replacement), 1)
	layoutData, err := os.ReadFile("testdata/xmi/private_layout_alias.layout.xml")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := mtip.Parse(layoutData)
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	r, err := migrate.MigrateOptions("private_layout_alias_scope.xmi", sourceData, migrate.Options{
		Layout: layout, LayoutSource: "private_layout_alias.layout.xml",
	})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	return r
}

// A private layout alias never takes a name the owner's body already refers
// to unqualified: part c keeps its top-level type 'x (A)', and the alias for
// the shadowed private part takes the numbered name.
func TestPrivateLayoutAliasKeepsReferencedNameResolving(t *testing.T) {
	const docName = "private_layout_alias_referenced.sysml"
	data := document(`
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_x" name="x" visibility="private" type="_b" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_c" name="c" type="_taken" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b_x" name="x"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_taken" name="x (A)"/>
    <xmi:Extension extender="Example UML Tool 1.0">
      <ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag" name="A BDD" ownerOfDiagram="_a">
        <xmi:Extension extender="Example UML Tool 1.0">
          <diagramRepresentation>
            <diagram:DiagramRepresentationObject xmlns:diagram="http://www.example.com/tool/diagram" type="SysML Block Definition Diagram" umlType="Class Diagram">
              <diagramContents>
                <usedElements>_a_x</usedElements>
                <usedElements>_b_x</usedElements>
              </diagramContents>
            </diagram:DiagramRepresentationObject>
          </diagramRepresentation>
        </xmi:Extension>
      </ownedDiagram>
    </xmi:Extension>`, `
  <sysml:Block xmi:id="_s_a" base_Class="_a"/>
  <sysml:Block xmi:id="_s_b" base_Class="_b"/>
  <sysml:Block xmi:id="_s_taken" base_Class="_taken"/>`)
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID: "_diag",
		Placements: []mtip.Placement{
			{ID: "_a_x", X: 10, Y: 10, Width: 80, Height: 40},
			{ID: "_b_x", X: 110, Y: 10, Width: 80, Height: 40},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := migrate.MigrateOptions("private_layout_alias_referenced.xmi", data, migrate.Options{
		Layout: layout, LayoutSource: "private_layout_alias_referenced.layout.xml",
	})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	if l := r.Report.Layout; l == nil || l.PlacementsWritten != 2 || l.PlacementsUnexposed != 0 {
		t.Fatalf("layout summary: %+v", l)
	}
	for _, d := range errors(t, docName, r.Notation) {
		t.Errorf("%v", d)
	}
	wantLine(t, r.Notation, "part c : 'x (A)';")
	wantLine(t, r.Notation, "private alias 'x (A) 2' for x;")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about 'x (A) 2' {")
	assertPrivateLayoutAliasTarget(t, docName, r.Notation, []string{"Model", "A"}, "A BDD", "x (A) 2", "Model::A::x", "Model::B::x")

	ws := model.NewWorkspace()
	ws.Open(docName, r.Notation, 1)
	doc := ws.Document(docName)
	if doc == nil {
		t.Fatalf("workspace did not open %s", docName)
	}
	part, ok := ws.ResolveQualifiedInDoc(docName, doc.Scope, ast.QualifiedNameOf("Model", "A", "c"))
	if !ok {
		t.Fatal("A::c is unresolved")
	}
	var types []string
	if err := ws.Read(func(reading *model.Reading) error {
		reading.Query(docName, func(_ *resolve.Resolver, semantic *semantics.Model) {
			for _, typ := range semantic.FeatureTypes(part) {
				types = append(types, symbols.FQNOf(typ))
			}
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(types, []string{"Model::x (A)"}) {
		t.Fatalf("A::c is typed by %v, want the top-level block x (A)", types)
	}
}

func TestPrivateLayoutAliasKeepsNestedTypeReferenceResolving(t *testing.T) {
	const docName = "private_layout_alias_nested_type.sysml"
	data := document(`
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_x" name="x" visibility="private" type="_b" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a_c" name="c" type="_thing" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b_x" name="x"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_taken" name="x (A)">
      <nestedClassifier xmi:type="uml:Class" xmi:id="_thing" name="Thing"/>
    </packagedElement>
    <xmi:Extension extender="Example UML Tool 1.0">
      <ownedDiagram xmi:type="uml:Diagram" xmi:id="_diag" name="A BDD" ownerOfDiagram="_a">
        <xmi:Extension extender="Example UML Tool 1.0">
          <diagramRepresentation>
            <diagram:DiagramRepresentationObject xmlns:diagram="http://www.example.com/tool/diagram" type="SysML Block Definition Diagram" umlType="Class Diagram">
              <diagramContents>
                <usedElements>_a_x</usedElements>
                <usedElements>_b_x</usedElements>
              </diagramContents>
            </diagram:DiagramRepresentationObject>
          </diagramRepresentation>
        </xmi:Extension>
      </ownedDiagram>
    </xmi:Extension>`, `
  <sysml:Block xmi:id="_s_a" base_Class="_a"/>
  <sysml:Block xmi:id="_s_b" base_Class="_b"/>
  <sysml:Block xmi:id="_s_taken" base_Class="_taken"/>
  <sysml:Block xmi:id="_s_thing" base_Class="_thing"/>`)
	layout := &mtip.Export{Diagrams: []mtip.Diagram{{
		ID: "_diag",
		Placements: []mtip.Placement{
			{ID: "_a_x", X: 10, Y: 10, Width: 80, Height: 40},
			{ID: "_b_x", X: 110, Y: 10, Width: 80, Height: 40},
		},
		Unsupported: map[string]int{},
	}}}
	r, err := migrate.MigrateOptions("private_layout_alias_nested_type.xmi", data, migrate.Options{
		Layout: layout, LayoutSource: "private_layout_alias_nested_type.layout.xml",
	})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	for _, d := range errors(t, docName, r.Notation) {
		t.Errorf("%v", d)
	}
	wantLine(t, r.Notation, "part c : 'x (A)'::Thing;")
	wantLine(t, r.Notation, "private alias 'x (A) 2' for x;")

	ws := model.NewWorkspace()
	ws.Open(docName, r.Notation, 1)
	doc := ws.Document(docName)
	if doc == nil {
		t.Fatalf("workspace did not open %s", docName)
	}
	part, ok := ws.ResolveQualifiedInDoc(docName, doc.Scope, ast.QualifiedNameOf("Model", "A", "c"))
	if !ok {
		t.Fatal("A::c is unresolved")
	}
	var types []string
	if err := ws.Read(func(reading *model.Reading) error {
		reading.Query(docName, func(_ *resolve.Resolver, semantic *semantics.Model) {
			for _, typ := range semantic.FeatureTypes(part) {
				types = append(types, symbols.FQNOf(typ))
			}
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(types, []string{"Model::x (A)::Thing"}) {
		t.Fatalf("A::c is typed by %v, want the nested block x (A)::Thing", types)
	}
}

func TestStrictPrivateLayoutAliasIsOmitted(t *testing.T) {
	r := migrateLaidOutOptions(t, "private_layout_alias", migrate.Options{Strict: true})
	if strings.Contains(string(r.Notation), "private alias ") {
		t.Fatalf("strict migration wrote a private layout alias:\n%s", r.Notation)
	}
}

func assertPrivateLayoutAliasTarget(t *testing.T, docName string, notation []byte, ownerPath []string, viewName, aliasName, targetFQN, wrongFQN string) {
	t.Helper()
	ws := model.NewWorkspace()
	ws.Open(docName, notation, 1)
	doc := ws.Document(docName)
	if doc == nil {
		t.Fatalf("workspace did not open %s", docName)
	}
	owner, ok := ws.ResolveQualifiedInDoc(docName, doc.Scope, ast.QualifiedNameOf(ownerPath...))
	if !ok || owner.Scope == nil {
		t.Fatalf("owner %v is unresolved or has no scope", ownerPath)
	}
	viewSym, ok := ws.ResolveQualifiedInDoc(docName, owner.Scope, ast.QualifiedNameOf(viewName))
	if !ok || viewSym.Scope == nil {
		t.Fatalf("view %q is unresolved or has no scope", viewName)
	}
	alias, ok := owner.Scope.LookupLocal(aliasName)
	if !ok || alias.Kind != symbols.SymbolAlias || alias.Visibility != ast.VisibilityPrivate {
		t.Fatalf("owner alias %q = %v, want private alias", aliasName, alias)
	}
	var aboutTarget, aliasTarget *symbols.Symbol
	var aboutOK, aliasOK bool
	if err := ws.Read(func(r *model.Reading) error {
		r.Query(docName, func(resolver *resolve.Resolver, _ *semantics.Model) {
			aboutTarget, aboutOK = resolver.ResolveQualified(viewSym.Scope, ast.QualifiedNameOf(aliasName))
			aliasTarget, aliasOK = resolver.ResolveAliasTarget(alias)
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !aboutOK || !aliasOK {
		t.Fatalf("about target resolved = %t, alias target resolved = %t", aboutOK, aliasOK)
	}
	if aboutTarget != aliasTarget {
		t.Fatalf("about target %s is not alias target %s", symbols.FQNOf(aboutTarget), symbols.FQNOf(aliasTarget))
	}
	if got := symbols.FQNOf(aliasTarget); got != targetFQN {
		t.Fatalf("alias %q resolves to %s, want %s", aliasName, got, targetFQN)
	}
	pos := doc.Lines().PosAt(aliasTarget.DeclSpan.Offset)
	t.Logf("alias %q resolves to %s at %s:%d", aliasName, symbols.FQNOf(aliasTarget), docName, pos.Line)
	if aliasTarget.Visibility != ast.VisibilityPrivate {
		t.Fatalf("alias %q target visibility = %v, want private", aliasName, aliasTarget.Visibility)
	}
	if wrong, ok := ws.ResolveQualifiedInDoc(docName, doc.Scope, ast.QualifiedNameOf(strings.Split(wrongFQN, "::")...)); ok && aboutTarget == wrong {
		t.Fatalf("alias %q resolves to %s, not a distinct target from %s", aliasName, wrongFQN, targetFQN)
	}
}

// A connector's line is pinned with its points run from the feature its first
// written end names to its second's however the stream drew it: a binding
// Cameo drew from the second end's property is written reversed, so a rendering
// that reads the Route from `bind mass = limit`'s first end follows the line.
func TestGoldenBindingRouteRunsFromItsFirstEnd(t *testing.T) {
	r := migrateStream(t, "binding_route")
	checkGolden(t, "testdata/xmi/binding_route.golden.sysml", r.Notation)
	for _, d := range errors(t, "binding_route.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	wantLine(t, r.Notation, "metadata DiagramLayout::Route about 'mass = limit' { points = (120, 110, 200, 110); }")
}

// The shapes Cameo draws inside a part shape are listed under its parts, not its
// mdOwnedViews: the nested parts are placed inside the enclosing part, which keeps a
// placement of its own, and the connector between them is routed.
func TestGoldenNestedPartsPlacedInsideTheirPart(t *testing.T) {
	r := migrateStream(t, "nested_parts")
	checkGolden(t, "testdata/xmi/nested_parts.golden.sysml", r.Notation)
	for _, d := range errors(t, "nested_parts.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about engine { x = 20; y = 20; width = 440; height = 240; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about Blocks::Engine::piston { x = 60; y = 100; width = 120; height = 60; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Layout about Blocks::Engine::sensor { x = 300; y = 100; width = 120; height = 60; }")
	wantLine(t, r.Notation, "metadata DiagramLayout::Route about Blocks::Engine::'piston to sensor' { points = (180, 130, 300, 130); }")
	// Only the two connector-end shapes on the line lie on a connection; the nested parts are nodes.
	if l := r.Report.Layout; l == nil || l.PlacementsWritten != 3 || l.PlacementsOnEdges != 2 || l.RoutesWritten != 1 {
		t.Errorf("layout report = %+v", l)
	}
}

// An initial node's symbol, and a flow final's, is placed as the start or done of the
// body it is drawn in, a nested action's included; a control flow the tool drew from
// its target back to its source is routed the way its succession runs, an object flow
// drawn between the actions owning its pins too. Of a region's initial pseudostates only
// the one written as its entry places its start, a stray one donated to an orthogonal
// region as that region's; a duplicate or dangling one, drawn first, places nothing.
func TestGoldenPseudoNodeLayout(t *testing.T) {
	r := migrateStream(t, "pseudo_nodes")
	checkGolden(t, "testdata/xmi/pseudo_nodes.golden.sysml", r.Notation)
	for _, d := range errors(t, "pseudo_nodes.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	checkPseudoNodeLayout(t, r, 17, 15, 9,
		"metadata DiagramLayout::Layout about Imaging::Camera::Modes::start { x = 50; y = 50; width = 20; height = 20; }",
		"metadata DiagramLayout::Layout about Imaging::Camera::Modes::done { x = 50; y = 300; width = 20; height = 20; }",
		"metadata DiagramLayout::Layout about Imaging::Camera::Modes::Active::regions::b::start { x = 150; y = 180; width = 16; height = 16; }",
		"metadata DiagramLayout::Route about 'start then Idle' { points = (60, 70, 60, 100); }",
		"metadata DiagramLayout::Route about 'Active then done' { points = (60, 280, 60, 300); }",
	)
	for _, unwanted := range []string{"{ x = 10; y = 50;", "{ x = 110; y = 50;"} {
		if strings.Contains(string(r.Notation), unwanted) {
			t.Errorf("a duplicate or dangling initial pseudostate's symbol is placed:\n%s", r.Notation)
		}
	}
}

// The export record names no end of a line; its own boxes say which way the tool
// drew it, so a flow recorded from its target is routed from its source all the same,
// an object flow between pins by the boxes of the actions owning them.
func TestExportPseudoNodeLayout(t *testing.T) {
	r := migrateLaidOut(t, "pseudo_nodes")
	checkPseudoNodeLayout(t, r, 11, 11, 9,
		"metadata DiagramLayout::Layout about Imaging::Camera::Modes::start { x = 50; y = 50; width = 20; height = 20; }",
		"metadata DiagramLayout::Layout about Imaging::Camera::Modes::done { x = 50; y = 190; width = 20; height = 20; }",
		"metadata DiagramLayout::Route about 'start then Idle' { points = (60, 70, 60, 100); }",
	)
}

func checkPseudoNodeLayout(t *testing.T, r *migrate.Result, placements, written, routes int, wants ...string) {
	t.Helper()
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.Placements != placements || l.PlacementsWritten != written || l.Routes != routes || l.RoutesWritten != routes {
		t.Errorf("layout: %+v", l)
	}
	wants = append(wants,
		"metadata DiagramLayout::Layout about Imaging::Camera::Acquire::start { x = 85; y = 90; width = 15; height = 15; }",
		"metadata DiagramLayout::Layout about 'Position Not Correct'::start { x = 379; y = 127; width = 15; height = 15; }",
		"metadata DiagramLayout::Route about 'start to position' { points = (92, 105, 92, 117); }",
		"metadata DiagramLayout::Route about Imaging::Camera::Acquire::'Position Not Correct'::'start to adjust' { points = (386, 142, 386, 152); }",
		"metadata DiagramLayout::Route about 'position to check' { points = (120, 150, 120, 260); }",
		"metadata DiagramLayout::Route about 'position.result to check.value' { points = (40, 150, 40, 260); }",
	)
	for _, want := range wants {
		if !strings.Contains(string(r.Notation), want) {
			t.Errorf("the migrated view lacks %s\n%s", want, r.Notation)
		}
	}
	s := session(t, r)
	rendering, err := s.ViewRendering("Imaging::Camera::Acquire::Acquire")
	if err != nil {
		t.Fatal(err)
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatal(err)
	}
	// Each start is drawn at its Layout, 15 px wide, above the action its route leads to.
	for _, x := range []string{"92.5", "386.5"} {
		found := false
		for _, line := range strings.Split(dot, "\n") {
			if strings.Contains(line, "shape=circle") && strings.Contains(line, `pos="`+x+`,`) && strings.Contains(line, "width=0.20833333333333334, height=0.20833333333333334, fixedsize=true") {
				found = true
			}
		}
		if !found {
			t.Errorf("DOT of the activity view has no start placed at x = %s:\n%s", x, dot)
		}
	}
}
