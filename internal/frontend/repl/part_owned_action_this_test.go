package repl

import "testing"

// partOwnedActionThisModel owns a typed action usage in a part definition whose
// body redefines the action's part-typed feature with `this`.
const partOwnedActionThisModel = `action def A {
    ref part ctx : P;
}
part def P {
    action a : A {
        :>> ctx = this;
    }
}
part p : P;
`

// `this` in an action usage a part owns is the owning part, not the action's
// own performance: the usage is one of the part's owned performances, so the
// feature it values with `this` reads as the part's object.
func TestFeaturesReportsThisInPartOwnedActionAsTheOwningPart(t *testing.T) {
	s := loadSource(t, partOwnedActionThisModel)
	wants(t, run(t, s, "%instantiate p"), "Created instance of p", "ID: 1")

	got := run(t, s, "%features p.a")
	wants(t, got, "Instance: p.a (ID: 2)", "ctx = Instance(ID: 1)")
	rejects(t, got, "type mismatch", "<error:", "ctx = Instance(ID: 2)")
}
