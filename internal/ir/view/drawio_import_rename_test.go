package view

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// setLabel rewrites the label attribute of the object cell id, the only
// attribute draw.io desktop rewrites when a user edits a cell's text.
func setLabel(t *testing.T, document, id, label string) string {
	t.Helper()
	re := regexp.MustCompile(`(<object id="` + regexp.QuoteMeta(id) + `") label="[^"]*"`)
	if !re.MatchString(document) {
		t.Fatalf("no object %s in the document", id)
	}
	return re.ReplaceAllString(document, `$1 label="`+strings.ReplaceAll(label, "\n", "&#xA;")+`"`)
}

func layoutGolden(t *testing.T) string {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join("testdata", layoutCase.name+".drawio.golden"))
	if err != nil {
		t.Fatal(err)
	}
	return string(golden)
}

func TestDrawioImportRenamedNodeIsARename(t *testing.T) {
	document := setLabel(t, layoutGolden(t), "n1", "«part»\npumpX")
	imp := drawioGoldenImport(t, layoutCase, document)
	edit := onlyEdit(t, imp, DrawioEditRename)
	if edit.FQN != "Plant::Loop::pump" || edit.NewName != "pumpX" || edit.Cell != "n1" || len(imp.Notices) != 0 {
		t.Errorf("rename = %+v, notices %v", edit, imp.Notices)
	}
}

func TestDrawioImportRenamedPortTargetsTheUsagePath(t *testing.T) {
	document := setLabel(t, layoutGolden(t), "n1:n1.0", "^discharge")
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditRename)
	if edit.FQN != "Plant::Loop::pump::outlet" || edit.NewName != "discharge" {
		t.Errorf("rename = %s to %q, want the port by the path a Layout of it takes", edit.FQN, edit.NewName)
	}
}

func TestDrawioImportRenameToAQuotedName(t *testing.T) {
	document := setLabel(t, layoutGolden(t), "n1", "«part»\n'main pump'")
	edit := onlyEdit(t, drawioGoldenImport(t, layoutCase, document), DrawioEditRename)
	if edit.NewName != "'main pump'" {
		t.Errorf("new name = %q", edit.NewName)
	}
}

func TestDrawioImportDecodesFormattedTextLabels(t *testing.T) {
	for _, label := range []string{"«part»&lt;div&gt;pumpX&lt;/div&gt;", "«part»&lt;br&gt;pumpX", "&amp;laquo;part&amp;raquo;&lt;br style=&quot;x&quot;&gt;pumpX"} {
		document := setLabel(t, layoutGolden(t), "n1", label)
		document = mutateCell(t, document, "n1", func(cell string) string { return strings.Replace(cell, "html=0;", "html=1;", 1) })
		imp := drawioGoldenImport(t, layoutCase, document)
		if len(imp.Edits) != 1 || imp.Edits[0].Kind != DrawioEditRename || imp.Edits[0].NewName != "pumpX" {
			t.Errorf("%s: edits %+v notices %q unknown %d, want one Rename to pumpX", label, imp.Edits, imp.Notices, len(imp.Unknown))
		}
	}
}

func TestDrawioImportRenameAndMoveOrderTheRenameLast(t *testing.T) {
	document := setLabel(t, layoutGolden(t), "n2", "«part»\nreservoir")
	document = mutateCell(t, document, "n2", func(cell string) string { return setGeometryAttr(t, cell, "x", "533") })
	imp := drawioGoldenImport(t, layoutCase, document)
	last := imp.Edits[len(imp.Edits)-1]
	if last.Kind != DrawioEditRename || last.FQN != "Plant::Loop::tank" || last.NewName != "reservoir" {
		t.Fatalf("last edit = %+v, want the Rename of the tank after every Layout", last)
	}
	// Moving a container's child pins the container's children: Layouts first, the rename last.
	moved := false
	for _, e := range imp.Edits[:len(imp.Edits)-1] {
		if e.Kind != DrawioEditLayout {
			t.Errorf("before the Rename: %+v, want Layouts only", e)
		}
		moved = moved || e.Cell == "n2" && e.FQN == last.FQN
	}
	if !moved {
		t.Errorf("edits %+v, want a Layout of the tank", imp.Edits)
	}
}

func TestDrawioImportReportsLabelEditsItCannotRename(t *testing.T) {
	cases := []struct{ name, id, label, notice string }{
		{"beyond the name", "n0", "«part def»\nLoop : Thing", "changed beyond the name \"Loop\""},
		{"keyword", "n1", "«item»\npump", "changed beyond the name"},
		{"two words", "n1", "«part»\npump x", "changed beyond the name"},
		{"not a name", "n1", "«part»\n1pump", `"1pump", which is not an identifier`},
		{"a keyword", "n1", "«part»\npart", `"part", which is a keyword`},
		{"empty", "n1", "«part»\n", `"", which is empty`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			imp := drawioGoldenImport(t, layoutCase, setLabel(t, layoutGolden(t), c.id, c.label))
			if len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], c.notice) {
				t.Errorf("edits %+v notices %q, want none and one saying %q", imp.Edits, imp.Notices, c.notice)
			}
		})
	}
}

func TestDrawioImportSameLabelIsNoEdit(t *testing.T) {
	document := setLabel(t, layoutGolden(t), "n1", "«part»\npump")
	if imp := drawioGoldenImport(t, layoutCase, document); len(imp.Edits)+len(imp.Notices) != 0 {
		t.Errorf("edits %+v notices %v, want nothing", imp.Edits, imp.Notices)
	}
}

func TestDrawioImportReportsRenamedCopiesThatDisagree(t *testing.T) {
	c, golden := drawioGolden(t, "abstract-tree")
	// The tree draws Vehicle::mass under Vehicle and again, inherited, under each kind of vehicle.
	imp := drawioGoldenImport(t, c, setLabel(t, golden, "n1", "«attribute»\nmassX"))
	if len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], "AbstractViews::Vehicle::mass: the view draws it twice and the copies disagree") {
		t.Errorf("edits %+v notices %q", imp.Edits, imp.Notices)
	}
	// Renamed alike on every copy, the rename imports once.
	document := golden
	for _, id := range []string{"n1", "n7", "n10", "n15"} {
		document = setLabel(t, document, id, strings.Replace(drawioLabelOf(t, document, id), "mass", "massX", 1))
	}
	edit := onlyEdit(t, drawioGoldenImport(t, c, document), DrawioEditRename)
	if edit.FQN != "AbstractViews::Vehicle::mass" || edit.NewName != "massX" {
		t.Errorf("rename = %+v", edit)
	}
}

// drawioLabelOf is the label attribute of the object cell id, decoded.
func drawioLabelOf(t *testing.T, document, id string) string {
	t.Helper()
	m := regexp.MustCompile(`<object id="` + regexp.QuoteMeta(id) + `" label="([^"]*)"`).FindStringSubmatch(document)
	if m == nil {
		t.Fatalf("no object %s", id)
	}
	return html.UnescapeString(m[1])
}
