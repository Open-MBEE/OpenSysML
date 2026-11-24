package runtime

import "testing"

func noteSerials(ctx *Context) []int64 {
	var out []int64
	for _, n := range ctx.run.notes {
		if c, ok := n.(ChoicePoint); ok {
			out = append(out, c.serial)
		}
	}
	return out
}

// Two contexts number their choice points apart: what one classifies leaves the
// serials of the other as a run alone would draw them.
func TestChoiceSerialsAreNumberedPerContext(t *testing.T) {
	const model = `package test { item def A; }`
	first, _ := libraryShapeContext(t, model)
	second, _ := libraryShapeContext(t, model)
	choice := ChoicePoint{Kind: ChoiceDueOrder, Alternatives: []string{"a", "b"}}
	for i := 0; i < 3; i++ {
		first.note(choice)
		second.note(choice)
		second.note(choice)
	}
	for name, ctx := range map[string]*Context{"first": first, "second": second} {
		got := noteSerials(ctx)
		for i, serial := range got {
			if serial != int64(i+1) {
				t.Fatalf("%s context drew serials %v, want 1..%d", name, got, len(got))
			}
		}
	}
	if len(noteSerials(first)) != 3 || len(noteSerials(second)) != 6 {
		t.Fatalf("contexts kept %d and %d choice points, want 3 and 6", len(noteSerials(first)), len(noteSerials(second)))
	}
}

// A reader's notes are its seeding context's, so the two draw from one sequence:
// no choice point either makes shares a serial with one of the other's.
func TestChoiceSerialsAreSharedWithASeededReader(t *testing.T) {
	held, _ := libraryShapeContext(t, `package test { item def A; }`)
	reader := NewDeclaredReaderIn(held)
	choice := ChoicePoint{Kind: ChoiceDueOrder, Alternatives: []string{"a", "b"}}
	held.note(choice)
	reader.ctx.note(choice)
	held.note(choice)
	got := noteSerials(held)
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("the seeding context reports serials %v, want [1 2 3]", got)
	}
}
