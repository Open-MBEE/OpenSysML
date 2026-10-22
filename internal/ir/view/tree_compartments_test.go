package view

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenTreeCompartments locks every graphical form of a positioned tree
// whose layout-less members list in compartments of their owner's box.
func TestGoldenTreeCompartments(t *testing.T) {
	rendering := render(t, "tree-compartments.sysml", "AmplifierViews::positioned")
	for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormText} {
		out, err := rendering.Write(form)
		if err != nil {
			t.Fatalf("write %s: %v", form, err)
		}
		checkGolden(t, filepath.Join("testdata", "tree-compartments."+string(form)+".golden"), out)
	}
	cameo, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOTWith(cameo): %v", err)
	}
	checkGolden(t, filepath.Join("testdata", "tree-compartments.cameo.dot.golden"), cameo)
	for _, view := range []string{"cornered", "wrapped", "cramped", "bare"} {
		dot, err := render(t, "tree-compartments.sysml", "AmplifierViews::"+view).Write(FormDot)
		if err != nil {
			t.Fatalf("write %s: %v", view, err)
		}
		checkGolden(t, filepath.Join("testdata", "tree-compartments-"+view+".dot.golden"), dot)
	}
}

// TestTreeCompartmentsFold checks which members of a positioned tree fold into
// compartment rows: the layout-less ones, grouped by kind in the notation's
// order, while a member with a Layout of its own stays a node and the
// unpositioned tree is left as it is.
func TestTreeCompartmentsFold(t *testing.T) {
	rendering := render(t, "tree-compartments.sysml", "AmplifierViews::positioned")
	folded := foldCompartments(rendering)
	if folded == rendering {
		t.Fatal("foldCompartments returned the rendering unchanged")
	}
	if len(rendering.Roots[0].Compartments) != 0 || len(rendering.Roots[0].Children) != 10 {
		t.Errorf("folding changed the rendering it was given: %d compartments, %d children", len(rendering.Roots[0].Compartments), len(rendering.Roots[0].Children))
	}
	stage := folded.Roots[0]
	var names []string
	for _, child := range stage.Children {
		names = append(names, child.Name)
	}
	if got := strings.Join(names, ","); got != "housing,shield" {
		t.Errorf("children kept as nodes = %q, want the two with a Layout", got)
	}
	var got []string
	for _, compartment := range stage.Compartments {
		var rows []string
		for _, row := range compartment.Rows {
			rows = append(rows, row.Name)
		}
		got = append(got, compartment.Name+": "+strings.Join(rows, ", "))
	}
	want := []string{"attributes: gain, noiseFigure", "parts: heatsink", "ports: input, output", "actions: calibrate", "constraints: bound", "item defs: Slot"}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Errorf("compartments = %q, want %q", got, want)
	}
	band := folded.Roots[2]
	if len(band.Compartments) != 1 || band.Compartments[0].Name != "enumerations" || len(band.Compartments[0].Rows) != 2 {
		t.Errorf("Band compartments = %+v, want one `enumerations` compartment of two literals", band.Compartments)
	}
	for _, edge := range folded.Edges {
		for _, id := range []string{edge.From, edge.To} {
			if strings.HasPrefix(id, "n") && !hasNode(folded.Roots, id) {
				t.Errorf("edge %s->%s at a node folded into a row", edge.From, edge.To)
			}
		}
	}
	plain := render(t, "tree-compartments.sysml", "AmplifierViews::plain")
	if foldCompartments(plain) != plain {
		t.Error("an unpositioned tree was folded")
	}
	bare := render(t, "tree-compartments.sysml", "AmplifierViews::bare")
	if foldCompartments(bare) != bare {
		t.Error("a tree not showing compartments was folded")
	}
}

func hasNode(nodes []*Node, id string) bool {
	for _, node := range nodes {
		if node.ID == id || hasNode(node.Children, id) {
			return true
		}
	}
	return false
}

// TestTreeCompartmentRows checks the grouping and text of compartment rows:
// the compartments in the notation's order whatever the declaration order, a
// modifier kept on the row, the type after the colon, the value after the
// equals sign, and a member with nothing else to show read as its kind.
func TestTreeCompartmentRows(t *testing.T) {
	rows := []*Node{
		{ID: "a", Kind: "constraint", Name: "bound", Type: "GainBound"},
		{ID: "b", Kind: "ref part", Name: "heatsink", Type: "Heatsink"},
		{ID: "c", Kind: "attribute", Name: "gain", Type: "dB", Value: "15.0"},
		{ID: "d", Kind: "calc", Name: "total"},
		{ID: "e", Kind: "attribute", Name: "noiseFigure", Type: "ScalarValues::Real"},
		{ID: "f", Kind: "metadata", Name: "", Type: "Safety"},
		{ID: "g", Kind: "widget", Name: "w"},
		{ID: "h", Kind: "attribute", Name: "x", NameSynthesized: true},
		{ID: "i", Kind: "ref part", Name: "y", NameSynthesized: true, Value: "z"},
	}
	compartments := compartmentsOf(rows)
	labels := labeller{shown: Shown{ElementCompartments: true, ElementTypings: true}}
	var got []string
	for _, compartment := range compartments {
		var texts []string
		for _, row := range compartment.Rows {
			texts = append(texts, labels.rowText(row))
		}
		got = append(got, compartment.Name+": "+strings.Join(texts, " | "))
	}
	want := []string{
		"attributes: gain : dB = 15.0 | noiseFigure : Real | attribute",
		"parts: ref heatsink : Heatsink | ref part = z",
		"calculations: total",
		"constraints: bound : GainBound",
		"metadata: : Safety",
		"widgets: w",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("compartments:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if compartmentsOf(nil) != nil {
		t.Error("compartmentsOf(nil) != nil")
	}
	hidden := labeller{shown: Shown{}}
	if lines := hidden.compartmentLines(&Node{Compartments: compartments}); lines != nil {
		t.Errorf("compartment lines with compartments not shown = %q, want none", lines)
	}
}

// TestTreeCompartmentsClipped checks a stated box keeps its Layout size and
// the rows it has no room for are left unwritten in order, each compartment
// kept only with a row under it, and counted in a notice.
func TestTreeCompartmentsClipped(t *testing.T) {
	dot, err := render(t, "tree-compartments.sysml", "AmplifierViews::cramped").Write(FormDot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dot, "// not represented: 10 node(s) without a position, drawn as compartment rows of the node above them, and 1 node(s) under them left undrawn\n") {
		t.Errorf("the notice does not count the rows and the member under one apart:\n%s", dot)
	}
	if !strings.Contains(dot, "// not represented: 9 compartment row(s) with no room in the stated size of their node's box, left unwritten") {
		t.Errorf("no clipping notice in:\n%s", dot)
	}
	if !strings.Contains(dot, `width=2.2222222222222223, height=1.25, fixedsize=true`) {
		t.Errorf("the Stage box is not kept at its 160x90 Layout size:\n%s", dot)
	}
	if !strings.Contains(dot, `<tr><td align="left">gain = 15.0</td></tr></table>>`) {
		t.Errorf("the attributes compartment, with the one row that fits, is not the last written:\n%s", dot)
	}
	for _, absent := range []string{"noiseFigure", ">parts<", ">ports<", "heatsink", "calibrate"} {
		if strings.Contains(dot, absent) {
			t.Errorf("%q written though it has no room:\n%s", absent, dot)
		}
	}
	wide, err := render(t, "tree-compartments.sysml", "AmplifierViews::positioned").Write(FormDot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(wide, "<hr/>") != 4+1 {
		t.Errorf("want four compartments in Stage's 240x260 box and one in Band's, each under a rule, got:\n%s", wide)
	}
	if !strings.Contains(wide, "// not represented: 2 compartment row(s)") {
		t.Errorf("want the two remaining rows of Stage clipped:\n%s", wide)
	}
	cornered, err := render(t, "tree-compartments.sysml", "AmplifierViews::cornered").Write(FormDot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cornered, "compartment row(s) with no room") || strings.Count(cornered, "<hr/>") != 6 {
		t.Errorf("want a box with a corner and no size sized to all six compartments:\n%s", cornered)
	}
	wrapped, err := render(t, "tree-compartments.sysml", "AmplifierViews::wrapped").Write(FormDot)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<td align="left">attribute<br/>&#39;upper<br/>lower&#39;<br/>length = StringFunctions::Length(&#34;a  b&#34;)</td>`} {
		if !strings.Contains(wrapped, want) {
			t.Errorf("want %q in the rows of:\n%s", want, wrapped)
		}
	}
}

// TestTreeCompartmentExtent checks a corner-only box's extent counts every
// line a row is displayed on.
func TestTreeCompartmentExtent(t *testing.T) {
	labels := labeller{shown: Shown{ElementCompartments: true}}
	one := &Node{Kind: "part def", Name: "A", Compartments: []Compartment{{Name: "attributes", Rows: []*Node{{Kind: "attribute", Name: "upper"}}}}}
	two := &Node{Kind: "part def", Name: "A", Compartments: []Compartment{{Name: "attributes", Rows: []*Node{{Kind: "attribute", Name: "upper\rlower"}}}}}
	_, oneHeight := labels.dotLabelExtent(one)
	_, twoHeight := labels.dotLabelExtent(two)
	if math.Abs(twoHeight-oneHeight-labels.sizeOf(one)*dotLineEm) > 1e-9 {
		t.Errorf("a two-line row adds %v to the extent, want one line of %v", twoHeight-oneHeight, labels.sizeOf(one)*dotLineEm)
	}
}

// TestCollapseSpaceKeepsLiterals checks notation text collapses its formatting
// whitespace and none inside a string literal or a quoted name.
func TestCollapseSpaceKeepsLiterals(t *testing.T) {
	for in, want := range map[string]string{
		"f(\n\t1,  2)":            "f( 1, 2)",
		`format("a  b",  'c  d')`: `format("a  b", 'c  d')`,
		`"a \" b"  + x`:           `"a \" b" + x`,
		"  lead and  trail  ":     "lead and trail",
	} {
		if got := collapseSpace(in); got != want {
			t.Errorf("collapseSpace(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTreeCompartmentVariantForms pins the two variation compartments of the
// graphical notation (SysML v2.1 §8.2.3.6, `compartment`): a shorthand
// `variant x` files under "variants", a typed `variant part x` under
// "variant parts" with no prefix on its row, while `variation` is a modifier
// kept on the row of an ordinary "parts" compartment.
func TestTreeCompartmentVariantForms(t *testing.T) {
	rows := []*Node{
		{ID: "a", Kind: "variant", Name: "manual"},
		{ID: "b", Kind: "variant part", Name: "automatic", Type: "Transmission"},
		{ID: "c", Kind: "variation part", Name: "transmission", Type: "Transmission"},
		{ID: "d", Kind: "part", Name: "engine", Type: "Engine"},
		{ID: "e", Kind: "variant attribute", Name: "large", Value: "100[mm]"},
	}
	labels := labeller{shown: Shown{ElementCompartments: true, ElementTypings: true}}
	var got []string
	for _, compartment := range compartmentsOf(rows) {
		var texts []string
		for _, row := range compartment.Rows {
			texts = append(texts, labels.rowText(row))
		}
		got = append(got, compartment.Name+": "+strings.Join(texts, " | "))
	}
	want := []string{
		"parts: variation transmission : Transmission | engine : Engine",
		"variants: manual",
		"variant parts: automatic : Transmission",
		"variant attributes: large = 100[mm]",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("compartments:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
