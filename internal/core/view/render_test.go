package view

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var update = flag.Bool("update", false, "rewrite the golden artifacts in testdata")

// loadFixture indexes a testdata model over the standard library, the way the
// REPL and the CLI do, and returns a renderer over it.
func loadFixture(t *testing.T, file string) (*Renderer, *symbols.Index) {
	t.Helper()
	path := filepath.Join("testdata", file)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sf := source.New(file, content)
	p := parser.New(sf)
	root := p.ParseFile()
	for _, diag := range p.Diagnostics {
		t.Fatalf("%s: parse diagnostic: %v", file, diag)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(file, root)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	sem := semantics.NewModel(resolver)
	resolver.SetModel(sem)
	text := func(doc string, span source.Span) string {
		if doc != file {
			return ""
		}
		return sf.Text(span)
	}
	return NewRenderer(sem, resolver, text), idx
}

// lookup is the symbol a fixture declares under fqn.
func lookup(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	syms := idx.LookupQualified(fqn)
	if len(syms) == 0 {
		t.Fatalf("symbol %q not indexed", fqn)
	}
	return syms[0]
}

// render is the rendering of one view of a fixture, which must render.
func render(t *testing.T, file, view string) *Rendering {
	t.Helper()
	r, idx := loadFixture(t, file)
	rendering, err := r.Render(lookup(t, idx, view))
	if err != nil {
		t.Fatalf("render %s of %s: %v", view, file, err)
	}
	return rendering
}

// TestGoldenRenderings locks the exact text and Mermaid artifact of each
// supported rendering kind, rendered from a real model.
func TestGoldenRenderings(t *testing.T) {
	cases := []struct {
		name string
		file string
		view string
		kind Kind
	}{
		{"tree", "tree.sysml", "VehicleViews::vehicleView", KindTree},
		{"interconnection", "interconnection.sysml", "PlantViews::loopView", KindInterconnection},
		{"state", "state.sysml", "MachineViews::vehicleStates", KindState},
		{"state-entry", "state-entry.sysml", "MachineViews::thermostat", KindState},
		{"action", "action.sysml", "FlowViews::driveView", KindAction},
		{"filters", "filters.sysml", "FilteredViews::safetyView", KindTree},
		{"table", "table.sysml", "TableViews::partsTable", KindTable},
		{"grid-table", "table.sysml", "TableViews::fleetTable", KindTable},
		{"sequence", "sequence.sysml", "SequenceViews::pubSubView", KindSequence},
		{"sequence-vehicle", "sequence-vehicle.sysml", "VehicleSequenceViews::startVehicleView", KindSequence},
		{"sequence-notices", "sequence-notices.sysml", "NoticeViews::handshakeView", KindSequence},
		{"sequence-order", "sequence-order.sysml", "OrderingViews::relayView", KindSequence},
		{"sequence-cycle", "sequence-order.sysml", "OrderingViews::deadlockView", KindSequence},
		{"sequence-empty", "errors.sysml", "ErrorViews::emptySequenceView", KindSequence},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			if rendering.Kind != tc.kind {
				t.Errorf("kind = %q, want %q", rendering.Kind, tc.kind)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+".text.golden"), rendering.Text())
			form := tc.kind.MachineForm()
			machine, err := rendering.Write(form)
			if err != nil {
				t.Fatalf("write %s: %v", form, err)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+"."+string(form)+".golden"), machine)
		})
	}
}

// A tree rendering shows what the view exposes and each view nested in it, and
// defaults to a tree because the view states no rendering.
func TestTreeRenderingShowsNestedViewsAndDefaults(t *testing.T) {
	rendering := render(t, "tree.sysml", "VehicleViews::vehicleView")
	if rendering.Stated != "" {
		t.Errorf("Stated = %q, want empty: the view states no rendering", rendering.Stated)
	}
	if !strings.Contains(rendering.Text(), "the view states no rendering; a tree is the default") {
		t.Errorf("text does not say the tree is the default:\n%s", rendering.Text())
	}
	names := nodeNames(rendering.Roots)
	for _, want := range []string{"Vehicles::Vehicle", "engine", "wheels", "VehicleViews::vehicleView::engineSubview", "Vehicles::Engine", "cylinder"} {
		if !names[want] {
			t.Errorf("tree rendering has no node %q; nodes: %v", want, sortedKeys(names))
		}
	}
}

// An interconnection rendering draws the connections the model holds between the
// exposed features, and no containment edge.
func TestInterconnectionRenderingDrawsConnections(t *testing.T) {
	rendering := render(t, "interconnection.sysml", "PlantViews::loopView")
	var connections, flows int
	for _, edge := range rendering.Edges {
		switch edge.Kind {
		case EdgeConnection:
			connections++
		case EdgeFlow:
			flows++
		default:
			t.Errorf("interconnection rendering has an edge of kind %v", edge.Kind)
		}
	}
	if connections == 0 || flows == 0 {
		t.Errorf("edges: %d connections, %d flows; want at least one of each", connections, flows)
	}
	if !strings.Contains(rendering.Mermaid(), "flowchart LR") {
		t.Errorf("Mermaid is no left-to-right flowchart:\n%s", rendering.Mermaid())
	}
}

// A state rendering comes from the lowered state graph: nested states are nested
// nodes, and every transition is an edge that carries its trigger and guard.
func TestStateRenderingComesFromTheLoweredGraph(t *testing.T) {
	rendering := render(t, "state.sysml", "MachineViews::vehicleStates")
	names := nodeNames(rendering.Roots)
	for _, want := range []string{"off", "operating", "idle", "moving"} {
		if !names[want] {
			t.Errorf("state rendering has no state %q; nodes: %v", want, sortedKeys(names))
		}
	}
	var labeled bool
	for _, edge := range rendering.Edges {
		if edge.Kind != EdgeTransition {
			t.Errorf("state rendering has an edge of kind %v, want a transition", edge.Kind)
		}
		if strings.Contains(edge.Label, "accept") && strings.Contains(edge.Label, "[") {
			labeled = true
		}
	}
	if !labeled {
		t.Errorf("no transition carries its trigger and guard; edges: %v", rendering.Edges)
	}
	if !strings.Contains(rendering.Mermaid(), "stateDiagram-v2") {
		t.Errorf("Mermaid is no state diagram:\n%s", rendering.Mermaid())
	}
}

// A body's entry transitions are edges out of its start node, in the order their
// guards are tried in and carrying the guard as transitions carry theirs; only
// the state an unguarded first entry transition names is initial, a guarded
// alternative never is. The machine's own body, a composite state and a region
// of a parallel state each have a start of their own.
func TestStateRenderingDrawsGuardedEntryTransitions(t *testing.T) {
	rendering := render(t, "state-entry.sysml", "MachineViews::thermostat")
	byID := map[string]*Node{}
	bodies := map[string]string{} // start node ID -> name of the body it starts
	var walk func(*Node)
	walk = func(node *Node) {
		byID[node.ID] = node
		for _, child := range node.Children {
			if child.Kind == "start" {
				bodies[child.ID] = node.Name
			}
			walk(child)
		}
	}
	for _, root := range rendering.Roots {
		walk(root)
	}
	if len(bodies) != 4 {
		t.Errorf("%d start nodes, want one each for the machine, idle, lights and fans: %v", len(bodies), bodies)
	}

	states := map[string]*Node{}
	for _, node := range byID {
		if node.Kind == "state" {
			states[node.Name] = node
		}
	}
	initial := map[string]bool{"off": true}
	for _, name := range []string{"heating", "idle", "drying", "resting", "lit", "unlit", "off", "on"} {
		node := states[name]
		if node == nil {
			t.Fatalf("state rendering has no state %q", name)
		}
		if got := strings.Contains(node.Detail, "initial"); got != initial[name] {
			t.Errorf("state %s detail = %q; initial = %t, want %t", name, node.Detail, got, initial[name])
		}
	}

	var entries []string
	for _, edge := range rendering.Edges {
		body, fromStart := bodies[edge.From]
		if edge.Kind != EdgeTransition || byID[edge.To] == nil || !edge.Origin.Located() {
			t.Errorf("edge %s -> %s is no located transition between nodes: %+v", edge.From, edge.To, edge)
		}
		if fromStart {
			entries = append(entries, body+" -> "+byID[edge.To].Name+": "+edge.Label)
		}
	}
	want := []string{
		"Machines::Thermostat -> heating: [cold]",
		"Machines::Thermostat -> idle: [not cold]",
		"idle -> drying: [wet]",
		"idle -> resting: ",
		"lights -> lit: [dark]",
		"lights -> unlit: [not dark]",
		"fans -> off: ",
	}
	if strings.Join(entries, "\n") != strings.Join(want, "\n") {
		t.Errorf("entry edges:\n%s\nwant:\n%s", strings.Join(entries, "\n"), strings.Join(want, "\n"))
	}

	// Mermaid draws each body's entry transitions from its own start marker,
	// inside the body, and no second start arrow for an initial state.
	mermaid := rendering.Mermaid()
	blocks := []string{
		"    [*] --> n1 : [cold]\n    [*] --> n2 : [not cold]\n  }",
		"      [*] --> n3 : [wet]\n      [*] --> n4\n    }",
		"        [*] --> n8 : [dark]\n        [*] --> n9 : [not dark]\n      }",
		"        [*] --> n10\n      }",
	}
	for _, block := range blocks {
		if !strings.Contains(mermaid, block) {
			t.Errorf("Mermaid lacks %q:\n%s", block, mermaid)
		}
	}
	if got := strings.Count(mermaid, "[*] -->"); got != len(entries) {
		t.Errorf("Mermaid draws %d start arrows, want %d, one per entry transition:\n%s", got, len(entries), mermaid)
	}
}

// An action rendering comes from the lowered action graph, down to the flow
// within a nested action, and shows successions, guards and object flows.
func TestActionRenderingComesFromTheLoweredGraph(t *testing.T) {
	rendering := render(t, "action.sysml", "FlowViews::driveView")
	names := nodeNames(rendering.Roots)
	for _, want := range []string{"start", "provide", "monitor", "split", "sync", "check", "done", "record"} {
		if !names[want] {
			t.Errorf("action rendering has no node %q; nodes: %v", want, sortedKeys(names))
		}
	}
	var guards, flows int
	for _, edge := range rendering.Edges {
		if edge.Kind == EdgeFlow {
			flows++
		}
		if strings.HasPrefix(edge.Label, "[") {
			guards++
		}
	}
	if guards == 0 || flows == 0 {
		t.Errorf("edges: %d guarded, %d flows; want at least one of each", guards, flows)
	}
}

// A node performing statements rather than a flow of its own is rendered as the
// node it is, with nothing reported: it is no action the rendering cannot show.
func TestActionRenderingSaysNothingOfANodePerformingStatements(t *testing.T) {
	rendering := render(t, "action.sysml", "FlowViews::driveView")
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none: every node the action declares is shown", rendering.Notices)
	}
	if !nodeNames(rendering.Roots)["tally"] {
		t.Errorf("action rendering has no node %q; nodes: %v", "tally", sortedKeys(nodeNames(rendering.Roots)))
	}
}

// The rendering is of the real exposed set: a filtered recursive expose and the
// exposes inherited from the view definition, and nothing else.
func TestRenderingUsesFilteredAndInheritedExposure(t *testing.T) {
	rendering := render(t, "filters.sysml", "FilteredViews::safetyView")
	var roots []string
	for _, root := range rendering.Roots {
		roots = append(roots, root.Name)
	}
	want := []string{"Systems::Airbag", "Systems::Braking::Brake", "Systems::Radio"}
	if strings.Join(sorted(roots), " ") != strings.Join(want, " ") {
		t.Errorf("exposed set rendered = %v, want %v (filtered, inherited, and the view's own)", sorted(roots), want)
	}
}

// A tabular rendering is rows rather than nodes: one per exposed element, one per
// element declared in it, and one per nested view with its own exposed elements.
func TestTableRenderingRows(t *testing.T) {
	rendering := render(t, "table.sysml", "TableViews::fleetTable")
	if len(rendering.Roots) != 0 || len(rendering.Edges) != 0 {
		t.Errorf("a table rendering has %d nodes and %d edges, want neither", len(rendering.Roots), len(rendering.Edges))
	}
	if strings.Join(rendering.Columns, "|") != strings.Join(tableColumns, "|") {
		t.Errorf("Columns = %v, want %v", rendering.Columns, tableColumns)
	}
	owners := map[string]string{}
	for _, row := range rendering.Rows {
		if len(row) != len(tableColumns) {
			t.Fatalf("row %v has %d cells, want %d", row, len(row), len(tableColumns))
		}
		owners[row[0]] = row[3]
	}
	for name, owner := range map[string]string{
		"Fleet::Engine":                         "",
		"power":                                 "Fleet::Engine",
		"cylinder":                              "Fleet::Engine",
		"TableViews::fleetTable::partsSubtable": "TableViews::fleetTable",
		"Fleet::Truck":                          "TableViews::fleetTable::partsSubtable",
		"engine":                                "Fleet::Truck",
	} {
		got, ok := owners[name]
		if !ok {
			t.Errorf("table has no row for %q; rows: %v", name, sortedKeys(rowNames(rendering.Rows)))
			continue
		}
		if got != owner {
			t.Errorf("row %q is declared in %q, want %q", name, got, owner)
		}
	}
}

// A table is written as a Markdown table, and asking for the Mermaid a diagram is
// written as is a typed error naming the form the kind is written in.
func TestTableFormsAreMarkdownNotMermaid(t *testing.T) {
	rendering := render(t, "table.sysml", "TableViews::partsTable")
	markdown, err := rendering.Write(FormMarkdown)
	if err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	if !strings.Contains(markdown, "| Element | Kind | Type | Declared in |") {
		t.Errorf("Markdown carries no table header:\n%s", markdown)
	}
	text := rendering.Text()
	if !strings.Contains(text, "Element ") || !strings.Contains(text, "\n------") {
		t.Errorf("text is no aligned table:\n%s", text)
	}
	_, err = rendering.Write(FormMermaid)
	var wrong *WrongFormError
	if !errors.As(err, &wrong) || !errors.Is(err, ErrWrongForm) {
		t.Fatalf("write mermaid error = %v, want a *WrongFormError", err)
	}
	if !strings.Contains(err.Error(), string(FormMarkdown)) {
		t.Errorf("error %q does not name the form a table is written in", err)
	}
	// The graph-shaped kinds are the other way round.
	if _, err := render(t, "tree.sysml", "VehicleViews::vehicleView").Write(FormMarkdown); !errors.Is(err, ErrWrongForm) {
		t.Errorf("markdown of a tree error = %v, want ErrWrongForm", err)
	}
}

// A rendering kind OpenSysML does not produce is a typed error naming the kind
// and the view, never another kind's rendering.
func TestUnsupportedRenderingKinds(t *testing.T) {
	r, idx := loadFixture(t, "errors.sysml")
	cases := []struct {
		view string
		kind Kind
		says string
	}{
		{"ErrorViews::geometryView", KindGeometry, "view def GeometryView"},
	}
	for _, tc := range cases {
		t.Run(tc.view, func(t *testing.T) {
			rendering, err := r.Render(lookup(t, idx, tc.view))
			if rendering != nil {
				t.Fatalf("rendered %v, want no rendering", rendering)
			}
			var unsupported *UnsupportedKindError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error = %v, want an *UnsupportedKindError", err)
			}
			if !errors.Is(err, ErrUnsupportedKind) {
				t.Errorf("error does not wrap ErrUnsupportedKind: %v", err)
			}
			if unsupported.Kind != tc.kind {
				t.Errorf("Kind = %q, want %q", unsupported.Kind, tc.kind)
			}
			if !strings.Contains(err.Error(), tc.view) || !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q names neither the view nor how it states the kind (%q)", err, tc.says)
			}
		})
	}
}

// A name that is no view is semantics.ErrNotAView, as %view reports it, from
// both Render and KindOf.
func TestRenderingSomethingThatIsNoView(t *testing.T) {
	r, idx := loadFixture(t, "errors.sysml")
	for _, name := range []string{"Kit", "Kit::Widget"} {
		if _, err := r.Render(lookup(t, idx, name)); !errors.Is(err, semantics.ErrNotAView) {
			t.Errorf("Render(%s) error = %v, want ErrNotAView", name, err)
		}
	}
	if _, err := r.Render(nil); !errors.Is(err, semantics.ErrNotAView) {
		t.Errorf("Render(nil) error = %v, want ErrNotAView", err)
	}
	if _, _, err := r.KindOf(nil); !errors.Is(err, semantics.ErrNotAView) {
		t.Errorf("KindOf(nil) error = %v, want ErrNotAView", err)
	}
}

// A view exposing nothing renders an empty artifact and says so, in both forms.
func TestRenderingAViewExposingNothing(t *testing.T) {
	rendering := render(t, "errors.sysml", "ErrorViews::emptyView")
	if !rendering.Empty() {
		t.Fatalf("rendering is not empty: %+v", rendering.Roots)
	}
	if len(rendering.Notices) != 0 {
		t.Errorf("notices = %v, want none: the view exposes nothing at all", rendering.Notices)
	}
	if !strings.Contains(rendering.Text(), "the view exposes nothing") {
		t.Errorf("text does not say the view exposes nothing:\n%s", rendering.Text())
	}
	if !strings.Contains(rendering.Mermaid(), "the view exposes nothing") {
		t.Errorf("Mermaid does not say the view exposes nothing:\n%s", rendering.Mermaid())
	}
}

// An empty state rendering says so as a state, since a state diagram takes a
// note only attached to a state.
func TestEmptyStateRenderingIsAStateNotABareNote(t *testing.T) {
	rendering := render(t, "errors.sysml", "ErrorViews::emptyStateView")
	if rendering.Kind != KindState || !rendering.Empty() {
		t.Fatalf("rendering = %s, empty = %v, want an empty state rendering", rendering.Kind, rendering.Empty())
	}
	mermaid := rendering.Mermaid()
	if !strings.Contains(mermaid, "state \"") || !strings.Contains(mermaid, "as empty") {
		t.Errorf("the empty state diagram holds no state:\n%s", mermaid)
	}
	for _, line := range strings.Split(mermaid, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "note \"") {
			t.Errorf("a bare note is no state diagram: %q", line)
		}
	}
}

// An element a rendering cannot represent is reported, not dropped.
func TestRenderingReportsWhatItCannotRepresent(t *testing.T) {
	rendering := render(t, "errors.sysml", "ErrorViews::misfitView")
	if len(rendering.Notices) != 1 || !strings.Contains(rendering.Notices[0], "Kit::Widget") {
		t.Fatalf("notices = %v, want one naming Kit::Widget", rendering.Notices)
	}
	if !strings.Contains(rendering.Text(), "not represented:") {
		t.Errorf("text does not report what it dropped:\n%s", rendering.Text())
	}
	// The kind is named with the article it reads with, "an action" rather than "a action".
	if !strings.Contains(rendering.Text(), "the rendering is empty: nothing the view exposes is shown by an action rendering") {
		t.Errorf("text does not say the rendering came out empty:\n%s", rendering.Text())
	}
	if !strings.Contains(rendering.Mermaid(), "%% not represented:") {
		t.Errorf("Mermaid loses the notice:\n%s", rendering.Mermaid())
	}
}

// A Mermaid label carries no character that would break the diagram.
func TestMermaidLabelsAreEscaped(t *testing.T) {
	mermaid := render(t, "action.sysml", "FlowViews::driveView").Mermaid()
	for _, line := range strings.Split(mermaid, "\n") {
		if strings.HasPrefix(line, "%%") {
			continue
		}
		if i := strings.Index(line, "[\""); i >= 0 {
			label := line[i+2 : strings.LastIndex(line, "\"")]
			if strings.ContainsAny(label, "\"<>") {
				t.Errorf("unescaped label %q in %q", label, line)
			}
		}
	}
}

// nodeNames collects the names of a rendering's nodes, at every depth.
func nodeNames(nodes []*Node) map[string]bool {
	out := map[string]bool{}
	var walk func([]*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			out[node.Name] = true
			walk(node.Children)
		}
	}
	walk(nodes)
	return out
}

// rowNames collects the element each row of a table is about.
func rowNames(rows [][]string) map[string]bool {
	out := map[string]bool{}
	for _, row := range rows {
		if len(row) > 0 {
			out[row[0]] = true
		}
	}
	return out
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return sorted(out)
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func checkGolden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}
