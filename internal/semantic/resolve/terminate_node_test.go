package resolve

import "testing"

// The occurrence a terminate action usage names (`action halt terminate x;`) is
// resolved as the bare statement's target is: an undeclared name is reported.
func TestResolveTerminateNodeTargetUnresolved(t *testing.T) {
	r := resolveDoc(t, "<t>", "package P { action def A { action halt terminate nowhere; } }")
	if len(r.Diagnostics) == 0 {
		t.Fatalf("expected an unresolved-reference diagnostic for the terminate node's target")
	}
}

func TestResolveTerminateNodeTargetDeclared(t *testing.T) {
	r := resolveDoc(t, "<t>", "package P { action def W; action def A { ref action process : W; action halt terminate process; } }")
	if len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", r.Diagnostics)
	}
}
