package view

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// drawioGolden is the golden case of the fixture name and its document.
func drawioGolden(t *testing.T, name string) (struct {
	name string
	file string
	view string
	kind Kind
}, string) {
	t.Helper()
	for _, c := range plantumlGoldenCases {
		if c.name != name {
			continue
		}
		golden, err := os.ReadFile(filepath.Join("testdata", name+".drawio.golden"))
		if err != nil {
			t.Fatal(err)
		}
		return c, string(golden)
	}
	t.Fatalf("no golden case %s", name)
	panic("unreachable")
}

// drawioObjectXML is the whole <object> wrapper of the semantic cell id.
func drawioObjectXML(t *testing.T, document, id string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<object id="` + regexp.QuoteMeta(id) + `"[^>]*>.*?</object>`).FindString(document)
	if m == "" {
		t.Fatalf("no object %s in the document", id)
	}
	return m
}

// swapObjects brings the object b before a, as draw.io does when a cell is
// brought to front: the file order changes, the elements do not.
func swapObjects(t *testing.T, document, a, b string) string {
	t.Helper()
	objA, objB := drawioObjectXML(t, document, a), drawioObjectXML(t, document, b)
	if strings.Index(document, objA) > strings.Index(document, objB) {
		t.Fatalf("%s is not before %s", a, b)
	}
	document = strings.Replace(document, objB, "", 1)
	return strings.Replace(document, objA, objB+objA, 1)
}

func drawioCount(t *testing.T, document, fqn string) int {
	t.Helper()
	return strings.Count(document, `fqn="`+fqn+`"`)
}

// The tree draws Vehicle::mass once declared and three times inherited; the
// cells share one fqn, so the order of the file must not decide which is which,
// and the inherited one moved is placed as its inheritor's (vehicleFamily::mass).
func TestDrawioImportReorderedDuplicatesKeepTheirIdentity(t *testing.T) {
	c, golden := drawioGolden(t, "abstract-tree")
	const mass = "AbstractViews::Vehicle::mass"
	if n := drawioCount(t, golden, mass); n < 2 {
		t.Fatalf("%s is drawn %d times, the test needs a repeated element", mass, n)
	}
	reordered := swapObjects(t, golden, "n1", "n7")
	if imp := drawioGoldenImport(t, c, reordered); len(imp.Edits)+len(imp.Unmatched)+len(imp.Deleted) != 0 {
		t.Fatalf("reordering alone: edits %+v unmatched %v deleted %v", imp.Edits, imp.Unmatched, imp.Deleted)
	}
	moved := mutateCell(t, reordered, "n7", func(cell string) string { return setGeometryAttr(t, cell, "x", "900") })
	imp := drawioGoldenImport(t, c, moved)
	edit := editOf(t, imp, DrawioEditLayout, "n7")
	rendering, _ := renderWithSites(t, c.file, c.view)
	var n7 *Node
	for _, node := range drawioNodes(rendering.Roots) {
		if node.ID == "n7" {
			n7 = node
		}
	}
	if n7 == nil {
		t.Fatal("no node n7 in the rendering")
	}
	if edit.Cell != "n7" || edit.FQN != "AbstractViews::vehicleFamily::mass" || edit.Origin != n7.Origin || !strings.Contains(edit.Label, "^mass") {
		t.Errorf("edit = %+v, want the inherited mass at n7 (origin %+v)", edit, n7.Origin)
	}
	if len(imp.Unmatched)+len(imp.Deleted) != 0 {
		t.Errorf("unmatched %v deleted %v", imp.Unmatched, imp.Deleted)
	}
}

// A start pseudostate is declared nowhere; the writer names it as the member
// its owner's body knows, so moving it is a Layout about Owner::start.
func TestDrawioImportMovedStartIsALayoutAboutTheMember(t *testing.T) {
	c, golden := drawioGolden(t, "state")
	const start = "Machines::VehicleStates::start"
	if !strings.Contains(drawioObjectXML(t, golden, "n5"), `label="start" fqn="`+start+`" kind="start"`) {
		t.Fatalf("n5 is not the start of the machine:\n%s", drawioObjectXML(t, golden, "n5"))
	}
	moved := mutateCell(t, golden, "n5", func(cell string) string { return setGeometryAttr(t, cell, "y", "333") })
	file, err := ReadDrawio([]byte(moved), c.view)
	if err != nil {
		t.Fatal(err)
	}
	imp := drawioGoldenImport(t, c, moved)
	edit := editOf(t, imp, DrawioEditLayout, "n5")
	if edit.Cell != "n5" || edit.FQN != start || edit.Label != "start" || edit.Origin.Located() || edit.Layout.Y != file.Cell("n5").Geometry.Y {
		t.Errorf("edit = %+v, want the start by its member name at the absolute y %v", edit, file.Cell("n5").Geometry.Y)
	}
	if len(imp.Unmatched)+len(imp.Deleted)+len(imp.Notices) != 0 {
		t.Errorf("unmatched %v deleted %v notices %v", imp.Unmatched, imp.Deleted, imp.Notices)
	}
	// Under another id the cell is still the start: nothing else carries its name.
	renumbered := strings.ReplaceAll(strings.Replace(golden, `<object id="n5" label="start"`, `<object id="n55" label="start"`, 1), `source="n5"`, `source="n55"`)
	if imp := drawioGoldenImport(t, c, renumbered); len(imp.Edits)+len(imp.Unmatched)+len(imp.Deleted) != 0 {
		t.Errorf("renumbered start: edits %+v unmatched %v deleted %v", imp.Edits, imp.Unmatched, imp.Deleted)
	}
}

// A port is drawn by its direction, so the writer gives it no colours of its
// own; recolouring one in draw.io is reported, not written as a Style.
func TestDrawioImportRecolouredPortIsReported(t *testing.T) {
	c, golden := drawioGolden(t, layoutCase.name)
	recoloured := mutateCell(t, golden, "n1:n1.0", func(cell string) string {
		return strings.Replace(cell, `style="`, `style="fillColor=#ff0000;`, 1)
	})
	imp := drawioGoldenImport(t, c, recoloured)
	if len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], "Plant::Pump::outlet: a port is drawn by its direction") {
		t.Errorf("edits %+v notices %v, want the port reported", imp.Edits, imp.Notices)
	}
}

// Reconnecting a note's anchor edge in draw.io does not change which element
// the Note annotates; the import says so instead of dropping it.
func TestDrawioImportReanchoredNoteIsReported(t *testing.T) {
	c, golden := drawioGolden(t, layoutCase.name)
	anchor, _ := drawioCellXML(t, golden, "note0:anchor0")
	if !strings.Contains(anchor, `target="n1"`) {
		t.Fatalf("anchor = %s", anchor)
	}
	reanchored := strings.Replace(golden, anchor, strings.Replace(anchor, `target="n1"`, `target="n2"`, 1), 1)
	imp := drawioGoldenImport(t, c, reanchored)
	if len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], `note "always" re-anchored`) {
		t.Errorf("edits %+v notices %v, want the note reported", imp.Edits, imp.Notices)
	}
}

// Deleting every semantic cell leaves a page of the view its name states, so
// the import reports the deletions instead of refusing the page.
func TestDrawioImportEmptiedPageReportsEveryDeletion(t *testing.T) {
	c, golden := drawioGolden(t, layoutCase.name)
	emptied := regexp.MustCompile(`(?s)<object .*?</object>`).ReplaceAllString(golden, "")
	if strings.Contains(emptied, "fqn=") {
		t.Fatal("objects left")
	}
	imp := drawioGoldenImport(t, c, emptied)
	if len(imp.Edits) != 0 || len(imp.Deleted) < 3 || len(imp.Unmatched) != 0 {
		t.Errorf("edits %+v deleted %v unmatched %v, want every element deleted", imp.Edits, imp.Deleted, imp.Unmatched)
	}
	for _, fqn := range []string{"Plant::Loop::pump", "Plant::Loop::tank", "Plant::Pump::outlet"} {
		found := false
		for _, d := range imp.Deleted {
			found = found || d == fqn
		}
		if !found {
			t.Errorf("deleted %v lacks %s", imp.Deleted, fqn)
		}
	}
}

// An element a nested view exposes is drawn under the view's node but is not
// the view's member: a Layout about it names the element, not a path through
// the view.
func TestDrawioImportNestedViewElementKeepsItsOwnName(t *testing.T) {
	c, golden := drawioGolden(t, "tree")
	const engine = "Vehicles::Engine"
	if !strings.Contains(drawioObjectXML(t, golden, "n7"), `fqn="`+engine+`"`) {
		t.Fatalf("n7 is not Engine:\n%s", drawioObjectXML(t, golden, "n7"))
	}
	file, err := ReadDrawio([]byte(golden), c.view)
	if err != nil {
		t.Fatal(err)
	}
	if parent := file.Cell(file.Cell("n7").Parent); parent == nil || parent.FQN != "VehicleViews::vehicleView::engineSubview" {
		t.Fatalf("n7's parent = %+v, want the nested view", parent)
	}
	moved := mutateCell(t, golden, "n7", func(cell string) string { return setGeometryAttr(t, cell, "y", "444") })
	edit := editOf(t, drawioGoldenImport(t, c, moved), DrawioEditLayout, "n7")
	if edit.FQN != engine {
		t.Errorf("edit names %q, want %q", edit.FQN, engine)
	}
}

// A usage's typing edge drawn beside its membership edge follows a Route naming
// its type, which no edit writes yet: re-routing it is reported, while
// re-routing the membership edge, which the plain Route steers, is a Route.
func TestDrawioImportReroutedTypingEdgeBesideMembershipIsReported(t *testing.T) {
	c, golden := drawioGolden(t, "mixed")
	typing, membership := drawioObjectXML(t, golden, "e37"), drawioObjectXML(t, golden, "e15")
	if !strings.Contains(typing, `kind="typing"`) || !strings.Contains(typing, `source="n26" target="n21"`) ||
		!strings.Contains(membership, `kind="composition"`) || !strings.Contains(membership, `target="n26"`) {
		t.Fatalf("e37/e15 are not pump's typing and membership edges:\n%s\n%s", typing, membership)
	}
	const points = `<Array as="points"><mxPoint x="600" y="300"/></Array>`
	reroute := func(cell string) string {
		if strings.Contains(cell, "<Array") {
			return regexp.MustCompile(`(?s)<Array as="points">.*?</Array>`).ReplaceAllString(cell, points)
		}
		return strings.Replace(cell, `<mxGeometry relative="1" as="geometry"/>`, `<mxGeometry relative="1" as="geometry">`+points+`</mxGeometry>`, 1)
	}
	imp := drawioGoldenImport(t, c, mutateCell(t, golden, "e37", reroute))
	if len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], "no Route steers a typing edge") {
		t.Errorf("typing edge: edits %+v notices %v, want one notice and no edit", imp.Edits, imp.Notices)
	}
	imp = drawioGoldenImport(t, c, mutateCell(t, golden, "e15", reroute))
	if edit := onlyEdit(t, imp, DrawioEditRoute); edit.Cell != "e15" || edit.Route == nil || len(edit.Route.Points) != 1 {
		t.Errorf("membership edge: edit %+v, want its one waypoint as a Route", edit)
	}
}
