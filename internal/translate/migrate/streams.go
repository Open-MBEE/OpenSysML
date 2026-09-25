package migrate

import (
	"fmt"
	"slices"
	"sort"
	"strings"

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
// A comment's note is dressing, not a placement.
func (m *migration) streamRecord(d *sysmlv1.Diagram) *mtip.Diagram {
	if !d.Drawn {
		return nil
	}
	rec := &mtip.Diagram{ID: d.ID, Name: d.Name, Type: d.Kind}
	for _, s := range d.Symbols {
		switch {
		case s.Free() || s.Class == "DiagramFrame" || s.ElementID == d.ID || isNoteSymbol(s, m):
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

// viewDressing is what a diagram's stream adds to its view beyond geometry:
// the Style and Note annotation lines and the clauses accounting for them.
type viewDressing struct {
	lines []string
	notes []string
}

// viewDressing plans the Style of every symbol drawn in its own colours or font
// whose element the view draws (refOf naming it, positioned or not), the Note of
// every comment and text box, each anchored to the element its anchor reaches
// when the view draws that and free on the view otherwise, and counts the free
// symbols nothing represents.
func (m *migration) viewDressing(v *view, prefix string, refOf func(string) string) viewDressing {
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
	styledRefs := map[string]bool{}
	for _, sym := range d.Symbols {
		switch {
		case sym.Class == "DiagramFrame" || sym.ElementID == d.ID:
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
	s.Styles += styles
	s.StylesWritten += styled
	s.Notes += notes
	s.NotesAnchored += anchored
	s.NotesFreed += freed
	for class, n := range dropped {
		s.Dropped[class] += n
	}
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
		if sym.ElementID == el.ID && sym.Bounds != nil {
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
