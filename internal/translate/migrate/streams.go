package migrate

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/imagefile"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// streamSource names the geometry source of a view laid out from its own
// diagram stream; streamsSource the source of a migration laid out from none but those.
const (
	streamSource  = "the diagram's own symbol stream"
	streamsSource = "the diagrams' own symbol streams"
)

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
	for _, s := range d.Symbols {
		switch {
		case s.Hidden || s.Free() || s.Class == "DiagramFrame" || s.ElementID == d.ID || isNoteSymbol(s, m):
		case s.IsPath():
			if len(s.Points) < 2 {
				continue
			}
			c := mtip.Connector{ID: s.ElementID, Type: s.Class}
			for _, p := range s.Points {
				c.Points = append(c.Points, p.X, p.Y)
			}
			rec.Connectors = append(rec.Connectors, c)
		case s.Bounds != nil:
			b := s.Bounds
			rec.Placements = append(rec.Placements, mtip.Placement{
				ID: s.ElementID, Type: s.Class, X: b.X, Y: b.Y, Width: b.Width, Height: b.Height,
			})
		}
	}
	return rec
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
	routed := map[string]bool{}
	for _, c := range export.Connectors {
		routed[c.ID] = true
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
	unread := map[string]*sysmlv1.ImageError{}
	for _, e := range d.ImageErrors {
		unread[e.Symbol] = e
	}
	boxes := elementBoxes(d)
	for i, sym := range d.Symbols {
		if !sym.Free() {
			continue
		}
		e := unread[sym.ID]
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
			p.lost = append(p.lost, named+" is no image (content type "+imagefile.Described(data)+")")
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
func pictureLine(prefix string, p picture) string {
	b := p.sym.Bounds
	var sb strings.Builder
	fmt.Fprintf(&sb, "@%sPicture { location = %s; x = %s; y = %s; width = %s; height = %s;",
		prefix, stringLiteral(p.location), layoutNumber(b.X), layoutNumber(b.Y), layoutNumber(b.Width), layoutNumber(b.Height))
	if alt := pastedAlt(p.sym.Attachment); alt != "" {
		fmt.Fprintf(&sb, " alt = %s;", stringLiteral(alt))
	}
	if p.above {
		sb.WriteString(" above = true;")
	}
	sb.WriteString(" }")
	return sb.String()
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
		clause := plural(n, "pasted image") + " written as " + strings.Join(files, ", ")
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
				over = append(over, plural(covered, "pasted image"))
			}
			clauses = append(clauses, plural(sandwiched, "pasted image")+" drawn under the "+strings.Join(over, " and ")+
				" it lay over, since symbols drawn after it lie over it")
		}
	}
	if n := len(p.lost); n > 0 {
		clauses = append(clauses, plural(n, "pasted image")+" not written: "+strings.Join(p.lost, " and "))
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
func (m *migration) viewDressing(v *view, form viewForm, prefix string, refOf func(string) string) viewDressing {
	d := v.d
	if !d.Drawn {
		return viewDressing{}
	}
	s := m.layoutSummary
	var dress viewDressing
	var styles, styled, notes, anchored, freed int
	dropped := map[string]int{}
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
		anchors[note] = append(anchors[note], target.ElementID)
	}
	pics := m.pastedPictures(d)
	drawn := map[*sysmlv1.Symbol]bool{}
	for _, p := range pics.drawn {
		drawn[p.sym] = true
		dress.lines = append(dress.lines, pictureLine(prefix, p))
	}
	styledRefs := map[string]bool{}
	for _, sym := range d.Symbols {
		switch {
		case sym.Hidden || sym.Class == "DiagramFrame" || sym.ElementID == d.ID || drawn[sym]:
		case isNoteSymbol(sym, m):
			if sym.Bounds == nil {
				continue
			}
			text := sym.Text
			if el := m.model.Lookup(sym.ElementID); el != nil {
				text = commentBody(el)
			}
			if text == "" {
				continue
			}
			notes++
			var refs []string
			for _, id := range anchors[sym] {
				if ref := refOf(id); ref != "" {
					refs = append(refs, ref)
				}
			}
			body := noteBody(text, sym.Bounds)
			if len(refs) == 0 {
				if len(anchors[sym]) > 0 {
					freed++
				}
				dress.lines = append(dress.lines, "@"+prefix+"Note "+body)
				continue
			}
			anchored++
			for _, ref := range refs {
				dress.lines = append(dress.lines, "metadata "+prefix+"Note about "+ref+" "+body)
			}
		case sym.Free():
			if sym.Class != "NoteAnchor" && sym.Parent == nil {
				dropped[sym.Class]++
			}
		default:
			if !styledSymbol(sym) {
				continue
			}
			styles++
			ref := refOf(sym.ElementID)
			if ref == "" || styledRefs[ref] {
				continue
			}
			styledRefs[ref] = true
			styled++
			if sym.Style.NoFill && sym.Style.Fill == "" {
				s.Unsupported["USE_FILL_COLOR"]++
			}
			if line := styleLine(prefix, ref, sym.Style); line != "" {
				dress.lines = append(dress.lines, line)
			}
		}
	}
	dress.pictures, dress.lost = len(pics.drawn), len(pics.lost)
	if form.drawsPictures() {
		dress.underlaid, _, _ = pics.underlaid()
	} else {
		dress.pictures, dress.undrawn = 0, dress.pictures
	}
	s.Pictures += dress.pictures + dress.undrawn + dress.lost
	s.PicturesWritten += dress.pictures
	s.PicturesUnderlaid += dress.underlaid
	s.PicturesUndrawn += dress.undrawn
	s.Styles += styles
	s.StylesWritten += styled
	s.Notes += notes
	s.NotesAnchored += anchored
	s.NotesFreed += freed
	for class, n := range dropped {
		s.Dropped[class] += n
	}
	dress.notes = append(dress.notes, picturesClause(pics, form)...)
	if styles > 0 {
		dress.notes = append(dress.notes, fmt.Sprintf("%d of %d symbols drawn in their own colours or font styled", styled, styles))
	}
	if notes > 0 {
		clause := fmt.Sprintf("%d notes written, %d anchored", notes, anchored)
		if freed > 0 {
			clause += fmt.Sprintf(", %d left free of an anchor the view does not lay out", freed)
		}
		dress.notes = append(dress.notes, clause)
	}
	if len(dropped) > 0 {
		classes := make([]string, 0, len(dropped))
		for class := range dropped {
			classes = append(classes, class)
		}
		sort.Strings(classes)
		var parts []string
		for _, class := range classes {
			parts = append(parts, fmt.Sprintf("%d %s", dropped[class], class))
		}
		dress.notes = append(dress.notes, "free symbols not represented: "+strings.Join(parts, ", "))
	}
	return dress
}

// notesShown reports whether d's stream draws el, a comment with a body, as a note
// the view writes: a bounded Note symbol stands for it.
func (m *migration) notesShown(d *sysmlv1.Diagram, el *sysmlv1.Element) bool {
	if !d.Drawn || el.Type != "Comment" || commentBody(el) == "" {
		return false
	}
	for _, sym := range d.Symbols {
		if sym.ElementID == el.ID && sym.Bounds != nil && !sym.Hidden {
			return true
		}
	}
	return false
}

// isNoteSymbol reports a symbol drawn as a note: a comment's, or a text box saying something.
func isNoteSymbol(sym *sysmlv1.Symbol, m *migration) bool {
	if sym.Free() {
		return sym.Text != "" && !sym.IsPath()
	}
	el := m.model.Lookup(sym.ElementID)
	return el != nil && el.Type == "Comment"
}

// noteBody writes the attribute block of a Note annotation.
func noteBody(text string, b *sysmlv1.Bounds) string {
	return fmt.Sprintf("{ text = %s; x = %s; y = %s; width = %s; height = %s; }",
		stringLiteral(text), layoutNumber(b.X), layoutNumber(b.Y), layoutNumber(b.Width), layoutNumber(b.Height))
}

// styledSymbol reports whether a symbol makes any presentation choice of its own.
func styledSymbol(sym *sysmlv1.Symbol) bool {
	st := sym.Style
	return st.Fill != "" || st.Pen != "" || st.Text != "" || st.Font != nil || st.NoFill
}

// styleLine writes the Style annotation of ref from a symbol's own choices; ""
// when none of them is one DiagramLayout::Style carries.
func styleLine(prefix, ref string, st sysmlv1.Style) string {
	var attrs []string
	if st.Fill != "" {
		attrs = append(attrs, "fill = "+stringLiteral(st.Fill)+";")
	}
	if st.Pen != "" {
		attrs = append(attrs, "line = "+stringLiteral(st.Pen)+";")
	}
	if st.Text != "" {
		attrs = append(attrs, "text = "+stringLiteral(st.Text)+";")
	}
	if f := st.Font; f != nil {
		if f.Name != "" {
			attrs = append(attrs, "font = "+stringLiteral(f.Name)+";")
		}
		if f.Size > 0 {
			attrs = append(attrs, "fontSize = "+layoutNumber(f.Size)+";")
		}
		if f.Bold {
			attrs = append(attrs, "bold = true;")
		}
		if f.Italic {
			attrs = append(attrs, "italic = true;")
		}
	}
	if len(attrs) == 0 {
		return ""
	}
	return "metadata " + prefix + "Style about " + ref + " { " + strings.Join(attrs, " ") + " }"
}
