package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// annotationSpan is the span of the first annotation declared with text in content.
func annotationSpan(t *testing.T, content, text string) source.Span {
	t.Helper()
	at := strings.Index(content, text)
	if at < 0 {
		t.Fatalf("no %q in the model", text)
	}
	return source.Span{Offset: at, Len: len(text)}
}

func TestSetNote(t *testing.T) {
	note := &semantics.Note{Text: "feeds the tank", X: 120, Y: 40}
	got := applyLayout(t, plantModel, SetNote("Plant::Loop::pump", "PlantViews::loopView", note))
	requireReplaced(t, plantModel, "        render asInterconnectionDiagram;\n    }\n",
		"        render asInterconnectionDiagram;\n        metadata DiagramLayout::Note about Plant::Loop::pump { text = \"feeds the tank\"; x = 120; y = 40; }\n    }\n", got)
	noted := got
	sized := &semantics.Note{Text: "feeds the tank", X: 130, Y: 50, Width: 100, Height: 30, HasSize: true}
	got = applyLayout(t, noted, SetNote("Plant::Loop::pump", "PlantViews::loopView", sized))
	requireReplaced(t, noted, `{ text = "feeds the tank"; x = 120; y = 40; }`, `{ text = "feeds the tank"; x = 130; y = 50; width = 100; height = 30; }`, got)
	got = applyLayout(t, noted, SetNote("Plant::Loop::pump", "PlantViews::loopView", nil))
	if got != plantModel {
		t.Fatalf("clearing the note did not restore the model:\n%s", got)
	}
	got = applyLayout(t, plantModel, SetNote("Plant::Loop::pump", "", note))
	requireReplaced(t, plantModel, "        part pump : Pump;\n",
		"        part pump : Pump {\n            @DiagramLayout::Note { text = \"feeds the tank\"; x = 120; y = 40; }\n        }\n", got)
}

// A note about nothing in particular is stated inline in the view's own body.
func TestSetNoteOnTheView(t *testing.T) {
	note := &semantics.Note{Text: "draft", X: 10, Y: 10}
	got := applyLayout(t, plantModel, SetNote("PlantViews::loopView", "", note))
	requireReplaced(t, plantModel, "        render asInterconnectionDiagram;\n    }\n",
		"        render asInterconnectionDiagram;\n        @DiagramLayout::Note { text = \"draft\"; x = 10; y = 10; }\n    }\n", got)
	free := got
	got = applyLayout(t, free, SetNote("PlantViews::loopView", "", &semantics.Note{Text: "final", X: 20, Y: 30}))
	requireReplaced(t, free, `{ text = "draft"; x = 10; y = 10; }`, `{ text = "final"; x = 20; y = 30; }`, got)
	got = applyLayout(t, free, SetNote("PlantViews::loopView", "", nil))
	if got != plantModel {
		t.Fatalf("clearing the free note did not restore the model:\n%s", got)
	}
}

// Two notes about one element are told apart by their declarations.
func TestSetNoteAtRewritesTheDeclaredNote(t *testing.T) {
	const first = `metadata DiagramLayout::Note about Plant::Loop::pump { text = "first"; x = 1; y = 2; }`
	const second = `metadata DiagramLayout::Note about Plant::Loop::pump { text = "second"; x = 3; y = 4; }`
	twice := strings.Replace(plantModel, "        render asInterconnectionDiagram;\n",
		"        render asInterconnectionDiagram;\n        "+first+"\n        "+second+"\n", 1)
	got := applyLayout(t, twice, SetNoteAt(annotationSpan(t, twice, second), &semantics.Note{Text: "second", X: 30, Y: 40}))
	requireReplaced(t, twice, second, `metadata DiagramLayout::Note about Plant::Loop::pump { text = "second"; x = 30; y = 40; }`, got)
	got = applyLayout(t, twice, SetNoteAt(annotationSpan(t, twice, first), nil))
	requireReplaced(t, twice, "        "+first+"\n", "", got)
	inline := strings.Replace(plantModel, "        part pump : Pump;\n",
		"        part pump : Pump {\n            @DiagramLayout::Note { text = \"inline\"; x = 5; y = 6; }\n        }\n", 1)
	got = applyLayout(t, inline, SetNoteAt(annotationSpan(t, inline, `@DiagramLayout::Note { text = "inline"; x = 5; y = 6; }`), &semantics.Note{Text: "moved", X: 7, Y: 8}))
	requireReplaced(t, inline, `{ text = "inline"; x = 5; y = 6; }`, `{ text = "moved"; x = 7; y = 8; }`, got)
}

func TestSetPicture(t *testing.T) {
	picture := &semantics.Picture{Location: "plant.png", X: 10, Y: 20, Width: 300, Height: 200, Alt: "the plant"}
	got := applyLayout(t, plantModel, SetPicture("PlantViews::loopView", picture))
	requireReplaced(t, plantModel, "        render asInterconnectionDiagram;\n    }\n",
		"        render asInterconnectionDiagram;\n        @DiagramLayout::Picture { location = \"plant.png\"; x = 10; y = 20; width = 300; height = 200; alt = \"the plant\"; }\n    }\n", got)
	pictured := got
	got = applyLayout(t, pictured, SetPicture("PlantViews::loopView", &semantics.Picture{Location: "plant.png", X: 15, Y: 25, Width: 300, Height: 200, Above: true}))
	requireReplaced(t, pictured, `{ location = "plant.png"; x = 10; y = 20; width = 300; height = 200; alt = "the plant"; }`,
		`{ location = "plant.png"; x = 15; y = 25; width = 300; height = 200; above = true; }`, got)
	const annotation = `@DiagramLayout::Picture { location = "plant.png"; x = 10; y = 20; width = 300; height = 200; alt = "the plant"; }`
	got = applyLayout(t, pictured, SetPictureAt(annotationSpan(t, pictured, annotation), &semantics.Picture{Location: "site.png", X: 10, Y: 20, Width: 30, Height: 20}))
	requireReplaced(t, pictured, annotation, `@DiagramLayout::Picture { location = "site.png"; x = 10; y = 20; width = 30; height = 20; }`, got)
	got = applyLayout(t, pictured, SetPicture("PlantViews::loopView", nil))
	if got != plantModel {
		t.Fatalf("clearing the picture did not restore the model:\n%s", got)
	}
}

func TestSetNoteAndPictureRefusals(t *testing.T) {
	pictured := strings.Replace(plantModel, "        render asInterconnectionDiagram;\n",
		"        render asInterconnectionDiagram;\n        @DiagramLayout::Picture { location = \"plant.png\"; x = 1; y = 2; width = 3; height = 4; }\n", 1)
	pictureSpan := annotationSpan(t, pictured, `@DiagramLayout::Picture { location = "plant.png"; x = 1; y = 2; width = 3; height = 4; }`)
	cases := []struct {
		name    string
		op      Operation
		failure Failure
		message string
	}{
		{"empty note", SetNote("Plant::Loop::pump", "", &semantics.Note{Text: " ", X: 1, Y: 2}), FailureInvalidValue, "shows no text"},
		{"note on nothing drawn", SetNote("Plant::Pump", "PlantViews::loopView", &semantics.Note{Text: "x", X: 1, Y: 2}), FailureNotExposed, "does not expose"},
		{"picture without a file", SetPicture("PlantViews::loopView", &semantics.Picture{X: 1, Y: 2, Width: 3, Height: 4}), FailureInvalidValue, "names no file"},
		{"flat picture", SetPicture("PlantViews::loopView", &semantics.Picture{Location: "p.png", X: 1, Y: 2}), FailureInvalidValue, "positive width and height"},
		{"picture on a part", SetPicture("Plant::Loop::pump", &semantics.Picture{Location: "p.png", X: 1, Y: 2, Width: 3, Height: 4}), FailureNotAView, "is no view"},
		{"nothing to clear", SetNote("Plant::Loop::pump", "", nil), FailureNotAnnotated, "no Note"},
		{"note at a part", SetNoteAt(annotationSpan(t, pictured, "part pump : Pump;"), &semantics.Note{Text: "x", X: 1, Y: 2}), FailureNotAnnotated, "no Note annotation"},
		{"picture at a note", SetNoteAt(pictureSpan, &semantics.Note{Text: "x", X: 1, Y: 2}), FailureNotAnnotated, "no Note annotation"},
		{"declared with a view", Operation{Kind: OpSetLayout, Declaration: pictureSpan, View: "PlantViews::loopView", Annotation: semantics.PictureFQN}, FailureInvalidValue, "name no view"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "plant.sysml", pictured)
			_, err := Apply(m, []Operation{tc.op})
			e := editError(t, err)
			if e.Failure != tc.failure || !strings.Contains(e.Message, tc.message) {
				t.Fatalf("err = %v, want %s containing %q", err, tc.failure, tc.message)
			}
		})
	}
}
