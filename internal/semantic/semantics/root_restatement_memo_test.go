package semantics

import "testing"

// Whether a member restates a frame root or the featuring reference is decided
// once per symbol: a repeated query allocates nothing and answers as the first did.
func TestRootRestatementIsDecidedOncePerSymbol(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		occurrence def O { ref occurrence slices :> timeSlices; ref occurrence other; }
	}`)
	p := sym(t, root, "P")
	o := nested(t, p.Scope, "O")
	slices, other := nested(t, o.Scope, "slices"), nested(t, o.Scope, "other")
	if !m.restatesRoot(slices, frameRootSet) {
		t.Fatalf("restatesRoot(slices, frame roots) = false, want true: it subsets Occurrence::timeSlices")
	}
	if m.restatesRoot(other, frameRootSet) || m.NamesFeaturingReference(other) {
		t.Fatalf("other restates a root, want none")
	}
	for name, query := range map[string]func(){
		"restatesRoot(slices, frame roots)": func() { m.restatesRoot(slices, frameRootSet) },
		"restatesRoot(other, frame roots)":  func() { m.restatesRoot(other, frameRootSet) },
		"NamesFeaturingReference(other)":    func() { m.NamesFeaturingReference(other) },
	} {
		if got := testing.AllocsPerRun(20, query); got != 0 {
			t.Errorf("a repeated %s query allocates %.0f times, want 0", name, got)
		}
	}
	if !m.restatesRoot(slices, frameRootSet) || m.restatesRoot(other, frameRootSet) {
		t.Fatalf("memoized answers differ from the first")
	}
}
