package migrate

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// streamSource names the geometry source of a view laid out from its own
// diagram stream; streamsSource the source of a migration laid out from none but those.
const (
	streamSource  = "the diagram's own symbol stream"
	streamsSource = "the diagrams' own symbol streams"
)

const pastedImage = "pasted image"

// layoutSourceName words where v's geometry came from for its layout note.
func (m *migration) layoutSourceName(src layoutSources) string {
	switch {
	case src.export && src.stream:
		return m.layoutSource + " supplemented by " + streamSource
	case src.export:
		return m.layoutSource
	}
	return streamSource
}

// drawsAny reports whether any diagram's symbol stream was read.
func drawsAny(model *sysmlv1.Model) bool {
	for i := range model.Diagrams {
		if model.Diagrams[i].Drawn {
			return true
		}
	}
	return false
}

// streamRecord reads d's own symbol stream as a layout record: a placement per
// shape standing for an element, a connector per path; nil when d has no stream.
// A comment's note is dressing, not a placement; a hidden symbol is not drawn.
func (m *migration) streamRecord(d *sysmlv1.Diagram) *mtip.Diagram {
	if !d.Drawn {
		return nil
	}
	rec := &mtip.Diagram{ID: d.ID, Name: d.Name, Type: d.Kind}
	elementAt := map[string]string{}
	for _, s := range d.Symbols {
		elementAt[s.ID] = s.ElementID
	}
	for _, s := range d.Symbols {
		switch {
		case s.Hidden || s.Free() || s.Class == "DiagramFrame" || s.ElementID == d.ID || isNoteSymbol(s, m):
		case s.IsPath():
			if len(s.Points) < 2 {
				continue
			}
			c := mtip.Connector{ID: s.ElementID, Type: s.Class, Ends: [2]string{elementAt[s.Ends[0]], elementAt[s.Ends[1]]}}
			for _, p := range s.Points {
				c.Points = append(c.Points, p.X, p.Y)
			}
			rec.Connectors = append(rec.Connectors, c)
		case s.Bounds != nil:
			b := s.Bounds
			rec.Placements = append(rec.Placements, mtip.Placement{
				ID: s.ElementID, Type: s.Class, X: b.X, Y: b.Y, Width: b.Width, Height: b.Height, OnPath: onPath(s),
			})
		}
	}
	return rec
}

// onPath reports a shape nested in a path symbol: the tool keeps a line's end
// symbols there (an association's Role, a connector's ConnectorEnd), so the
// shape marks where the line ends and stands for no node of its own.
func onPath(s *sysmlv1.Symbol) bool {
	for p := s.Parent; p != nil; p = p.Parent {
		if p.IsPath() {
			return true
		}
	}
	return false
}

// layoutSources names where a view's geometry came from.
type layoutSources struct {
	export bool // the export's record covers the diagram
	stream bool // the diagram's own stream placed or routed something
	frame  bool // the diagram's own stream sizes the canvas by its frame
}

// layoutRecord is the layout record laying out v: the export's record for v's
// diagram, with the diagram's own stream supplying every element the record
// does not place or route; the stream alone when the export has no record.
func (m *migration) layoutRecord(v *view) (*mtip.Diagram, layoutSources) {
	stream := m.streamRecord(v.d)
	var export *mtip.Diagram
	if m.layout != nil {
		export = m.layoutByID[v.d.ID]
	}
	frame := stream != nil && v.d.Frame != nil
	switch {
	case export == nil:
		return stream, layoutSources{stream: stream != nil, frame: frame}
	case stream == nil:
		return export, layoutSources{export: true}
	}
	merged := *export
	merged.Placements = slices.Clone(export.Placements)
	merged.Connectors = slices.Clone(export.Connectors)
	placed := map[string]bool{}
	for _, p := range export.Placements {
		placed[p.ID] = true
	}
	// The export says nothing about nesting, so an end symbol the stream shows
	// only on a path keeps lying on the line when the export's bounds win.
	onPath := map[string]bool{}
	for _, p := range stream.Placements {
		if v, ok := onPath[p.ID]; !ok || v {
			onPath[p.ID] = p.OnPath
		}
	}
	for i := range merged.Placements {
		if onPath[merged.Placements[i].ID] {
			merged.Placements[i].OnPath = true
		}
	}
	routed := map[string]bool{}
	for _, c := range export.Connectors {
		routed[c.ID] = true
	}
	// The export says nothing about which element is at which end of a line either.
	ends := map[string][2]string{}
	for _, c := range stream.Connectors {
		ends[c.ID] = c.Ends
	}
	for i := range merged.Connectors {
		if c := &merged.Connectors[i]; c.Ends == [2]string{} {
			c.Ends = ends[c.ID]
		}
	}
	src := layoutSources{export: true, frame: frame}
	for _, p := range stream.Placements {
		if !placed[p.ID] {
			merged.Placements = append(merged.Placements, p)
			src.stream = true
		}
	}
	for _, c := range stream.Connectors {
		if !routed[c.ID] {
			merged.Connectors = append(merged.Connectors, c)
			src.stream = true
		}
	}
	return &merged, src
}

// picture is a pasted image the view draws: its symbol, its file, whether it lies over the element
// symbols it overlaps, and how many element symbols and pictures it lay over yet is drawn under.
type picture struct {
	sym      *sysmlv1.Symbol
	location string
	above    bool
	under    int
	covered  int
}

// pictures is what became of the pasted images of one diagram's stream: those
// drawn, in stream order, and why each of the rest is not.
type pictures struct {
	drawn []picture
	lost  []string
}

// pastedPictures writes d's pasted images (inline bytes, else the archive entry
// the file name finds) under images/ at their bounds; the rest are lost with a reason.
func (m *migration) pastedPictures(d *sysmlv1.Diagram) *pictures {
	if p, ok := m.pictureOf[d]; ok {
		return p
	}
	p := &pictures{}
	m.pictureOf[d] = p
	if !d.Drawn {
		return p
	}
	boxes := elementBoxes(d)
	for i, sym := range d.Symbols {
		if !sym.Free() {
			continue
		}
		e := sym.ImageError
		if len(sym.Image) == 0 && sym.Attachment == "" && e == nil {
			continue
		}
		named := "the pasted image"
		if sym.Attachment != "" {
			named += " " + strconv.Quote(sym.Attachment)
		} else if sym.ID != "" {
			named += " of symbol " + sym.ID
		}
		data, ct := sym.Image, sym.ImageType()
		var entry string
		switch {
		case sym.Hidden:
			p.lost = append(p.lost, named+" is hidden on the diagram")
			continue
		case e != nil:
			p.lost = append(p.lost, named+" has bytes that do not read ("+e.Reason()+")")
			continue
		case len(data) > 0 && ct == "":
			p.lost = append(p.lost, named+" is no image (content type "+describedImageContentType(data)+")")
			continue
		case len(data) == 0:
			var reason string
			if data, entry, ct, reason = m.archivedImage(named, sym.Attachment, nil); reason != "" {
				p.lost = append(p.lost, reason)
				continue
			}
		}
		if sym.Bounds == nil {
			p.lost = append(p.lost, named+" has no geometry to place it by")
			continue
		}
		if sym.Bounds.Width <= 0 || sym.Bounds.Height <= 0 {
			p.lost = append(p.lost, fmt.Sprintf("%s has no area to fill (%s by %s)", named, layoutNumber(sym.Bounds.Width), layoutNumber(sym.Bounds.Height)))
			continue
		}
		fallback := sym.ID
		if entry != "" {
			fallback = entry
		}
		before, after := overlapping(sym.Bounds, boxes, i)
		pic := picture{sym: sym, location: m.addFile(imagefile.Name(sym.Attachment, fallback, ct), data)}
		if after == 0 {
			pic.above = before > 0 || p.overAbove(sym.Bounds) > 0
		} else {
			pic.under, pic.covered = before, p.overAbove(sym.Bounds)
		}
		p.drawn = append(p.drawn, pic)
	}
	return p
}

// overAbove counts the pictures drawn so far that lie over the element symbols and share
// area with b: a later picture over them stays over those, one under them is covered by them.
func (p *pictures) overAbove(b *sysmlv1.Bounds) int {
	n := 0
	for _, pic := range p.drawn {
		if pic.above && overlaps(b, pic.sym.Bounds) {
			n++
		}
	}
	return n
}

// elementBox is the bounds of an element symbol and its place in the stream.
type elementBox struct {
	index  int
	bounds *sysmlv1.Bounds
}

// elementBoxes is the box of every element symbol d draws, in stream order.
func elementBoxes(d *sysmlv1.Diagram) []elementBox {
	var boxes []elementBox
	for i, sym := range d.Symbols {
		if !sym.Free() && !sym.Hidden && sym.Bounds != nil && sym.Class != "DiagramFrame" && sym.ElementID != d.ID {
			boxes = append(boxes, elementBox{index: i, bounds: sym.Bounds})
		}
	}
	return boxes
}

// overlapping counts the boxes sharing area with b among the symbols before
// and after the one at index in the stream: the tool draws later symbols on top.
func overlapping(b *sysmlv1.Bounds, boxes []elementBox, index int) (before, after int) {
	for _, box := range boxes {
		if !overlaps(b, box.bounds) {
			continue
		}
		if box.index < index {
			before++
		} else {
			after++
		}
	}
	return before, after
}

// overlaps reports whether two boxes share area.
func overlaps(b, o *sysmlv1.Bounds) bool {
	return b.X < o.X+o.Width && o.X < b.X+b.Width && b.Y < o.Y+o.Height && o.Y < b.Y+b.Height
}

// underlaid counts the drawn pictures that lay between element symbols in the tool, and
// the element symbols and pictures they lay over that they are drawn under.
func (p *pictures) underlaid() (sandwiched, under, covered int) {
	for _, pic := range p.drawn {
		if pic.under > 0 || pic.covered > 0 {
			sandwiched++
			under += pic.under
			covered += pic.covered
		}
	}
	return sandwiched, under, covered
}

// pictureLine writes p's Picture annotation: file, bounds, the pasted file's
// name as alt text, and above when it lies over an element symbol.
func (m *migration) pictureLine(host *sysmlv1.Element, prefix string, p picture, x exposures) string {
	b := p.sym.Bounds
	attrs := []string{
		m.diagramLayoutAttributeForHost(prefix, "Picture", "location", stringLiteral(p.location), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Picture", "x", layoutNumber(b.X), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Picture", "y", layoutNumber(b.Y), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Picture", "width", layoutNumber(b.Width), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Picture", "height", layoutNumber(b.Height), host, x),
	}
	if alt := pastedAlt(p.sym.Attachment); alt != "" {
		attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Picture", "alt", stringLiteral(alt), host, x))
	}
	if p.above {
		attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Picture", "above", "true", host, x))
	}
	return "@" + prefix + "Picture { " + strings.Join(attrs, "; ") + "; }"
}

// pastedAlt is the pasted file's name without its directories and extension.
func pastedAlt(name string) string {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	if alt := strings.TrimSuffix(base, path.Ext(base)); alt != "" && alt != "." && alt != "/" {
		return alt
	}
	return ""
}

// picturesClause words for the layout note what became of a diagram's pasted images:
// the files written, that the view's form does not draw them when so, and why the rest are not.
func picturesClause(p *pictures, form viewForm) []string {
	var clauses []string
	if n := len(p.drawn); n > 0 {
		files := make([]string, 0, n)
		for _, pic := range p.drawn {
			if !slices.Contains(files, pic.location) {
				files = append(files, pic.location)
			}
		}
		clause := plural(n, pastedImage) + " written as " + strings.Join(files, ", ")
		if !form.drawsPictures() {
			clause += ", which a view rendered " + form.rendering + " does not draw"
		}
		clauses = append(clauses, clause)
		if sandwiched, under, covered := p.underlaid(); sandwiched > 0 && form.drawsPictures() {
			var over []string
			if under > 0 {
				over = append(over, plural(under, "element symbol"))
			}
			if covered > 0 {
				over = append(over, plural(covered, pastedImage))
			}
			clauses = append(clauses, plural(sandwiched, pastedImage)+" drawn under the "+strings.Join(over, " and ")+
				" it lay over, since symbols drawn after it lie over it")
		}
	}
	if n := len(p.lost); n > 0 {
		clauses = append(clauses, plural(n, pastedImage)+" not written: "+strings.Join(p.lost, " and "))
	}
	return clauses
}

// viewDressing is what a stream adds to its view beyond geometry: the Picture, Style and
// Note lines, their report clauses, and the pasted images drawn, underlaid, undrawn and lost.
type viewDressing struct {
	lines     []string
	notes     []string
	pictures  int
	underlaid int
	undrawn   int
	lost      int
}

// viewDressing plans the Picture of each pasted image written, the Style of each symbol drawn
// in its own look (refOf naming its element), the Note of each text box, and counts the free rest.
func (m *migration) viewDressing(v *view, form viewForm, prefix string, x exposures, refOf func(string) string) viewDressing {
	d := v.d
	if !d.Drawn {
		return viewDressing{}
	}
	r := &dresser{m: m, v: v, prefix: prefix, x: x, refOf: refOf,
		dropped: map[string]int{}, anchors: noteAnchors(d, m), styledRefs: map[string]bool{}}
	pics := m.pastedPictures(d)
	drawn := map[*sysmlv1.Symbol]bool{}
	for _, p := range pics.drawn {
		drawn[p.sym] = true
		r.dress.lines = append(r.dress.lines, m.pictureLine(v.host, prefix, p, x))
	}
	for _, sym := range d.Symbols {
		switch {
		case sym.Hidden || sym.Class == "DiagramFrame" || sym.ElementID == d.ID || drawn[sym]:
		case isNoteSymbol(sym, m):
			r.note(sym)
		case sym.Free():
			if sym.Class != "NoteAnchor" && sym.Parent == nil {
				r.dropped[sym.Class]++
			}
		default:
			r.style(sym)
		}
	}
	r.dress.pictures, r.dress.lost = len(pics.drawn), len(pics.lost)
	if form.drawsPictures() {
		r.dress.underlaid, _, _ = pics.underlaid()
	} else {
		r.dress.pictures, r.dress.undrawn = 0, r.dress.pictures
	}
	r.summarize(m.layoutSummary)
	r.dress.notes = append(r.dress.notes, picturesClause(pics, form)...)
	r.dress.notes = append(r.dress.notes, r.clauses()...)
	return r.dress
}

// dresser accumulates a view's dressing and the counts its notes and the
// layout summary report.
type dresser struct {
	m          *migration
	v          *view
	prefix     string
	x          exposures
	refOf      func(string) string
	dress      viewDressing
	styles     int
	styled     int
	notes      int
	anchored   int
	freed      int
	dropped    map[string]int
	anchors    map[*sysmlv1.Symbol][]string
	styledRefs map[string]bool
}

// noteAnchors is, per note symbol, the elements the NoteAnchor paths tie it to.
func noteAnchors(d *sysmlv1.Diagram, m *migration) map[*sysmlv1.Symbol][]string {
	symbolOf := map[string]*sysmlv1.Symbol{}
	for _, sym := range d.Symbols {
		if sym.ID != "" {
			symbolOf[sym.ID] = sym
		}
	}
	anchors := map[*sysmlv1.Symbol][]string{}
	for _, sym := range d.Symbols {
		if sym.Class != "NoteAnchor" || !sym.IsPath() {
			continue
		}
		note, target := symbolOf[sym.Ends[0]], symbolOf[sym.Ends[1]]
		if note == nil || target == nil {
			continue
		}
		if !isNoteSymbol(note, m) && isNoteSymbol(target, m) {
			note, target = target, note
		}
		if target.ElementID != "" && !slices.Contains(anchors[note], target.ElementID) {
			anchors[note] = append(anchors[note], target.ElementID)
		}
	}
	return anchors
}

// note writes a bounded note symbol with text, anchored to the elements it refers to.
func (r *dresser) note(sym *sysmlv1.Symbol) {
	if sym.Bounds == nil {
		return
	}
	text := noteText(sym, r.m)
	if text == "" {
		return
	}
	r.notes++
	var refs []string
	for _, id := range r.anchors[sym] {
		if ref := r.refOf(id); ref != "" && !slices.Contains(refs, ref) {
			refs = append(refs, ref)
		}
	}
	body := r.m.noteBody(r.v.host, r.prefix, r.x, text, sym.Bounds)
	if len(refs) == 0 {
		if len(r.anchors[sym]) > 0 {
			r.freed++
		}
		r.dress.lines = append(r.dress.lines, "@"+r.prefix+"Note "+body)
		return
	}
	r.anchored++
	r.dress.lines = append(r.dress.lines, "metadata "+r.prefix+"Note about "+strings.Join(refs, ", ")+" "+body)
}

// style writes the Style of a symbol drawn in its own look, once per element.
func (r *dresser) style(sym *sysmlv1.Symbol) {
	if !styledSymbol(sym) {
		return
	}
	r.styles++
	ref := r.refOf(sym.ElementID)
	if ref == "" || r.styledRefs[ref] {
		return
	}
	r.styledRefs[ref] = true
	r.styled++
	if sym.Style.NoFill && sym.Style.Fill == "" {
		r.m.layoutSummary.Unsupported["USE_FILL_COLOR"]++
	}
	if line := r.m.styleLine(r.v.host, r.prefix, ref, sym.Style, r.x); line != "" {
		r.dress.lines = append(r.dress.lines, line)
	}
}

func (r *dresser) summarize(s *LayoutSummary) {
	s.Pictures += r.dress.pictures + r.dress.undrawn + r.dress.lost
	s.PicturesWritten += r.dress.pictures
	s.PicturesUnderlaid += r.dress.underlaid
	s.PicturesUndrawn += r.dress.undrawn
	s.Styles += r.styles
	s.StylesWritten += r.styled
	s.Notes += r.notes
	s.NotesAnchored += r.anchored
	s.NotesFreed += r.freed
	for class, n := range r.dropped {
		s.Dropped[class] += n
	}
}

// clauses reports the styles, notes and free symbols the view dressed or left.
func (r *dresser) clauses() []string {
	var notes []string
	if r.styles > 0 {
		notes = append(notes, fmt.Sprintf("%d of %d symbols drawn in their own colours or font styled", r.styled, r.styles))
	}
	if r.notes > 0 {
		clause := fmt.Sprintf("%d notes written, %d anchored", r.notes, r.anchored)
		if r.freed > 0 {
			clause += fmt.Sprintf(", %d left free of an anchor the view does not lay out", r.freed)
		}
		notes = append(notes, clause)
	}
	if len(r.dropped) > 0 {
		classes := make([]string, 0, len(r.dropped))
		for class := range r.dropped {
			classes = append(classes, class)
		}
		sort.Strings(classes)
		var parts []string
		for _, class := range classes {
			parts = append(parts, fmt.Sprintf("%d %s", r.dropped[class], class))
		}
		notes = append(notes, "free symbols not represented: "+strings.Join(parts, ", "))
	}
	return notes
}

// notesShown reports whether d's stream draws el, a comment with a body, as a note
// the view writes: a bounded Note symbol stands for it.
func (m *migration) notesShown(d *sysmlv1.Diagram, el *sysmlv1.Element) bool {
	if !d.Drawn || el.Type != "Comment" || m.commentBody(el) == "" {
		return false
	}
	for _, sym := range d.Symbols {
		if sym.ElementID == el.ID && sym.Bounds != nil && !sym.Hidden {
			return true
		}
	}
	return false
}

// isNoteSymbol reports a symbol drawn as a note: a Note symbol whatever it names, a
// comment's symbol, or a text box saying something of its own rather than labelling
// the element symbol it is drawn inside.
func isNoteSymbol(sym *sysmlv1.Symbol, m *migration) bool {
	if sym.Class == "Note" {
		return true
	}
	if sym.Free() {
		return sym.Text != "" && !sym.IsPath() && !labelsParent(sym)
	}
	el := m.model.Lookup(sym.ElementID)
	return el != nil && el.Type == "Comment"
}

// labelsParent reports a free text symbol nested in an element's symbol: the tool
// keeps a symbol's name, stereotype and multiplicity labels there, so the text is the
// element's own and the view draws it with the element, not as a note.
func labelsParent(sym *sysmlv1.Symbol) bool {
	p := sym.Parent
	return p != nil && !p.Free() && p.Class != "DiagramFrame"
}

// noteText is what a note symbol says: its comment's body when it stands for a
// comment, else the text the tool wrote on the symbol itself.
func noteText(sym *sysmlv1.Symbol, m *migration) string {
	if el := m.model.Lookup(sym.ElementID); el != nil && el.Type == "Comment" {
		return m.commentBody(el)
	}
	return sym.Text
}

// noteBody writes the attribute block of a Note annotation.
func (m *migration) noteBody(host *sysmlv1.Element, prefix string, x exposures, text string, b *sysmlv1.Bounds) string {
	attrs := []string{
		m.diagramLayoutAttributeForHost(prefix, "Note", "text", stringLiteral(text), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Note", "x", layoutNumber(b.X), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Note", "y", layoutNumber(b.Y), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Note", "width", layoutNumber(b.Width), host, x),
		m.diagramLayoutAttributeForHost(prefix, "Note", "height", layoutNumber(b.Height), host, x),
	}
	return "{ " + strings.Join(attrs, "; ") + "; }"
}

// styledSymbol reports whether a symbol makes any presentation choice of its own.
func styledSymbol(sym *sysmlv1.Symbol) bool {
	st := sym.Style
	return st.Fill != "" || st.Pen != "" || st.Text != "" || st.Font != nil || st.NoFill
}

// styleLine writes the Style annotation of ref from a symbol's own choices; ""
// when none of them is one DiagramLayout::Style carries.
func (m *migration) styleLine(host *sysmlv1.Element, prefix, ref string, st sysmlv1.Style, x exposures) string {
	var attrs []string
	if st.Fill != "" {
		attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Style", "fill", stringLiteral(st.Fill), host, x)+";")
	}
	if st.Pen != "" {
		attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Style", "line", stringLiteral(st.Pen), host, x)+";")
	}
	if st.Text != "" {
		attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Style", "text", stringLiteral(st.Text), host, x)+";")
	}
	if f := st.Font; f != nil {
		if f.Name != "" {
			attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Style", "font", stringLiteral(f.Name), host, x)+";")
		}
		if f.Size > 0 {
			attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Style", "fontSize", layoutNumber(f.Size), host, x)+";")
		}
		if f.Bold {
			attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Style", "bold", "true", host, x)+";")
		}
		if f.Italic {
			attrs = append(attrs, m.diagramLayoutAttributeForHost(prefix, "Style", "italic", "true", host, x)+";")
		}
	}
	if len(attrs) == 0 {
		return ""
	}
	return "metadata " + prefix + "Style about " + ref + " { " + strings.Join(attrs, " ") + " }"
}
