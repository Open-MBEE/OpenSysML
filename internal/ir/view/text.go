package view

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Text is the human-readable form of a rendering, written to no particular
// width. It is ASCII throughout, so it reads the same on any terminal; a
// positioned node ends in `at (x, y)` and, when sized, `size W×H`.
func (r *Rendering) Text() string { return r.TextWidth(WidthUnbounded) }

// TextWidth is the human-readable form of a rendering: a header saying what was
// rendered and how, the nodes as an indented tree, the edges beneath it, and
// what the rendering could not represent. It is what the REPL prints. A table's
// columns are written to fit width, wrapping their cells; WidthUnbounded writes
// each column as wide as its widest cell.
func (r *Rendering) TextWidth(width int) string {
	var b strings.Builder
	if r.View == "" {
		fmt.Fprintf(&b, "%s rendering", r.Kind)
	} else {
		fmt.Fprintf(&b, "%s - %s rendering", r.View, r.Kind)
	}
	if r.Stated != "" {
		fmt.Fprintf(&b, " (%s)", r.Stated)
	} else {
		b.WriteString(" (the view states no rendering; a tree is the default)")
	}
	b.WriteString("\n")
	if r.Empty() {
		b.WriteString("\n" + r.EmptyReason() + "\n")
		writeNotices(&b, r.Notices)
		return b.String()
	}
	b.WriteString("\n")
	if r.Kind == KindTable {
		writeTableText(&b, r.Columns, r.Rows, width)
		writeNotices(&b, r.Notices)
		return b.String()
	}
	if c := r.Canvas; c != nil {
		b.WriteString(canvasText(c) + "\n\n")
	}
	labels := map[string]string{}
	for _, root := range r.Roots {
		writeNodeText(&b, root, 0, labels)
	}
	if len(r.Edges) > 0 {
		fmt.Fprintf(&b, "\n%s:\n", edgeSectionName(r.Kind))
		for _, edge := range r.Edges {
			line := fmt.Sprintf("  %s %s %s", labels[edge.From], edgeArrow(edge.Kind), labels[edge.To])
			if edge.Label != "" {
				line += ": " + edge.Label
			}
			if len(edge.Route) > 0 {
				line += " via"
				for _, p := range edge.Route {
					line += fmt.Sprintf(" (%s, %s)", formatCoord(p.X), formatCoord(p.Y))
				}
			}
			b.WriteString(line + "\n")
		}
	}
	if len(r.Notes) > 0 {
		b.WriteString("\nnotes:\n")
		for _, note := range r.Notes {
			b.WriteString(noteText(note, labels) + "\n")
		}
	}
	if len(r.Pictures) > 0 {
		b.WriteString("\npictures:\n")
		for _, picture := range r.Pictures {
			b.WriteString(pictureText(picture) + "\n")
		}
	}
	writeNotices(&b, slices.Concat(r.Notices, r.visualNotices(noStyleInText, false)))
	return b.String()
}

// pictureText is a picture as one line: its file, its corner and size, its
// alternative text, and whether it is drawn over the nodes.
func pictureText(p Picture) string {
	line := fmt.Sprintf("  %s at (%s, %s) size %s×%s", strconv.Quote(p.Location), formatCoord(p.X), formatCoord(p.Y), formatCoord(p.Width), formatCoord(p.Height))
	if p.Alt != "" {
		line += " alt " + strconv.Quote(p.Alt)
	}
	if p.Above {
		line += " above"
	}
	return line
}

// noteText is a note as one line: its text, the node it is anchored on, and its
// corner and size when positioned.
func noteText(note Note, labels map[string]string) string {
	line := "  " + strconv.Quote(note.Text)
	if note.Anchor != "" {
		line += " on " + labelOr(labels, note.Anchor)
	} else if note.EdgeFrom != "" {
		line += " on " + labelOr(labels, note.EdgeFrom) + " -> " + labelOr(labels, note.EdgeTo)
	}
	if note.HasSize {
		line += fmt.Sprintf(" at (%s, %s) size %s×%s", formatCoord(note.X), formatCoord(note.Y), formatCoord(note.Width), formatCoord(note.Height))
	} else if note.X != 0 || note.Y != 0 {
		line += fmt.Sprintf(" at (%s, %s)", formatCoord(note.X), formatCoord(note.Y))
	}
	return line
}

// EmptyReason says why a rendering shows nothing: a view exposing nothing, or a
// view whose exposed elements this kind of rendering cannot show, which the
// notices then account for one by one.
func (r *Rendering) EmptyReason() string {
	if len(r.Notices) > 0 {
		return fmt.Sprintf("the rendering is empty: nothing the view exposes is shown by %s %s rendering",
			r.Kind.article(), r.Kind)
	}
	return "the view exposes nothing; the rendering is empty"
}

// blank reports whether a form that draws no picture shows nothing of the
// rendering: it has no node, edge or row, whether or not it has pictures.
func (r *Rendering) blank() bool {
	return len(r.Roots) == 0 && len(r.Edges) == 0 && len(r.Rows) == 0
}

// blankReason is EmptyReason or, for a rendering of pictures alone, that
// form does not draw them.
func (r *Rendering) blankReason(form Form) string {
	if r.Empty() {
		return r.EmptyReason()
	}
	return fmt.Sprintf("the view shows %d picture(s), which the %s form does not draw", len(r.Pictures), form)
}

// writeNodeText writes one node and its children, and records the label an edge
// names the node by. A body's start is named by the body it starts.
func writeNodeText(b *strings.Builder, node *Node, depth int, labels map[string]string) {
	labels[node.ID] = nodeLabel(node)
	line := strings.Repeat("  ", depth) + node.Kind
	if node.Name != "" {
		line += " " + node.Name
	}
	if node.Type != "" {
		line += " : " + node.Type
	}
	if node.Detail != "" {
		line += " (" + node.Detail + ")"
	}
	if g := node.Geometry; g != nil {
		line += fmt.Sprintf(" at (%s, %s)", formatCoord(g.X), formatCoord(g.Y))
		if g.HasSize {
			line += fmt.Sprintf(" size %s×%s", formatCoord(g.Width), formatCoord(g.Height))
		}
		if g.Collapsed {
			line += " collapsed"
		}
	}
	b.WriteString(line + "\n")
	for _, child := range node.Children {
		writeNodeText(b, child, depth+1, labels)
		if child.Kind == startKind {
			labels[child.ID] = "start of " + labels[node.ID]
		}
	}
}

// writeTableText writes a tabular rendering as aligned columns, padding each
// cell to the widest one in its column and narrowing the columns to fit width.
// A trailing empty cell is left off, so no row ends in padding.
func writeTableText(b *strings.Builder, columns []string, rows [][]string, width int) {
	widths := columnWidths(columns, rows, width)
	writeTableRow(b, columns, widths)
	rule := make([]string, len(columns))
	for i := range rule {
		rule[i] = strings.Repeat("-", widths[i])
	}
	writeTableRow(b, rule, widths)
	for _, row := range rows {
		writeTableRow(b, row, widths)
	}
}

// writeTableRow writes one row, its cells padded to the column widths and
// wrapped over as many lines as a cell too wide for its column needs.
func writeTableRow(b *strings.Builder, cells []string, widths []int) {
	wrapped := make([][]string, len(cells))
	lines := 1
	for i, cell := range cells {
		wrapped[i] = wrapCell(cell, cellWidth(widths, i))
		if len(wrapped[i]) > lines {
			lines = len(wrapped[i])
		}
	}
	for line := 0; line < lines; line++ {
		text := ""
		for i, cell := range wrapped {
			part := ""
			if line < len(cell) {
				part = cell[line]
			}
			pad := cellWidth(widths, i) - utf8.RuneCountInString(part)
			if pad < 0 {
				pad = 0
			}
			text += part + strings.Repeat(" ", pad) + strings.Repeat(" ", columnGap)
		}
		b.WriteString(strings.TrimRight(text, " ") + "\n")
	}
}

// cellWidth is the width column i is written at, and 0 for a cell beyond the
// declared columns, which is written as it stands rather than dropped.
func cellWidth(widths []int, i int) int {
	if i < len(widths) {
		return widths[i]
	}
	return 0
}

// columnWidths are the widths the columns are written at: each as wide as its
// widest cell, narrowed to fit width.
func columnWidths(columns []string, rows [][]string, width int) []int {
	widths := make([]int, len(columns))
	for i, column := range columns {
		widths[i] = utf8.RuneCountInString(column)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && utf8.RuneCountInString(cell) > widths[i] {
				widths[i] = utf8.RuneCountInString(cell)
			}
		}
	}
	return fitWidths(widths, width)
}

// fitWidths narrows the widest column until the row fits width, stopping at
// minColumnWidth: a narrower terminal overflows rather than wrapping to nothing.
func fitWidths(widths []int, width int) []int {
	if width <= WidthUnbounded {
		return widths
	}
	total := columnGap * (len(widths) - 1)
	for _, w := range widths {
		total += w
	}
	for total > width {
		widest := 0
		for i, w := range widths {
			if w > widths[widest] {
				widest = i
			}
		}
		if widths[widest] <= minColumnWidth {
			break
		}
		widths[widest]--
		total--
	}
	return widths
}

// wrapCell breaks a cell over as many lines of width as it needs, at its spaces
// and mid-name where a name is longer than the column, so nothing is truncated.
func wrapCell(text string, width int) []string {
	if width <= 0 || utf8.RuneCountInString(text) <= width {
		return []string{text}
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
		for utf8.RuneCountInString(line) > width {
			runes := []rune(line)
			lines = append(lines, string(runes[:width]))
			line = string(runes[width:])
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// writeNotices writes what the rendering could not represent, so nothing is
// dropped silently.
func writeNotices(b *strings.Builder, notices []string) {
	if len(notices) == 0 {
		return
	}
	b.WriteString("\nnot represented:\n")
	for _, notice := range notices {
		b.WriteString("  - " + notice + "\n")
	}
}

// canvasText states the canvas a view draws on: its size when given, and its unit.
func canvasText(c *Canvas) string {
	line := "canvas"
	if c.HasSize {
		line += fmt.Sprintf(" size %s×%s", formatCoord(c.Width), formatCoord(c.Height))
	}
	if c.Unit != "" {
		line += " in " + c.Unit
	}
	return line
}

// labelOr is the label of the node with the given ID, else the ID itself.
func labelOr(labels map[string]string, id string) string {
	if label := labels[id]; label != "" {
		return label
	}
	return id
}

// nodeLabel names a node where an edge refers to it: its name, else its kind
// with the identity the rendering gave it.
func nodeLabel(node *Node) string {
	if node.Name != "" {
		return node.Name
	}
	return fmt.Sprintf("%s %s", node.Kind, node.ID)
}

// edgeSectionName heads the edge list with what the edges of that kind are.
func edgeSectionName(kind Kind) string {
	switch kind {
	case KindState:
		return "transitions"
	case KindAction:
		return "flow"
	case KindSequence:
		return "messages"
	}
	return "connections"
}

// edgeArrow is how an edge of each kind is drawn in text.
func edgeArrow(kind EdgeKind) string {
	switch kind {
	case EdgeConnection:
		return "--"
	case EdgeBinding:
		return "=="
	case EdgeFlow:
		return "=>"
	}
	return "->"
}
