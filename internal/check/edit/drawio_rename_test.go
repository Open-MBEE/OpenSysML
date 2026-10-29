package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

const drawioLoop = `package Plant {
	port def FluidPort;
	part def Pump {
		port outlet : FluidPort;
	}
	part def Tank {
		port inlet : FluidPort;
	}
	part def Loop {
		part pump : Pump;
		part tank : Tank;
		connect pump.outlet to tank.inlet;
	}
	view loopView {
		expose Loop::*;
	}
}
`

// drawioCellOf is the exported cell of the element named, by its kind.
func drawioCellOf(t *testing.T, file *view.DrawioFile, kind, fqn string) *view.DrawioCell {
	t.Helper()
	cell := drawioFirst(file, func(c *view.DrawioCell) bool { return c.Kind == kind && c.FQN == fqn })
	if cell == nil {
		t.Fatalf("no %s %s among the cells", kind, fqn)
	}
	return cell
}

func drawioRenameOf(t *testing.T, imp *view.DrawioImport) view.DrawioEdit {
	t.Helper()
	if len(imp.Edits) != 1 || imp.Edits[0].Kind != view.DrawioEditRename {
		t.Fatalf("edits %+v notices %q, want one Rename", imp.Edits, imp.Notices)
	}
	return imp.Edits[0]
}

func TestDrawioImportRenamesAPortAndItsConnectionEnds(t *testing.T) {
	m, rendering, options := drawioModel(t, drawioLoop, "Plant::loopView")
	document, exported := drawioExport(t, rendering, options)
	port := drawioCellOf(t, exported, "port", "Plant::Pump::outlet")
	edited := drawioRelabel(t, document, port.ID, func(label string) string { return strings.Replace(label, "outlet", "discharge", 1) })
	_, imp := drawioImport(t, rendering, options, edited)
	if e := drawioRenameOf(t, imp); e.FQN != "Plant::Loop::pump::outlet" || e.NewName != "discharge" {
		t.Fatalf("rename = %s to %q, want the port by the path a Layout of it takes", e.FQN, e.NewName)
	}
	res, err := Apply(m, DrawioOperations(imp, false))
	if err != nil {
		t.Fatal(err)
	}
	content := string(res.Content)
	if !strings.Contains(content, "port discharge : FluidPort;") || !strings.Contains(content, "connect pump.discharge to tank.inlet;") || strings.Contains(content, "outlet") {
		t.Errorf("renamed model:\n%s", content)
	}
	_, redrawn, reoptions := drawioModel(t, content, "Plant::loopView")
	_, re := drawioExport(t, redrawn, reoptions)
	if again := drawioCellOf(t, re, "port", "Plant::Pump::discharge"); again.Label != strings.Replace(port.Label, "outlet", "discharge", 1) {
		t.Errorf("re-exported label %q", again.Label)
	}
}

func TestDrawioImportRenameRewritesAnotherDocument(t *testing.T) {
	const other = "package Other {\n\tpart p : Plant::Pump;\n}\n"
	m := loadEditableWorkspace(t, "model.sysml", drawioLoop, map[string]string{"other.sysml": other})
	m, rendering, options := drawioModelOf(t, m, "Plant::loopView")
	document, exported := drawioExport(t, rendering, options)
	pump := drawioCellOf(t, exported, "part", "Plant::Loop::pump")
	_, imp := drawioImport(t, rendering, options, drawioRelabel(t, document, pump.ID, func(label string) string { return label + "2" }))
	if e := drawioRenameOf(t, imp); e.FQN != "Plant::Loop::pump" || e.NewName != "pump2" {
		t.Fatalf("rename = %+v", e)
	}
	res, err := Apply(m, DrawioOperations(imp, false))
	if err != nil {
		t.Fatal(err)
	}
	if content := string(res.Content); !strings.Contains(content, "part pump2 : Pump;") || !strings.Contains(content, "connect pump2.outlet") {
		t.Errorf("renamed model:\n%s", content)
	}
	if len(res.Others) != 0 {
		t.Errorf("a usage nothing else names rewrote %+v", res.Others)
	}
	// The definition the other document types its part with.
	_, rendering, options = drawioModelOf(t, loadEditableWorkspace(t, "model.sysml", strings.Replace(drawioLoop, "expose Loop::*;", "expose Plant::Pump;", 1), map[string]string{"other.sysml": other}), "Plant::loopView")
	document, exported = drawioExport(t, rendering, options)
	def := drawioCellOf(t, exported, "part def", "Plant::Pump")
	_, imp = drawioImport(t, rendering, options, drawioRelabel(t, document, def.ID, func(label string) string { return strings.Replace(label, "Pump", "Motor", 1) }))
	m = loadEditableWorkspace(t, "model.sysml", strings.Replace(drawioLoop, "expose Loop::*;", "expose Plant::Pump;", 1), map[string]string{"other.sysml": other})
	res, err = Apply(m, DrawioOperations(imp, false))
	if err != nil {
		t.Fatal(err)
	}
	if content := string(res.Content); !strings.Contains(content, "part def Motor {") || !strings.Contains(content, "part pump : Motor;") {
		t.Errorf("renamed model:\n%s", content)
	}
	if len(res.Others) != 1 || res.Others[0].Name != "other.sysml" || !strings.Contains(string(res.Others[0].Content), "part p : Plant::Motor;") {
		t.Errorf("other documents: %+v", res.Others)
	}
}

func TestDrawioImportReportsARenameTheEditorRefuses(t *testing.T) {
	m, rendering, options := drawioModel(t, drawioLoop, "Plant::loopView")
	document, exported := drawioExport(t, rendering, options)
	pump := drawioCellOf(t, exported, "part", "Plant::Loop::pump")
	edited := drawioRelabel(t, document, pump.ID, func(label string) string { return strings.Replace(label, "pump", "tank", 1) })
	edited = drawioMutate(t, edited, pump.ID, func(cell string) string { return drawioShift(cell, "x", 77) })
	_, imp := drawioImport(t, rendering, options, edited)
	if _, err := Apply(m, DrawioOperations(imp, false)); err == nil {
		t.Fatal("renaming pump to tank applied")
	}
	applied := 0
	res, err := ImportDrawio(imp, false, func(ops []Operation) (*Result, error) {
		applied++
		return Apply(m, ops)
	})
	if err != nil || res == nil || applied != 2 {
		t.Fatalf("result %v, error %v after %d applications; want the Layout applied once the rename was dropped", res, err, applied)
	}
	if len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], `Plant::Loop::pump: not renamed to "tank": `) || !strings.Contains(imp.Notices[0], "tank") {
		t.Errorf("notices %q", imp.Notices)
	}
	for _, e := range imp.Edits {
		if e.Kind == view.DrawioEditRename {
			t.Errorf("the refused rename is still among the edits: %+v", e)
		}
	}
	if content := string(res.Content); !strings.Contains(content, "part pump : Pump;") || !strings.Contains(content, "Layout about Plant::Loop::pump") {
		t.Errorf("model after the import:\n%s", content)
	}
}

func TestDrawioImportRenamesAndMovesOneElement(t *testing.T) {
	m, rendering, options := drawioModel(t, drawioLoop, "Plant::loopView")
	document, exported := drawioExport(t, rendering, options)
	pump := drawioCellOf(t, exported, "part", "Plant::Loop::pump")
	edited := drawioRelabel(t, document, pump.ID, func(label string) string { return label + "X" })
	edited = drawioMutate(t, edited, pump.ID, func(cell string) string { return drawioShift(cell, "x", 77) })
	editedFile, imp := drawioImport(t, rendering, options, edited)
	if n := len(imp.Edits); n < 2 || imp.Edits[n-1].Kind != view.DrawioEditRename || imp.Edits[n-1].NewName != "pumpX" {
		t.Fatalf("edits %+v, want Layouts then the Rename", imp.Edits)
	}
	res, err := Apply(m, DrawioOperations(imp, false))
	if err != nil {
		t.Fatal(err)
	}
	content := string(res.Content)
	if !strings.Contains(content, "part pumpX : Pump;") || !strings.Contains(content, "Layout about Plant::Loop::pumpX") || strings.Contains(content, "Loop::pump ") {
		t.Errorf("model after the import:\n%s", content)
	}
	_, redrawn, reoptions := drawioModel(t, content, "Plant::loopView")
	_, re := drawioExport(t, redrawn, reoptions)
	was := editedFile.Cell(pump.ID)
	again := drawioCellOf(t, re, "part", "Plant::Loop::pumpX")
	if again.Label != was.Label || !drawioSameBox(again.Geometry, was.Geometry) {
		t.Errorf("re-exported %q at %+v, edited %q at %+v", again.Label, again.Geometry, was.Label, was.Geometry)
	}
}
