package semantics

import "testing"

func TestIsBehaviorParameter(t *testing.T) {
	_, root := buildModel(t, `
		item def Cmd;
		port def P { in item cmd : Cmd; }
		action def A { in item cmd : Cmd; out item done : Cmd; }
		calc def C { in x : Cmd; return y : Cmd; }
		part def Q { item stored : Cmd; }
	`)
	member := func(owner, name string) string { return owner + "::" + name }
	cases := map[string]bool{
		member("P", "cmd"):    false,
		member("A", "cmd"):    true,
		member("A", "done"):   true,
		member("C", "x"):      true,
		member("C", "y"):      true,
		member("Q", "stored"): false,
	}
	seen := 0
	for _, owner := range []string{"P", "A", "C", "Q"} {
		for _, s := range sym(t, root, owner).Scope.AllMembers() {
			want, ok := cases[member(owner, s.Name)]
			if !ok {
				continue
			}
			seen++
			if got := IsBehaviorParameter(s); got != want {
				t.Errorf("IsBehaviorParameter(%s) = %v, want %v", member(owner, s.Name), got, want)
			}
		}
	}
	if seen != len(cases) {
		t.Fatalf("checked %d members, want %d", seen, len(cases))
	}
}
