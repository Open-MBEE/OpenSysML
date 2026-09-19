package passes

import "testing"

// A `featured by` name the declaration's own body inherits from its type
// resolves, as the pinned KerML validator has it (matched run, W6C row ~980).
func TestW6CHeaderNameInheritedByTheDeclarationsBody(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "w6c_header.kerml", `package R {
	datatype Amount;
	classifier Base {
		feature q : Amount;
	}
	classifier C {
		feature x : Base featured by q;
	}
}`)
	if got := Analyze("w6c_header.kerml", root, pd, idx); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// A subsetting target does not see the members its own declaration inherits:
// the pinned validator reports `q` unresolved there too (matched run).
func TestW6CSubsettingTargetDoesNotSeeInheritedMembers(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "w6c_subsets.kerml", `package R {
	datatype Amount;
	classifier Base {
		feature q : Amount;
	}
	classifier C {
		feature x : Base :> q;
	}
}`)
	got := Analyze("w6c_subsets.kerml", root, pd, idx)
	if len(got) != 1 || got[0].Code != "unresolved" {
		t.Fatalf("got %+v, want one unresolved-reference diagnostic", got)
	}
}

// A redefinition target is not looked up among the redefining feature's own
// inherited members: it would find the feature itself and report unresolved.
func TestW6CRedefinitionTargetIsNotResolvedInTheHeaderScope(t *testing.T) {
	root, pd, idx := analyzeInputs(t, "w6c_redefines.kerml", `package R {
	datatype Amount;
	classifier Base {
		feature q : Amount;
	}
	classifier C specializes Base {
		feature redefines q;
	}
}`)
	if got := Analyze("w6c_redefines.kerml", root, pd, idx); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}
