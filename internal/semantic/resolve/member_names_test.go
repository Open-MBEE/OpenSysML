package resolve

import "testing"

// memberNames is a fixed pair, so the distinguishability checks that range over
// it for every member of every scope with imports allocate nothing per member;
// a slice here escaped and cost one allocation per member per analysis.
func TestMemberNamesIsAFixedPairAndDoesNotAllocate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"app.sysml": `package App {
			part def Plain;
			part def <s> Short;
			part def Same;
		}`,
	})
	r := New(idx)
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	plain := scopeOf(t, app, "Plain").Owner()
	short := scopeOf(t, app, "Short").Owner()
	if plain == nil || short == nil {
		t.Fatal("Plain or Short has no owning symbol")
	}

	if got := r.memberNames(plain); got != [2]string{"Plain", ""} {
		t.Errorf("memberNames(Plain) = %q, want [Plain \"\"]", got)
	}
	if got := r.memberNames(short); got != [2]string{"Short", "s"} {
		t.Errorf("memberNames(Short) = %q, want [Short s]", got)
	}
	for _, c := range []struct {
		label string
		run   func()
	}{
		{"declared name only", func() {
			for _, name := range r.memberNames(plain) {
				_ = name
			}
		}},
		{"declared short name", func() {
			for _, name := range r.memberNames(short) {
				_ = name
			}
		}},
	} {
		if allocs := testing.AllocsPerRun(100, c.run); allocs != 0 {
			t.Errorf("ranging over memberNames (%s) allocates %.0f objects per call, want 0", c.label, allocs)
		}
	}
}
