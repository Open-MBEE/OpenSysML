package view

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
	"html"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
)

// DrawioTolerance is how far a coordinate may move in draw.io and still count
// as unchanged: half a pixel, below what the editor's grid lets a hand place.
const DrawioTolerance = 0.5

// DrawioEditKind is which DiagramLayout annotation a DrawioEdit writes.
type DrawioEditKind int

const (
	DrawioEditLayout DrawioEditKind = iota
	DrawioEditRoute
	DrawioEditStyle
	DrawioEditCanvas
	DrawioEditNote
	DrawioEditPicture
	DrawioEditRename
)

// String names the annotation the kind writes.
func (k DrawioEditKind) String() string {
	switch k {
	case DrawioEditLayout:
		return "Layout"
	case DrawioEditRoute:
		return "Route"
	case DrawioEditStyle:
		return "Style"
	case DrawioEditCanvas:
		return "Canvas"
	case DrawioEditNote:
		return "Note"
	case DrawioEditPicture:
		return "Picture"
	case DrawioEditRename:
		return "Rename"
	}
	return "Layout"
}

// DrawioEdit is one change read back from an edited draw.io file: the
// annotation to write for an element, located by its qualified name and
// declaration, with the value the file states. Exactly one of Layout, Route,
// Style, Canvas, Note and Picture applies to Kind; a nil Route or Style clears.
type DrawioEdit struct {
	Kind DrawioEditKind
	// Cell is the id of the file cell the edit was read from, Label its label.
	Cell, Label string
	// FQN is the element's qualified name as the writer stamped it, "" for an
	// edge or a note; Origin is where the element — or the annotation, for a
	// Note or Picture — was declared.
	FQN    string
	Origin Origin
	// Edge is set for an edit on an edge: the ends it joins and its kind.
	Edge *DrawioEdgeKey
	// Index is a note's or picture's position in the rendering's list.
	Index   int
	Layout  *semantics.Layout
	Route   *semantics.Route
	Style   *semantics.Style
	Canvas  *semantics.Canvas
	Note    *semantics.Note
	Picture *semantics.Picture
	// NewName is the name a Rename gives the element.
	NewName string
}

// DrawioEdgeKey identifies an edge across exports: the qualified names of its
// ends, its kind, and its ordinal among the edges sharing them in file order.
type DrawioEdgeKey struct {
	From, To string
	Kind     string
	Ordinal  int
}

func (k DrawioEdgeKey) String() string {
	s := fmt.Sprintf("%s %s → %s", k.Kind, k.From, k.To)
	if k.Ordinal > 0 {
		s += fmt.Sprintf(" (%d)", k.Ordinal+1)
	}
	return s
}

// DrawioImport is what an edited draw.io file says about a rendering: the edits
// that make the model state what the file draws, and what the file holds that
// no edit carries.
type DrawioImport struct {
	View string
	// Edits are in file order, one per annotation changed.
	Edits []DrawioEdit
	// Unmatched are semantic cells of the file matching no element of the
	// rendering: elements gone or renamed since the export.
	Unmatched []string
	// Unknown are cells the file holds that the export did not write and that
	// carry no element: shapes drawn in draw.io, which no edit creates.
	Unknown []DrawioCell
	// Deleted are elements of the rendering whose cells the file no longer
	// holds; nothing deletes them from the model.
	Deleted []string
	// Notices say what else the file changed that has no annotation to land in.
	Notices []string
}

// ImportDrawio reads an edited draw.io file of this rendering back as edits:
// the file the writer would emit now under options is the baseline, and only
// cells that differ from it are imported, so the writer's own geometry — grids,
// fitted sizes, computed routes — never becomes an annotation.
func (r *Rendering) ImportDrawio(file *DrawioFile, options Options) (*DrawioImport, error) {
	norm, w, err := r.drawioPrepare(options)
	if err != nil {
		return nil, err
	}
	if file.View != "" && file.View != norm.View {
		return nil, &DrawioViewError{View: norm.View, Pages: []string{file.View}}
	}
	pages, err := drawioPages([]byte(w.document(norm, options)))
	if err != nil {
		return nil, err
	}
	base, err := readDrawioModel(pages[0].model)
	if err != nil {
		return nil, err
	}
	out := &DrawioImport{View: norm.View}
	m := &drawioMatcher{norm: norm, drawn: r.drawn, w: w, base: base, file: file, out: out, nodes: map[string]*Node{}, ports: map[string]Port{}, framed: map[string]bool{}, paired: map[string]*DrawioCell{}}
	for _, node := range drawioNodes(norm.Roots) {
		m.nodes[node.ID] = node
		for _, port := range node.Ports {
			m.ports[node.ID+":"+port.ID] = port
		}
	}
	m.match()
	m.canvas()
	m.resolveRenames()
	return out, nil
}

// drawioNodes lists every node of the trees rooted at roots, parents first.
func drawioNodes(roots []*Node) []*Node {
	var out []*Node
	for _, node := range roots {
		out = append(out, node)
		out = append(out, drawioNodes(node.Children)...)
	}
	return out
}

// drawioMatcher pairs the cells of the edited file with the baseline's and
// turns each difference into an edit.
type drawioMatcher struct {
	renames []drawioRename
	norm    *Rendering
	// drawn is the rendering's own record of what it drew; its normalization
	// for the writer, a clone, carries none.
	drawn      *Drawn
	w          *drawioWriter
	base, file *DrawioFile
	out        *DrawioImport
	nodes      map[string]*Node
	ports      map[string]Port
	matched    map[string]bool
	// framed are the cells whose Layout pinning a container already wrote;
	// paired is the file's cell for each baseline cell matched so far.
	framed map[string]bool
	paired map[string]*DrawioCell
}

// match pairs the file's semantic cells with the baseline's — by the exported
// id while it still denotes the same element, then by qualified name, or by ends
// and kind for an edge, among those left, in order —, notes and pictures by
// position, and diffs each pair.
func (m *drawioMatcher) match() {
	m.matched = map[string]bool{}
	byFQN := map[string][]*DrawioCell{}
	byKey := map[DrawioEdgeKey][]*DrawioCell{}
	baseKeys, fileKeys := m.edgesByKey(m.base), m.edgesByKey(m.file)
	for i := range m.base.Cells {
		c := &m.base.Cells[i]
		switch {
		case c.Vertex && c.FQN != "":
			byFQN[c.FQN] = append(byFQN[c.FQN], c)
		case c.Edge && c.Kind != "":
			if key, ok := baseKeys[c.ID]; ok {
				byKey[key.unordered()] = append(byKey[key.unordered()], c)
			}
		}
	}
	var pending []*DrawioCell
	baseOf := map[*DrawioCell]*DrawioCell{}
	pair := func(cell, base *DrawioCell) {
		m.matched[base.ID] = true
		m.paired[base.ID] = cell
		baseOf[cell] = base
	}
	for i := range m.file.Cells {
		cell := &m.file.Cells[i]
		if cell.Kind == "" {
			continue
		}
		if base := m.base.Cell(cell.ID); base != nil && sameDrawioElement(cell, base, fileKeys, baseKeys) {
			pair(cell, base)
			continue
		}
		pending = append(pending, cell)
	}
	for _, cell := range pending {
		switch {
		case cell.Vertex && cell.FQN != "":
			if base := m.firstUnmatched(byFQN[cell.FQN]); base != nil {
				pair(cell, base)
			} else {
				m.out.Unmatched = append(m.out.Unmatched, cell.FQN)
			}
		case cell.Vertex:
			m.out.Unmatched = append(m.out.Unmatched, cell.Kind+" "+strconv.Quote(cell.Label))
		default:
			key, ok := fileKeys[cell.ID]
			if !ok {
				m.out.Unmatched = append(m.out.Unmatched, m.edgeLabel(cell, m.file))
				continue
			}
			if base := m.firstUnmatched(byKey[key.unordered()]); base != nil {
				pair(cell, base)
			} else {
				m.out.Unmatched = append(m.out.Unmatched, key.String())
			}
		}
	}
	// Every pair is known before any is diffed, so pinning a container reads
	// each child's edited cell whatever id the file gives it; the edits come
	// in the file's order.
	for i := range m.file.Cells {
		cell := &m.file.Cells[i]
		if cell.Kind == "" {
			m.plainCell(cell)
		} else if base := baseOf[cell]; base != nil {
			m.diff(cell, base, baseKeys)
		}
	}
	m.deleted()
}

// sameDrawioElement reports whether the file's cell and the baseline's of the
// same id still stand for one element: the same qualified name and kind — the
// same kind and label for a node no name reaches —, or the same ends and kind.
func sameDrawioElement(cell, base *DrawioCell, fileKeys, baseKeys map[string]DrawioEdgeKey) bool {
	if cell.Vertex != base.Vertex || cell.Edge != base.Edge || cell.Kind != base.Kind {
		return false
	}
	if cell.Vertex {
		return cell.FQN == base.FQN && (cell.FQN != "" || cell.Label == base.Label)
	}
	fileKey, ok := fileKeys[cell.ID]
	baseKey, okBase := baseKeys[base.ID]
	return ok && okBase && fileKey.unordered() == baseKey.unordered()
}

// firstUnmatched is the first baseline cell of a group not yet paired, nil
// when every one is.
func (m *drawioMatcher) firstUnmatched(cells []*DrawioCell) *DrawioCell {
	for _, c := range cells {
		if !m.matched[c.ID] {
			return c
		}
	}
	return nil
}

// diff compares the file's cell with the baseline's it stands for.
func (m *drawioMatcher) diff(cell, base *DrawioCell, baseKeys map[string]DrawioEdgeKey) {
	if cell.Vertex {
		m.diffVertex(cell, base)
		return
	}
	m.diffEdge(cell, base, baseKeys[base.ID])
}

// unordered is the key without its ordinal: what every edge between the same
// ends of one kind shares.
func (k DrawioEdgeKey) unordered() DrawioEdgeKey {
	k.Ordinal = 0
	return k
}

// edgesByKey keys every semantic edge of f by the names of its ends, its kind
// and its ordinal among those sharing them, in document order; an edge whose
// end is no cell of the file is left out.
func (m *drawioMatcher) edgesByKey(f *DrawioFile) map[string]DrawioEdgeKey {
	keys := map[string]DrawioEdgeKey{}
	ordinals := map[DrawioEdgeKey]int{}
	for i := range f.Cells {
		cell := &f.Cells[i]
		if !cell.Edge || cell.Kind == "" {
			continue
		}
		from, to := f.Cell(cell.Source), f.Cell(cell.Target)
		if from == nil || to == nil {
			continue
		}
		key := DrawioEdgeKey{From: drawioEndName(from), To: drawioEndName(to), Kind: cell.Kind}
		key.Ordinal = ordinals[key]
		ordinals[key]++
		keys[cell.ID] = key
	}
	return keys
}

// drawioEndName names an edge's end across exports: the element's qualified
// name, or for a cell without one — a pseudostate, an objective — its label.
func drawioEndName(cell *DrawioCell) string {
	if cell.FQN != "" {
		return cell.FQN
	}
	return strconv.Quote(cell.Label)
}

// edgeLabel describes an edge the matcher cannot key.
func (m *drawioMatcher) edgeLabel(cell *DrawioCell, f *DrawioFile) string {
	name := func(id string) string {
		if c := f.Cell(id); c != nil {
			return drawioEndName(c)
		}
		return "cell " + strconv.Quote(id)
	}
	return fmt.Sprintf("%s %s → %s", cell.Kind, name(cell.Source), name(cell.Target))
}

// diffVertex imports a moved, resized, folded or restyled node; a port's
// position is drawn on its owner's border, so only its style is read.
func (m *drawioMatcher) diffVertex(cell, base *DrawioCell) {
	origin, isPort := m.originOf(base.ID)
	edit := DrawioEdit{Cell: cell.ID, Label: cell.Label, FQN: m.targetName(cell, base), Origin: origin}
	if isPort {
		owner, size := m.base.Cell(base.Parent), DrawioBox{}
		if owner != nil {
			size = owner.Geometry
		}
		switch fileOwner := m.file.Cell(cell.Parent); {
		case !sameOwner(fileOwner, owner):
			m.out.Notices = append(m.out.Notices, fmt.Sprintf("%s: a port was moved to another cell; it stays on its owner, so the move is not imported", cell.FQN))
		case !sameBox(portPlace(cell, fileOwner, size), portPlace(base, owner, size)):
			m.out.Notices = append(m.out.Notices, fmt.Sprintf("%s: a port is drawn on its owner's border; its position in the file is not imported", cell.FQN))
		}
	} else if layout := m.layoutDiff(cell, base); layout != nil {
		edit.Kind, edit.Layout = DrawioEditLayout, layout
		m.out.Edits = append(m.out.Edits, edit)
	}
	if cell.Label != base.Label {
		m.diffLabel(cell, base, edit)
	}
	var current *Style
	if node := m.nodes[base.ID]; node != nil {
		current = node.Style
	}
	style, changed := styleDiff(cell.Style, base.Style, current, drawioVertexStyleKeys)
	if !changed {
		return
	}
	if isPort {
		m.out.Notices = append(m.out.Notices, fmt.Sprintf("%s: a port is drawn by its direction; its colours in the file are not imported", cell.FQN))
		return
	}
	edit.Kind, edit.Style, edit.Layout = DrawioEditStyle, style, nil
	m.out.Edits = append(m.out.Edits, edit)
}

// diffLabel imports an edited label as a rename when the only change is the
// declared name's token, replaced by one SysML name; any other change, and a
// cell whose label shows no name of its own, are reported and left alone.
func (m *drawioMatcher) diffLabel(cell, base *DrawioCell, edit DrawioEdit) {
	label := drawioLabelText(cell)
	if label == base.Label {
		return
	}
	cellName := edit.FQN
	if cellName == "" {
		cellName = cell.Kind + " " + strconv.Quote(base.Label)
	}
	name, ok := m.w.names[base.ID]
	if !ok || edit.FQN == "" {
		m.out.Notices = append(m.out.Notices, fmt.Sprintf("%s: its label was edited, but the cell shows no declared name of its own (its name is synthesized or the label carries other text), so nothing is renamed and the label is not imported", cellName))
		return
	}
	head, tail := base.Label[:name.Offset], base.Label[name.Offset+name.Len:]
	beyond := fmt.Sprintf("%s: its label changed beyond the name %q; a keyword, type, multiplicity, parameter or compartment edit is not imported", cellName, base.Label[name.Offset:name.Offset+name.Len])
	if len(label) < len(head)+len(tail) || !strings.HasPrefix(label, head) || !strings.HasSuffix(label, tail) {
		m.out.Notices = append(m.out.Notices, beyond)
		return
	}
	newName := label[len(head) : len(label)-len(tail)]
	if problem := lexer.NameProblem(newName); problem == lexer.NotASingleName {
		m.out.Notices = append(m.out.Notices, beyond)
		return
	} else if problem != "" {
		m.out.Notices = append(m.out.Notices, fmt.Sprintf("%s: renamed to %q, which %s, so the rename is not imported", cellName, newName, problem))
		return
	}
	edit.Kind, edit.Layout, edit.Style, edit.Route, edit.NewName, edit.Label = DrawioEditRename, nil, nil, nil, newName, label
	m.renames = append(m.renames, drawioRename{edit: edit, base: base.ID})
}

// drawioRename is a rename read from one cell, held back until every copy of
// the element the view draws has been read.
type drawioRename struct {
	edit DrawioEdit
	base string
}

// resolveRenames appends the renames after every other edit — the edit batch
// applies in order, so the Layout, Route and Style edits are written against
// the names the file still has and the rename then moves them too — keeping
// one rename per element; copies of an element the view draws twice that
// disagree are reported and none of them is imported.
func (m *drawioMatcher) resolveRenames() {
	byCell, seen := map[string]drawioRename{}, map[string]bool{}
	for _, r := range m.renames {
		byCell[r.base] = r
	}
	for _, r := range m.renames {
		if declared := m.base.Cell(r.base).FQN; seen[declared] {
			continue
		} else {
			seen[declared] = true
		}
		if other, differs := m.copyDisagrees(r, byCell); differs {
			m.out.Notices = append(m.out.Notices, fmt.Sprintf("%s: the view draws it twice and the copies disagree (%q and %q), so no rename is imported", r.edit.FQN, r.edit.NewName, other))
			continue
		}
		m.out.Edits = append(m.out.Edits, r.edit)
	}
}

// copyDisagrees finds another drawn copy of a renamed element whose new name
// differs — a copy left as it was still shows the old name — and names it.
func (m *drawioMatcher) copyDisagrees(r drawioRename, byCell map[string]drawioRename) (string, bool) {
	base := m.base.Cell(r.base)
	for i := range m.base.Cells {
		other := &m.base.Cells[i]
		if other.ID == base.ID || other.FQN != base.FQN || other.Kind != base.Kind || m.paired[other.ID] == nil {
			continue
		}
		if copied, ok := byCell[other.ID]; ok {
			if copied.edit.NewName != r.edit.NewName {
				return copied.edit.NewName, true
			}
			continue
		}
		if name, ok := m.w.names[other.ID]; ok {
			return other.Label[name.Offset : name.Offset+name.Len], true
		}
		return other.Label, true
	}
	return "", false
}

// drawioLabelText is a cell's label as plain text: the label as written when
// the cell is plain text, as the writer leaves every cell; the HTML draw.io
// writes once a user turns formatted text on decoded back to the lines and
// characters the writer emitted.
func drawioLabelText(cell *DrawioCell) string {
	return drawioText(cell.Label, cell.Style)
}

// drawioText is the plain text of a cell's text, decoded when its style formats it.
func drawioText(text string, style map[string]string) string {
	if style["html"] != "1" {
		return text
	}
	return drawioHTMLText(text)
}

// drawioHTMLText decodes draw.io's formatted-text HTML to plain text: a
// `<br>` is a line break, a `<div>` or `<p>` opens a new line after the first,
// other tags are dropped, and entities read as the characters they name.
func drawioHTMLText(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '<' {
			b.WriteByte(text[i])
			i++
			continue
		}
		end := strings.IndexByte(text[i:], '>')
		if end < 0 {
			b.WriteString(text[i:])
			break
		}
		tag := strings.ToLower(text[i+1 : i+end])
		name, _, _ := strings.Cut(strings.Trim(tag, "/ "), " ")
		closing := strings.HasPrefix(tag, "/")
		if name == "br" || (name == "div" || name == "p") && !closing && b.Len() > 0 {
			b.WriteByte('\n')
		}
		i += end + 1
	}
	return strings.ReplaceAll(html.UnescapeString(b.String()), "\u00a0", " ")
}

// originOf is the declaration behind a baseline cell id: a port's or a node's.
func (m *drawioMatcher) originOf(id string) (Origin, bool) {
	if port, ok := m.ports[id]; ok {
		return port.Origin, true
	}
	if node := m.nodes[id]; node != nil {
		return node.Origin, false
	}
	return Origin{}, false
}

// layoutDiff is the Layout a node needs for the file's box, nil when the box
// is the baseline's within tolerance and the fold is the same.
func (m *drawioMatcher) layoutDiff(cell, base *DrawioCell) *semantics.Layout {
	if m.framed[base.ID] {
		return nil
	}
	fileBox, baseBox := fullBox(cell), fullBox(base)
	moved := !near(fileBox.X, baseBox.X) || !near(fileBox.Y, baseBox.Y)
	resized := !near(fileBox.Width, baseBox.Width) || !near(fileBox.Height, baseBox.Height)
	folded := cell.Collapsed != base.Collapsed
	if !moved && !resized && !folded {
		return nil
	}
	shift := m.frameShift(base)
	node := m.nodes[base.ID]
	// A container the model does not place is pinned with its children once,
	// not placed here and pinned again when a child drawn in its frame moves.
	if node != nil && !node.Geometry.placed() && m.framesChildren(base.ID) {
		m.pin(base.ID)
	}
	if m.framed[base.ID] {
		return nil
	}
	layout := &semantics.Layout{X: fileBox.X - shift.X, Y: fileBox.Y - shift.Y, Collapsed: cell.Collapsed}
	if resized || (node != nil && node.Geometry != nil && node.Geometry.HasSize) {
		layout.Width, layout.Height, layout.HasSize = fileBox.Width, fileBox.Height, true
	}
	if node != nil && node.Geometry != nil {
		layout.Hidden = node.Geometry.Hidden
	}
	return layout
}

// framesChildren reports whether the writer drew vertex cells inside the
// baseline cell id's frame.
func (m *drawioMatcher) framesChildren(id string) bool {
	for i := range m.base.Cells {
		if child := &m.base.Cells[i]; child.Parent == id && child.Vertex && !child.Relative {
			return true
		}
	}
	return false
}

// frameShift is what the writer added to a node's Layout to draw it on the page:
// nothing for a root or a child of a placed container; for a child of one the
// model does not place, the writer hugs the placed children to the container's
// corner, so the container is pinned first and the page frame becomes the model's.
func (m *drawioMatcher) frameShift(base *DrawioCell) DrawioBox {
	parent := m.nodes[base.Parent]
	if parent == nil {
		return DrawioBox{}
	}
	if !parent.Geometry.placed() {
		m.pin(base.Parent)
		return DrawioBox{}
	}
	shift := m.w.childShift(parent, m.w.boxes[base.Parent])
	return DrawioBox{X: shift.X, Y: shift.Y}
}

// pin places a container the model does not place, and every child drawn in its
// frame, at the box the file shows, a child container pinned in turn.
func (m *drawioMatcher) pin(id string) {
	if m.framed[id] {
		return
	}
	m.framed[id] = true
	container, base := m.nodes[id], m.base.Cell(id)
	if container == nil || base == nil {
		return
	}
	m.framePlace(m.editedOf(base), base, container, true)
	for i := range m.base.Cells {
		child := &m.base.Cells[i]
		node := m.nodes[child.ID]
		if child.Parent != id || !child.Vertex || child.Relative || node == nil || m.framed[child.ID] {
			continue
		}
		if !node.Geometry.placed() && m.framesChildren(child.ID) {
			m.pin(child.ID)
			continue
		}
		m.framed[child.ID] = true
		edited := m.editedOf(child)
		sized := !node.Geometry.placed() || node.Geometry.HasSize ||
			!near(fullBox(edited).Width, fullBox(child).Width) || !near(fullBox(edited).Height, fullBox(child).Height)
		m.framePlace(edited, child, node, sized)
	}
}

// editedOf is the file's cell paired with a baseline cell, the baseline's own
// when the file dropped it.
func (m *drawioMatcher) editedOf(base *DrawioCell) *DrawioCell {
	if cell := m.paired[base.ID]; cell != nil {
		return cell
	}
	return base
}

// framePlace writes a Layout putting node at its page box, as the file has it.
func (m *drawioMatcher) framePlace(cell, base *DrawioCell, node *Node, sized bool) {
	box := fullBox(cell)
	layout := &semantics.Layout{X: box.X - m.frameShift(base).X, Y: box.Y - m.frameShift(base).Y, Collapsed: cell.Collapsed}
	if sized {
		layout.Width, layout.Height, layout.HasSize = box.Width, box.Height, true
	}
	if node.Geometry != nil {
		layout.Hidden = node.Geometry.Hidden
	}
	origin, _ := m.originOf(base.ID)
	m.out.Edits = append(m.out.Edits, DrawioEdit{Kind: DrawioEditLayout, Cell: cell.ID, Label: cell.Label, FQN: m.targetName(cell, base), Origin: origin, Layout: layout})
}

// targetName is the name an annotation about the cell's element states: its
// qualified name, or, for a member the rendering drew as its container's own —
// an inherited state, a part of the usage's type — that container's name and
// its own, the path a Layout about it resolves through (MemberLayoutOf). An
// element a nested view exposes keeps its own name: the view is no owner.
func (m *drawioMatcher) targetName(cell, base *DrawioCell) string {
	parent, name := m.base.Cell(base.Parent), symbols.LastSegment(cell.FQN)
	if parent == nil || parent.FQN == "" || !parent.Vertex || cell.FQN == "" {
		return cell.FQN
	}
	if owner, isPort := m.originOf(parent.ID); !isPort && m.drawn.Inherits(owner, name) {
		return parent.FQN + "::" + name
	}
	return cell.FQN
}

// diffEdge imports an edge's moved waypoints and changed colours or font.
func (m *drawioMatcher) diffEdge(cell, base *DrawioCell, key DrawioEdgeKey) {
	i, err := strconv.Atoi(strings.TrimPrefix(base.ID, "e"))
	if err != nil || i < 0 || i >= len(m.norm.Edges) {
		return
	}
	edge := m.norm.Edges[i]
	out := DrawioEdit{Cell: cell.ID, Label: cell.Label, FQN: m.w.siteName(edge.Origin), Origin: edge.Origin, Edge: &key}
	// A rendering built without a record of what it draws leaves the refusal
	// of an edge no Route steers to the edit applying the import.
	steered := m.drawn == nil || m.drawn.Steers(edge.Origin, edge.Kind)
	if !sameWaypoints(cell.Points, base.Points) && !steered {
		m.out.Notices = append(m.out.Notices, fmt.Sprintf("edge %s re-routed: no Route steers a %s edge yet, so its waypoints are not imported", key.String(), edge.Kind))
	} else if !sameWaypoints(cell.Points, base.Points) {
		out.Kind = DrawioEditRoute
		if len(cell.Points) > 0 {
			out.Route = &semantics.Route{}
			for _, p := range cell.Points {
				out.Route.Points = append(out.Route.Points, semantics.Waypoint{X: p.X, Y: p.Y})
			}
		}
		m.out.Edits = append(m.out.Edits, out)
	}
	if style, changed := styleDiff(cell.Style, base.Style, edge.Style, drawioEdgeStyleKeys); changed {
		out.Kind, out.Route, out.Style = DrawioEditStyle, nil, style
		m.out.Edits = append(m.out.Edits, out)
	}
	if cell.Label != base.Label {
		out.FQN = base.FQN
		m.diffLabel(cell, base, out)
	}
}

// plainCell handles a cell without an element: a note, a picture, an end label
// or a compartment row the export wrote, or a shape drawn in draw.io.
func (m *drawioMatcher) plainCell(cell *DrawioCell) {
	base := m.base.Cell(cell.ID)
	if base == nil || base.Kind != "" {
		m.out.Unknown = append(m.out.Unknown, *cell)
		return
	}
	m.matched[cell.ID] = true
	switch {
	case strings.HasPrefix(cell.ID, "note") && cell.Edge:
		if cell.Source != base.Source || cell.Target != base.Target {
			m.out.Notices = append(m.out.Notices, fmt.Sprintf("note %s re-anchored: a Note is about the element its annotation names; the attachment in the file is not imported", strconv.Quote(m.noteText(cell))))
		}
	case strings.HasPrefix(cell.ID, "note") && cell.Vertex:
		m.diffNote(cell, base)
	case strings.HasPrefix(cell.ID, "picture") && cell.Vertex:
		m.diffPicture(cell, base)
	case cell.Vertex && cell.Relative && base.Relative:
		if !near(cell.Geometry.X, base.Geometry.X) || !near(cell.Geometry.Y, base.Geometry.Y) {
			m.out.Notices = append(m.out.Notices, fmt.Sprintf("label %s moved: a Route places a line, not its label; label positions have no annotation yet and are not imported", strconv.Quote(cell.Value)))
		}
	}
}

// noteText is the text of the note an anchor edge joins.
func (m *drawioMatcher) noteText(anchor *DrawioCell) string {
	if note := m.file.Cell(anchor.Source); note != nil {
		return note.Value
	}
	return anchor.Source
}

// diffNote imports a moved, resized or reworded note box; a note emptied of
// its text is reported, as a Note with no text is not a Note.
func (m *drawioMatcher) diffNote(cell, base *DrawioCell) {
	i, err := strconv.Atoi(strings.TrimPrefix(cell.ID, "note"))
	if err != nil || i < 0 || i >= len(m.norm.Notes) {
		return
	}
	text, box := drawioText(cell.Value, cell.Style), boxOf(cell)
	if sameBox(box, boxOf(base)) && text == base.Value {
		return
	}
	note := m.norm.Notes[i]
	if strings.TrimSpace(text) == "" {
		m.out.Notices = append(m.out.Notices, fmt.Sprintf("note %s (%s) was emptied; a Note with no text is not a Note, so the note is left as it was", cell.ID, strconv.Quote(firstLine(base.Value))))
		return
	}
	m.out.Edits = append(m.out.Edits, DrawioEdit{Kind: DrawioEditNote, Cell: cell.ID, Label: text, Origin: note.Origin, Index: i,
		Note: &semantics.Note{Text: text, X: box.X, Y: box.Y, Width: box.Width, Height: box.Height, HasSize: true}})
}

// diffPicture imports a moved or resized picture, or one whose image was changed
// to a file the model can locate; embedded data or a URL is reported, since no
// Picture location names them, and the picture keeps its file.
func (m *drawioMatcher) diffPicture(cell, base *DrawioCell) {
	i, err := strconv.Atoi(strings.TrimPrefix(cell.ID, "picture"))
	if err != nil || i < 0 || i >= len(m.norm.Pictures) {
		return
	}
	picture, box := m.norm.Pictures[i], boxOf(cell)
	location, changed := picture.Location, cell.Style["image"] != base.Style["image"]
	if changed {
		if replaced, problem := drawioPictureLocation(cell.Style["image"], picture.Dir); problem != "" {
			m.out.Notices = append(m.out.Notices, fmt.Sprintf("picture %s (%s): its image was changed to %s, so the picture keeps its file", cell.ID, picture.Location, problem))
			changed = false
		} else {
			location = replaced
		}
	}
	if !changed && sameBox(box, boxOf(base)) && cell.Value == base.Value {
		return
	}
	m.out.Edits = append(m.out.Edits, DrawioEdit{Kind: DrawioEditPicture, Cell: cell.ID, Label: cell.Value, Origin: picture.Origin, Index: i,
		Picture: &semantics.Picture{Location: location, X: box.X, Y: box.Y, Width: box.Width, Height: box.Height, Alt: cell.Value, Above: picture.Above}})
}

// drawioPictureLocation is the Picture location a draw.io image source names:
// a path, kept relative to the view's directory dir as the Picture states it,
// or a file URL made one. Embedded data, a URL no drawing tool reads, and a
// file that does not read as an image are a problem, worded for a notice.
func drawioPictureLocation(src, dir string) (location, problem string) {
	src = strings.TrimSpace(src)
	switch {
	case src == "":
		return "", "no image"
	case strings.HasPrefix(strings.ToLower(src), "data:"):
		return "", "embedded image data, which no Picture location names (a Picture names a file)"
	case strings.HasPrefix(strings.ToLower(src), "file:"):
		u, err := url.Parse(src)
		if err != nil || u.Path == "" || (u.Host != "" && u.Host != "localhost") {
			return "", fmt.Sprintf("%s, a file URL naming no local file", src)
		}
		location = filepath.FromSlash(u.Path)
	case RemotePictureLocation(src):
		return "", fmt.Sprintf("%s, a URL no drawing tool reads", src)
	default:
		location = src
	}
	if dir != "" && filepath.IsAbs(location) {
		if rel, err := filepath.Rel(dir, location); err == nil && !strings.HasPrefix(rel, "..") {
			location = filepath.ToSlash(rel)
		}
	}
	data, err := os.ReadFile(Picture{Location: location, Dir: dir}.Path()) // #nosec G304 -- the user named the file in the diagram.
	if err != nil {
		return "", fmt.Sprintf("%q, a file that does not read", location)
	}
	if imagefile.ContentType(data) == "" {
		return "", fmt.Sprintf("%q, which is not an image", location)
	}
	if err := CheckPicture(location, data); err != nil {
		return "", fmt.Sprintf("%q, which is not drawn: %v", location, err)
	}
	return location, ""
}

// firstLine is the first line of text, for a notice naming a note.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// canvas imports a changed page size as the view's Canvas.
func (m *drawioMatcher) canvas() {
	if near(m.file.PageWidth, m.base.PageWidth) && near(m.file.PageHeight, m.base.PageHeight) {
		return
	}
	unit := m.file.Unit
	if unit == "" && m.norm.Canvas != nil {
		unit = m.norm.Canvas.Unit
	}
	m.out.Edits = append(m.out.Edits, DrawioEdit{Kind: DrawioEditCanvas, Cell: "page", FQN: m.norm.View,
		Canvas: &semantics.Canvas{Unit: unit, Width: m.file.PageWidth, Height: m.file.PageHeight, HasSize: true}})
}

// deleted lists the baseline's semantic cells the file no longer holds.
func (m *drawioMatcher) deleted() {
	var gone []string
	for i := range m.base.Cells {
		cell := &m.base.Cells[i]
		if m.matched[cell.ID] {
			continue
		}
		switch {
		case cell.FQN != "":
			gone = append(gone, cell.FQN)
		case cell.Edge && cell.Kind != "":
			gone = append(gone, m.edgeLabel(cell, m.base))
		}
	}
	sort.Strings(gone)
	m.out.Deleted = gone
}

// The draw.io style keys a Style annotation states, for a node and for an edge.
var (
	drawioVertexStyleKeys = []string{"fillColor", "strokeColor", "fontColor", "fontFamily", "fontSize", "fontStyle"}
	drawioEdgeStyleKeys   = []string{"strokeColor", "fontColor", "fontFamily", "fontSize", "fontStyle"}
)

// styleDiff is the Style the element needs for the file's colours and font:
// the current one, each key the file changed overriding it. The second result
// is false when nothing changed; a nil Style with true clears the current one.
func styleDiff(file, base map[string]string, current *Style, keys []string) (*semantics.Style, bool) {
	out := semantics.Style{}
	if current != nil {
		out = semantics.Style{Fill: current.Fill, Line: current.Line, Text: current.Text, Font: current.Font, FontSize: current.FontSize, Bold: current.Bold, Italic: current.Italic}
	}
	changed := false
	for _, key := range keys {
		fv, bv := file[key], base[key]
		if fv == bv {
			continue
		}
		changed = true
		switch key {
		case "fillColor":
			out.Fill = drawioColour(fv)
		case "strokeColor":
			out.Line = drawioColour(fv)
		case "fontColor":
			out.Text = drawioColour(fv)
		case "fontFamily":
			out.Font = fv
		case "fontSize":
			out.FontSize, _ = strconv.ParseFloat(fv, 64)
		case "fontStyle":
			bits, _ := strconv.Atoi(fv)
			out.Bold, out.Italic = bits&1 != 0, bits&2 != 0
		}
	}
	if !changed {
		return nil, false
	}
	if out.Empty() {
		return nil, current != nil
	}
	return &out, true
}

// drawioColour is a draw.io colour as a Style states it: `#RRGGBB`, or ""
// for `none`, `default` and anything that is no hexadecimal colour.
func drawioColour(v string) string {
	if len(v) == 7 && v[0] == '#' {
		if _, err := strconv.ParseUint(v[1:], 16, 32); err == nil {
			return strings.ToUpper(v)
		}
	}
	return ""
}

// boxOf is a cell's box; fullBox the box it expands to when collapsed.
func boxOf(cell *DrawioCell) DrawioBox { return cell.Geometry }

// sameOwner tells whether two cells, one of each file, are the same owner: the
// same element when both are semantic, else the same cell.
func sameOwner(a, b *DrawioCell) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.FQN != "" || b.FQN != "" {
		return a.FQN == b.FQN
	}
	return a.ID == b.ID
}

// portPlace is where a port sits on an owner of size: a relative port's fraction
// of it plus its offset, an absolute port's box against its own owner's corner.
func portPlace(cell, owner *DrawioCell, size DrawioBox) DrawioBox {
	box := cell.Geometry
	if cell.Relative {
		box.X = cell.Fraction.X*size.Width + cell.Offset.X
		box.Y = cell.Fraction.Y*size.Height + cell.Offset.Y
	} else if owner != nil && owner.Vertex && owner.HasGeometry {
		box.X -= owner.Geometry.X
		box.Y -= owner.Geometry.Y
	}
	return box
}

func fullBox(cell *DrawioCell) DrawioBox {
	box := cell.Geometry
	if cell.Collapsed && cell.Alternate != nil {
		box.Width, box.Height = cell.Alternate.Width, cell.Alternate.Height
	}
	return box
}

func near(a, b float64) bool { return math.Abs(a-b) <= DrawioTolerance }

func sameBox(a, b DrawioBox) bool {
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Width, b.Width) && near(a.Height, b.Height)
}

func sameWaypoints(a, b []Point) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !near(a[i].X, b[i].X) || !near(a[i].Y, b[i].Y) {
			return false
		}
	}
	return true
}
