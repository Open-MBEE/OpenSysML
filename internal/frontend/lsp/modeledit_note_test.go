package lsp

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A setNote writes a Note about a node into the view's body; the note's own
// range then reaches that one annotation to move or remove it, as a setPicture
// does the Picture it wrote into the view.
func TestApplyModelEditSetNoteAndPicture(t *testing.T) {
	s, docURI := renderServer(t, "plant.sysml", plantModel)
	drawn := render(t, s, docURI, "PlantViews::loopView")
	width, height := 100.0, 30.0
	out := applyModelEdit(t, s, docURI, drawn.Version,
		ModelEditOperation{Kind: EditSetNote, Target: "Plant::Loop::pump", View: drawn.View, Note: &ModelEditNote{Text: "feeds the tank", X: 120, Y: 40, Width: &width, Height: &height}},
		ModelEditOperation{Kind: EditSetPicture, Target: drawn.View, Picture: &ModelEditPicture{Location: "plant.png", X: 0, Y: 0, Width: 400, Height: 300, Alt: "the plant"}},
	)
	const note = `metadata DiagramLayout::Note about Plant::Loop::pump { text = "feeds the tank"; x = 120; y = 40; width = 100; height = 30; }`
	const picture = `@DiagramLayout::Picture { location = "plant.png"; x = 0; y = 0; width = 400; height = 300; alt = "the plant"; }`
	applied, _ := redraw(t, s, docURI, plantModel, out, 2, drawn.View)
	if !strings.Contains(applied, note) || !strings.Contains(applied, picture) {
		t.Fatalf("note and picture not written:\n%s", applied)
	}
	content := []byte(applied)
	noteRange := spanToRange(content, source.Span{Offset: strings.Index(applied, note), Len: len(note)})
	pictureRange := spanToRange(content, source.Span{Offset: strings.Index(applied, picture), Len: len(picture)})
	out = applyModelEdit(t, s, docURI, 2,
		ModelEditOperation{Kind: EditSetNote, Declaration: &noteRange, Note: &ModelEditNote{Text: "feeds the tank", X: 130, Y: 50}},
		ModelEditOperation{Kind: EditSetPicture, Declaration: &pictureRange},
	)
	moved, _ := redraw(t, s, docURI, applied, out, 3, drawn.View)
	if !strings.Contains(moved, `{ text = "feeds the tank"; x = 130; y = 50; }`) || strings.Contains(moved, "Picture") {
		t.Errorf("note not moved or picture not removed:\n%s", moved)
	}

	for _, tc := range []struct {
		name string
		op   ModelEditOperation
		want string
	}{
		{"note sized by half", ModelEditOperation{Kind: EditSetNote, Target: "Plant::Loop::pump", Note: &ModelEditNote{Text: "x", Width: &width}}, "together or not at all"},
		{"picture in a view", ModelEditOperation{Kind: EditSetPicture, Target: drawn.View, View: drawn.View, Picture: &ModelEditPicture{Location: "p.png", Width: 1, Height: 1}}, "names the view as its target"},
		{"declared note in a view", ModelEditOperation{Kind: EditSetNote, Declaration: &noteRange, View: drawn.View}, "names no view"},
	} {
		if _, err := tc.op.operation(content); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}
