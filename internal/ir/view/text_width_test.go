package view

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// asciiFixtures are one view of each rendering kind, so the ASCII check covers
// every shape the text form draws.
var asciiFixtures = []struct{ file, view string }{
	{"tree.sysml", "VehicleViews::vehicleView"},
	{"interconnection.sysml", "PlantViews::loopView"},
	{"state.sysml", "MachineViews::vehicleStates"},
	{"action.sysml", "FlowViews::driveView"},
	{"table.sysml", "TableViews::partsTable"},
	{"table.sysml", "TableViews::fleetTable"},
}

// TestTextFormIsASCII locks the text form to ASCII, so it reads the same in a
// terminal that draws no more than that.
func TestTextFormIsASCII(t *testing.T) {
	for _, fixture := range asciiFixtures {
		text := render(t, fixture.file, fixture.view).Text()
		for _, r := range text {
			if r >= utf8.RuneSelf {
				t.Errorf("%s: the text form is not ASCII: %q in\n%s", fixture.view, r, text)
				break
			}
		}
	}
}

// TestTextWidthFitsTheTerminal writes a table narrower than its widest cells,
// keeping every cell wrapped over as many lines as it needs.
func TestTextWidthFitsTheTerminal(t *testing.T) {
	const width = 48
	rendering := render(t, "table.sysml", "TableViews::fleetTable")
	text := rendering.TextWidth(width)
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if strings.HasPrefix(line, "TableViews::") {
			// The header names the view rendered; only the table is written to width.
			continue
		}
		if utf8.RuneCountInString(line) > width {
			t.Errorf("a line of a %d-column table is %d wide:\n%s", width, utf8.RuneCountInString(line), text)
		}
	}
	// A wrapped cell is broken over lines, so the cells are looked for in the
	// text with every separator removed.
	flat := strings.Join(strings.Fields(text), "")
	for _, cell := range []string{"TableViews::fleetTable::partsSubtable", "Fleet::Engine", "cylinder", "Cylinder"} {
		if !strings.Contains(flat, cell) {
			t.Errorf("cell %q was lost writing the table to %d columns:\n%s", cell, width, text)
		}
	}
}

// TestTextIsTheUnboundedWidth keeps the unwrapped table what Text writes, so a
// redirected artifact does not depend on the window it was written from.
func TestTextIsTheUnboundedWidth(t *testing.T) {
	rendering := render(t, "table.sysml", "TableViews::fleetTable")
	if text, unbounded := rendering.Text(), rendering.TextWidth(WidthUnbounded); text != unbounded {
		t.Errorf("Text is not the unbounded width:\n%s\n--- unbounded ---\n%s", text, unbounded)
	}
	if strings.Count(rendering.Text(), "\n") >= strings.Count(rendering.TextWidth(48), "\n") {
		t.Errorf("writing the table to 48 columns wrapped nothing:\n%s", rendering.TextWidth(48))
	}
}

// TestWriteWithWidthNarrowsTheTextFormAlone leaves the Markdown table as it is: a
// file a tool reads is no terminal.
func TestWriteWithWidthNarrowsTheTextFormAlone(t *testing.T) {
	rendering := render(t, "table.sysml", "TableViews::fleetTable")
	narrow, err := rendering.WriteWith(FormMarkdown, Options{Width: 20})
	if err != nil {
		t.Fatalf("write Markdown: %v", err)
	}
	if narrow != rendering.Markdown() {
		t.Errorf("a width narrowed the Markdown table:\n%s", narrow)
	}
	text, err := rendering.WriteWith(FormText, Options{Width: 48})
	if err != nil {
		t.Fatalf("write text: %v", err)
	}
	if text != rendering.TextWidth(48) {
		t.Errorf("the text form was not written to the width asked for:\n%s", text)
	}
}

// TestWrapCellKeepsEveryCharacter breaks a cell at its spaces where it has them
// and mid-name where it has none, and drops nothing.
func TestWrapCellKeepsEveryCharacter(t *testing.T) {
	cases := []struct {
		text  string
		width int
		want  []string
	}{
		{"", 8, []string{""}},
		{"short", 8, []string{"short"}},
		{"part def Vehicle", 8, []string{"part def", "Vehicle"}},
		{"TableViews::fleetTable", 8, []string{"TableVie", "ws::flee", "tTable"}},
		{"a name too long", WidthUnbounded, []string{"a name too long"}},
	}
	for _, c := range cases {
		got := wrapCell(c.text, c.width)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("wrapCell(%q, %d) = %q, want %q", c.text, c.width, got, c.want)
		}
	}
}

// TestFitWidthsStopsAtTheMinimum leaves a table wider than a terminal that could
// hold no readable column rather than wrapping every cell a character at a time.
func TestFitWidthsStopsAtTheMinimum(t *testing.T) {
	widths := fitWidths([]int{30, 30, 30}, 10)
	for i, w := range widths {
		if w != minColumnWidth {
			t.Errorf("column %d is %d wide, want the %d-wide minimum: %v", i, w, minColumnWidth, widths)
		}
	}
}
