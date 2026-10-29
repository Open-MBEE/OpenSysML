package edit

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const viewFixtures = "../../ir/view/testdata"

// drawioCase is one draw.io golden of the view package: the model it was
// rendered from and the view, read off the golden's own attributes.
type drawioCase struct{ name, file, view string }

func drawioCases(t *testing.T) []drawioCase {
	t.Helper()
	goldens, err := filepath.Glob(filepath.Join(viewFixtures, "*.drawio.golden"))
	if err != nil || len(goldens) == 0 {
		t.Fatalf("goldens: %v %v", goldens, err)
	}
	var cases []drawioCase
	for _, g := range goldens {
		data, err := os.ReadFile(g)
		if err != nil {
			t.Fatal(err)
		}
		file := regexp.MustCompile(`link="([^":]+):\d+"`).FindStringSubmatch(string(data))
		viewName := regexp.MustCompile(` view="([^"]+)"`).FindStringSubmatch(string(data))
		if file == nil || viewName == nil {
			continue
		}
		cases = append(cases, drawioCase{strings.TrimSuffix(filepath.Base(g), ".drawio.golden"), file[1], viewName[1]})
	}
	return cases
}

// drawioModel loads a view fixture as an editable model and renders view, with
// the export options the CLI and the language server use: the sites naming
// each cell's declaration.
func drawioModel(t *testing.T, content, viewName string) (Model, *view.Rendering, view.Options) {
	t.Helper()
	return drawioModelOf(t, loadContent(t, "model.sysml", content), viewName)
}

// drawioModelOf renders the view of a loaded model as the workspace does.
func drawioModelOf(t *testing.T, m Model, viewName string) (Model, *view.Rendering, view.Options) {
	t.Helper()
	r, sem := m.resolver()
	viewSym, err := m.viewNamed(0, viewName, "", viewName+" is no view")
	if err != nil {
		t.Fatalf("view %s: %v", viewName, err)
	}
	renderer := view.NewRenderer(sem, r, nil)
	rendering, err := renderer.Render(viewSym)
	if err != nil {
		t.Fatalf("render %s: %v", viewName, err)
	}
	lines := func(string) *source.LineIndex { return m.Source.Lines() }
	return m, rendering, view.Options{Links: view.Links{Sites: renderer.Sites(view.FileLocator(sem, lines))}}
}

// drawioExport writes the rendering as the drawio form does, read back as cells.
func drawioExport(t *testing.T, rendering *view.Rendering, options view.Options) (string, *view.DrawioFile) {
	t.Helper()
	document, err := rendering.DrawioWith(options)
	if err != nil {
		t.Fatal(err)
	}
	file, err := view.ReadDrawio([]byte(document), rendering.View)
	if err != nil {
		t.Fatal(err)
	}
	return document, file
}

// drawioMutation edits one cell of the exported document.
type drawioMutation struct {
	name string
	// pick is the cell to edit, nil when the fixture offers none; try imports
	// the mutation of a cell, for a pick that depends on what the import makes of it.
	pick func(file *view.DrawioFile, try func(*view.DrawioCell) *view.DrawioImport) *view.DrawioCell
	edit func(cell string) string
	// relabel edits the object's label instead, the text a user retypes in draw.io.
	relabel func(label string) string
	// kind is the edit expected, and check the cell of the re-export against the
	// edited one; a notice instead names what the import reports and leaves alone.
	kind view.DrawioEditKind
	// children asks that the edited cell's children keep their boxes as well.
	children bool
	notice   string
	check    func(t *testing.T, edited, reexported *view.DrawioCell)
}

func drawioFirst(file *view.DrawioFile, want func(c *view.DrawioCell) bool) *view.DrawioCell {
	for i := range file.Cells {
		if want(&file.Cells[i]) {
			return &file.Cells[i]
		}
	}
	return nil
}

// A named vertex a Layout can place: not a port, which is drawn on its owner's border.
func drawioPlaceable(c *view.DrawioCell) bool {
	return c.Vertex && c.FQN != "" && c.Kind != "port" && c.HasGeometry && !c.Collapsed
}

// drawioLeaf picks the first placeable vertex containing no other, so its move
// is its own alone; drawioContainer the first one with children, whose move is
// theirs too, a Layout being absolute.
func drawioLeaf(f *view.DrawioFile, _ func(*view.DrawioCell) *view.DrawioImport) *view.DrawioCell {
	return drawioFirst(f, func(c *view.DrawioCell) bool { return drawioPlaceable(c) && !drawioHasChildren(f, c) })
}

func drawioContainer(f *view.DrawioFile, _ func(*view.DrawioCell) *view.DrawioImport) *view.DrawioCell {
	return drawioFirst(f, func(c *view.DrawioCell) bool { return drawioPlaceable(c) && drawioHasChildren(f, c) })
}

func drawioHasChildren(f *view.DrawioFile, c *view.DrawioCell) bool {
	return drawioFirst(f, func(d *view.DrawioCell) bool { return d.Parent == c.ID && d.Vertex && !d.Relative }) != nil
}

func drawioSameBox(a, b view.DrawioBox) bool {
	near := func(x, y float64) bool { return x-y < view.DrawioTolerance && y-x < view.DrawioTolerance }
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Width, b.Width) && near(a.Height, b.Height)
}

// drawioEdge picks the first semantic edge cell whose re-routing the import
// takes as a Route, or the first it reports instead: the writer's edge ids are
// an order over the edges it draws, so which is which is the import's to say.
func drawioEdge(steered bool) func(f *view.DrawioFile, try func(*view.DrawioCell) *view.DrawioImport) *view.DrawioCell {
	return func(f *view.DrawioFile, try func(*view.DrawioCell) *view.DrawioImport) *view.DrawioCell {
		return drawioFirst(f, func(c *view.DrawioCell) bool {
			if !c.Edge || c.Kind == "" || c.Kind == "note" {
				return false
			}
			imp := try(c)
			return (len(imp.Edits) == 1 && imp.Edits[0].Kind == view.DrawioEditRoute) == steered
		})
	}
}

// drawioSameCell is the cell of the re-export standing for cell of the edited
// file, ids being an order renumbered by every change: a vertex of its kind and
// name under the same parent; an edge of its kind between the same ends or, once
// a routed member folds into a row and its edge reaches the member's definition,
// declared by the same line of source, by rank among those.
func drawioSameCell(edited *view.DrawioFile, content string, cell *view.DrawioCell, re *view.DrawioFile, recontent string) *view.DrawioCell {
	fqnOf := func(f *view.DrawioFile, id string) string {
		if c := f.Cell(id); c != nil {
			return c.FQN
		}
		return ""
	}
	if !cell.Edge {
		return drawioFirst(re, func(c *view.DrawioCell) bool {
			return !c.Edge && c.Kind == cell.Kind && c.FQN == cell.FQN && fqnOf(re, c.Parent) == fqnOf(edited, cell.Parent)
		})
	}
	declared := drawioLinkLine(content, cell.Link)
	same := func(f *view.DrawioFile, c *view.DrawioCell, source string) bool {
		if !c.Edge || c.Kind != cell.Kind {
			return false
		}
		ends := fqnOf(f, c.Source) == fqnOf(edited, cell.Source) && fqnOf(f, c.Target) == fqnOf(edited, cell.Target)
		return ends || drawioLinkLine(source, c.Link) == declared
	}
	rank := 0
	for i := range edited.Cells {
		if c := &edited.Cells[i]; c == cell {
			break
		} else if same(edited, c, content) {
			rank++
		}
	}
	return drawioFirst(re, func(c *view.DrawioCell) bool {
		if !same(re, c, recontent) {
			return false
		}
		rank--
		return rank < 0
	})
}

// drawioLinkLine is the declaration a cell's link points at: its line of
// source without the `;` or `{` ending it, which an inline edit rewrites.
func drawioLinkLine(content, link string) string {
	_, number, ok := strings.Cut(link, ":")
	n, err := strconv.Atoi(number)
	lines := strings.Split(content, "\n")
	if !ok || err != nil || n < 1 || n > len(lines) {
		return link
	}
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(lines[n-1]), ";{"))
}

// drawioReroute bends the edge through two waypoints of its own.
func drawioReroute(cell string) string {
	const points = `<Array as="points"><mxPoint x="33" y="44"/><mxPoint x="55" y="44"/></Array>`
	if re := regexp.MustCompile(`(?s)<Array as="points">.*?</Array>`); re.MatchString(cell) {
		return re.ReplaceAllString(cell, points)
	}
	return strings.Replace(cell, `as="geometry"/>`, `as="geometry">`+points+`</mxGeometry>`, 1)
}

var drawioMutations = []drawioMutation{
	{name: "rename", pick: drawioRenameable, kind: view.DrawioEditRename,
		relabel: func(label string) string { return label + "X" },
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if edited.Label != re.Label {
				t.Errorf("relabelled %q, re-exported as %q", edited.Label, re.Label)
			}
		}},
	{name: "note text", pick: drawioNote, kind: view.DrawioEditNote,
		edit: func(cell string) string { return drawioSetValue(cell, "Checked weekly&#xa;by hand") },
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if re.Value != edited.Value || edited.Value != "Checked weekly\nby hand" {
				t.Errorf("reworded to %q, re-exported as %q", edited.Value, re.Value)
			}
		}},
	{name: "note emptied", pick: drawioNote, notice: "was emptied",
		edit: func(cell string) string { return drawioSetValue(cell, "") }},
	{name: "move", pick: drawioLeaf,
		edit: func(cell string) string { return drawioShift(cell, "x", 77) }, kind: view.DrawioEditLayout,
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if !drawioSameBox(edited.Geometry, re.Geometry) {
				t.Errorf("moved to %+v, re-exported at %+v", edited.Geometry, re.Geometry)
			}
		}},
	{name: "move container", pick: drawioContainer, children: true,
		edit: func(cell string) string { return drawioShift(cell, "y", 60) }, kind: view.DrawioEditLayout,
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if !drawioSameBox(edited.Geometry, re.Geometry) {
				t.Errorf("%s moved to %+v, re-exported at %+v", edited.ID, edited.Geometry, re.Geometry)
			}
		}},
	{name: "resize container", pick: drawioContainer, children: true,
		edit: func(cell string) string { return drawioShift(cell, "width", 50) }, kind: view.DrawioEditLayout,
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if !drawioSameBox(edited.Geometry, re.Geometry) {
				t.Errorf("%s resized to %+v, re-exported at %+v", edited.ID, edited.Geometry, re.Geometry)
			}
		}},
	{name: "resize", pick: drawioLeaf,
		edit: func(cell string) string { return drawioShift(cell, "width", 50) }, kind: view.DrawioEditLayout,
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if !drawioSameBox(edited.Geometry, re.Geometry) {
				t.Errorf("resized to %+v, re-exported at %+v", edited.Geometry, re.Geometry)
			}
		}},
	{name: "route", pick: drawioEdge(true), edit: drawioReroute, kind: view.DrawioEditRoute,
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if len(re.Points) != 2 || re.Points[0] != edited.Points[0] || re.Points[1] != edited.Points[1] {
				t.Errorf("routed through %+v, re-exported through %+v", edited.Points, re.Points)
			}
		}},
	{name: "route without a Route home", pick: drawioEdge(false), edit: drawioReroute,
		notice: "no Route steers"},
	{name: "recolour", pick: drawioLeaf,
		edit: func(cell string) string {
			if re := regexp.MustCompile(`fillColor=[^;"]*;?`); re.MatchString(cell) {
				return re.ReplaceAllString(cell, "fillColor=#FFCC00;")
			}
			return strings.Replace(cell, `style="`, `style="fillColor=#FFCC00;`, 1)
		}, kind: view.DrawioEditStyle,
		check: func(t *testing.T, edited, re *view.DrawioCell) {
			if re.Style["fillColor"] != "#FFCC00" {
				t.Errorf("re-exported style = %s", re.StyleText)
			}
		}},
}

// drawioNote is the first note box the file draws.
func drawioNote(f *view.DrawioFile, _ func(*view.DrawioCell) *view.DrawioImport) *view.DrawioCell {
	return drawioFirst(f, func(c *view.DrawioCell) bool { return !c.Edge && c.FQN == "" && strings.HasPrefix(c.ID, "note") })
}

// drawioSetValue sets the cell's value attribute, XML-escaped as given.
func drawioSetValue(cell, value string) string {
	return regexp.MustCompile(`value="[^"]*"`).ReplaceAllLiteralString(cell, `value="`+value+`"`)
}

// drawioShift adds delta to one attribute of the cell's geometry.
func drawioShift(cell, attr string, delta float64) string {
	m := regexp.MustCompile(`<mxGeometry\b[^>]*\b` + attr + `="([^"]*)"`).FindStringSubmatchIndex(cell)
	if m == nil {
		return strings.Replace(cell, "<mxGeometry", `<mxGeometry `+attr+`="`+strconv.FormatFloat(delta, 'f', -1, 64)+`"`, 1)
	}
	v, _ := strconv.ParseFloat(cell[m[2]:m[3]], 64)
	return cell[:m[2]] + strconv.FormatFloat(v+delta, 'f', -1, 64) + cell[m[3]:]
}

// drawioMutate rewrites the <mxCell> of the cell id in the document.
// drawioRenameable is the first cell whose label, with text appended, imports
// as a rename: one whose label ends with the element's own name. The view and
// its owners are left alone, as the round trip renders the view by name.
func drawioRenameable(f *view.DrawioFile, try func(*view.DrawioCell) *view.DrawioImport) *view.DrawioCell {
	return drawioFirst(f, func(c *view.DrawioCell) bool {
		if c.FQN == "" || c.Label == "" || f.View == c.FQN || strings.HasPrefix(f.View, c.FQN+"::") {
			return false
		}
		imp := try(c)
		return len(imp.Edits) == 1 && imp.Edits[0].Kind == view.DrawioEditRename && len(imp.Notices) == 0
	})
}

// drawioRelabel rewrites the label attribute of the object cell id, the one
// attribute draw.io rewrites when a user edits the text of a cell.
func drawioRelabel(t *testing.T, document, id string, relabel func(label string) string) string {
	t.Helper()
	re := regexp.MustCompile(`<object id="` + regexp.QuoteMeta(id) + `" label="([^"]*)"`)
	m := re.FindStringSubmatchIndex(document)
	if m == nil {
		t.Fatalf("no object %s", id)
	}
	label := html.UnescapeString(document[m[2]:m[3]])
	edited := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(relabel(label), "&", "&amp;"), "<", "&lt;"), "\n", "&#xA;")
	edited = strings.ReplaceAll(edited, `"`, "&quot;")
	return document[:m[2]] + edited + document[m[3]:]
}

// drawioRenamed is the cell as the re-export of the renamed model draws it:
// the last segment of its qualified name is the new name.
func drawioRenamed(cell *view.DrawioCell, newName string) *view.DrawioCell {
	renamed := *cell
	if i := strings.LastIndex(cell.FQN, "::"); i >= 0 {
		renamed.FQN = cell.FQN[:i+2] + newName
	} else {
		renamed.FQN = newName
	}
	return &renamed
}

func drawioMutate(t *testing.T, document, id string, edit func(cell string) string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<object id="` + regexp.QuoteMeta(id) + `"[^>]*>\s*(<mxCell.*?</mxCell>)\s*</object>|<mxCell id="` + regexp.QuoteMeta(id) + `".*?(?:</mxCell>|/>)`)
	m := re.FindStringSubmatch(document)
	if m == nil {
		t.Fatalf("no cell %s", id)
	}
	whole, inner := m[0], m[1]
	if inner == "" {
		inner = whole
	}
	edited := strings.Replace(whole, inner, edit(inner), 1)
	if edited == whole {
		t.Fatalf("mutation of %s changed nothing", id)
	}
	return strings.Replace(document, whole, edited, 1)
}

func drawioImport(t *testing.T, rendering *view.Rendering, options view.Options, document string) (*view.DrawioFile, *view.DrawioImport) {
	t.Helper()
	file, err := view.ReadDrawio([]byte(document), rendering.View)
	if err != nil {
		t.Fatal(err)
	}
	imp, err := rendering.ImportDrawio(file, options)
	if err != nil {
		t.Fatal(err)
	}
	return file, imp
}

// TestDrawioRoundTrip is the import's contract over every draw.io golden: the
// untouched export writes nothing; each mutation imports as exactly its edit,
// applies through the model-edit path, and the re-export of the edited model
// draws the mutated cell as the file had it, after which the file imports clean.
func TestDrawioRoundTrip(t *testing.T) {
	cases := drawioCases(t)
	if len(cases) < 30 {
		t.Fatalf("%d golden cases found, want the draw.io goldens", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(viewFixtures, c.file))
			if err != nil {
				t.Fatal(err)
			}
			m, rendering, options := drawioModel(t, string(content), c.view)
			document, exported := drawioExport(t, rendering, options)
			if _, imp := drawioImport(t, rendering, options, document); len(imp.Edits)+len(imp.Unmatched)+len(imp.Deleted)+len(imp.Unknown)+len(imp.Notices) != 0 {
				t.Fatalf("untouched export: %+v", imp)
			}
			mutate := func(mut drawioMutation, id string) string {
				if mut.relabel != nil {
					return drawioRelabel(t, document, id, mut.relabel)
				}
				return drawioMutate(t, document, id, mut.edit)
			}
			try := func(mut drawioMutation) func(c *view.DrawioCell) *view.DrawioImport {
				return func(c *view.DrawioCell) *view.DrawioImport {
					_, imp := drawioImport(t, rendering, options, mutate(mut, c.ID))
					return imp
				}
			}
			for _, mut := range drawioMutations {
				cell := mut.pick(exported, try(mut))
				if cell == nil {
					continue
				}
				t.Run(mut.name, func(t *testing.T) {
					edited := mutate(mut, cell.ID)
					editedFile, imp := drawioImport(t, rendering, options, edited)
					if mut.notice != "" {
						if len(imp.Edits) != 0 || len(imp.Notices) != 1 || !strings.Contains(imp.Notices[0], mut.notice) {
							t.Fatalf("edits %+v notices %q, want none and one saying %q", imp.Edits, imp.Notices, mut.notice)
						}
						return
					}
					// A moved container moves what it contains, and a node moved in a
					// container the model does not place pins that container and its
					// placed siblings: a Layout mutation may import as several Layouts.
					if len(imp.Edits) == 0 || len(imp.Edits) != 1 && mut.kind != view.DrawioEditLayout {
						t.Fatalf("edits = %+v, want one %s of %s", imp.Edits, mut.kind, cell.ID)
					}
					picked := false
					for _, e := range imp.Edits {
						picked = picked || e.Cell == cell.ID
						if e.Kind != mut.kind {
							t.Fatalf("edit %+v, want a %s", e, mut.kind)
						}
					}
					if !picked {
						t.Fatalf("edits %+v, none of %s", imp.Edits, cell.ID)
					}
					res, err := Apply(m, DrawioOperations(imp, false))
					if err != nil {
						t.Fatalf("apply %+v: %v", imp.Edits, err)
					}
					_, redrawn, reoptions := drawioModel(t, string(res.Content), c.view)
					_, re := drawioExport(t, redrawn, reoptions)
					for _, e := range imp.Edits {
						was := editedFile.Cell(e.Cell)
						if e.Kind == view.DrawioEditRename {
							was = drawioRenamed(was, e.NewName)
						}
						again := drawioSameCell(editedFile, string(content), was, re, string(res.Content))
						if again == nil {
							t.Fatalf("the re-export has no cell for %s, %s %s", e.Cell, was.Kind, was.FQN)
						}
						mut.check(t, was, again)
					}
					if mut.children {
						for i := range editedFile.Cells {
							child := &editedFile.Cells[i]
							if child.Parent != cell.ID || !drawioPlaceable(child) || child.Relative {
								continue
							}
							again := drawioSameCell(editedFile, string(content), child, re, string(res.Content))
							if again == nil || !drawioSameBox(child.Geometry, again.Geometry) {
								t.Errorf("child %s %s was at %+v, re-exported at %+v", child.ID, child.FQN, child.Geometry, again)
							}
						}
					}
					// The edited cells are where the file has them; unplaced neighbours may have
					// flowed around them, and a renamed node is sized to its longer name.
					if _, imp := drawioImport(t, redrawn, reoptions, edited); len(imp.Edits) != 0 {
						for _, e := range imp.Edits {
							if e.Cell == cell.ID && (mut.kind != view.DrawioEditRename || e.Kind == view.DrawioEditRename) {
								t.Errorf("the edited file still imports %+v against the edited model", e)
							}
						}
					}
				})
			}
		})
	}
}
