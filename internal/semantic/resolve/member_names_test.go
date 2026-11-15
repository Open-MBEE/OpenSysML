package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// memberNames is a fixed pair, so the distinguishability checks that range over
// it for every member of every scope with imports allocate nothing per member;
// a slice here escaped and cost one allocation per member per analysis.
func TestMemberNamesIsAFixedPairAndDoesNotAllocate(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"app.sysml": `package App {
			part def Plain;
			part def <s> Short;
			part def Same;
			part def '';
		}`,
	})
	r := New(idx)
	app := scopeOf(t, idx.DocumentRoot("app.sysml"), "App")
	plain := scopeOf(t, app, "Plain").Owner()
	short := scopeOf(t, app, "Short").Owner()
	if plain == nil || short == nil {
		t.Fatal("Plain or Short has no owning symbol")
	}
	var empty *symbols.Symbol
	for _, sym := range app.Members() {
		if sym.Named() && sym.Name == "" {
			empty = sym
		}
	}
	if empty == nil {
		t.Fatal("the definition named by the empty unrestricted name is not a named member")
	}

	if got, n := r.memberNames(plain); got != [2]string{"Plain", ""} || n != 1 {
		t.Errorf("memberNames(Plain) = %q, %d, want [Plain \"\"], 1", got, n)
	}
	if got, n := r.memberNames(short); got != [2]string{"Short", "s"} || n != 2 {
		t.Errorf("memberNames(Short) = %q, %d, want [Short s], 2", got, n)
	}
	// The empty name is a name the membership binds, told from none by the count.
	if got, n := r.memberNames(empty); got != [2]string{} || n != 1 {
		t.Errorf("memberNames('') = %q, %d, want [\"\" \"\"], 1", got, n)
	}
	for _, c := range []struct {
		label string
		run   func()
	}{
		{"declared name only", func() {
			names, n := r.memberNames(plain)
			for _, name := range names[:n] {
				_ = name
			}
		}},
		{"declared short name", func() {
			names, n := r.memberNames(short)
			for _, name := range names[:n] {
				_ = name
			}
		}},
	} {
		if allocs := testing.AllocsPerRun(100, c.run); allocs != 0 {
			t.Errorf("ranging over memberNames (%s) allocates %.0f objects per call, want 0", c.label, allocs)
		}
	}
}
