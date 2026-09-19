package passes

import "testing"

// A `subject` reaches the usage-kind rules however its body is written. The
// requirement-specific body path yields an *ast.SubjectMember rather than an
// *ast.Usage, so before this the check depended on which keyword opened the
// body: a leading `subject` escaped it, one preceded by an `attribute` did not.
func TestTypeCheckSubjectIsCheckedWhateverPrecedesIt(t *testing.T) {
	// A subject's type must be a definition; a usage is an error in every form.
	for _, src := range []string{
		"part def V; part v : V; requirement def R { subject s : v; }",
		"part def V; part v : V; requirement def R { attribute x; subject s : v; }",
		"part def V; part v : V; requirement def R { doc /* d */ subject s : v; }",
		"part def V; part v : V; requirement R { subject s : v; }",
		"part def V; part v : V; concern def C { subject s : v; }",
		"part def V; part v : V; use case def U { subject s : v; }",
	} {
		if diags := typeDiags(t, src); len(diags) != 1 {
			t.Errorf("%s: expected one type diagnostic, got %v", src, diags)
		}
	}
}

// A requirement usage's subject is checked too; it always takes the requirement
// body path, so it was never checked before.
func TestTypeCheckRequirementUsageSubjectIsChecked(t *testing.T) {
	if diags := typeDiags(t, "part def V; part v : V; requirement R { attribute x; subject s : v; }"); len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if diags := typeDiags(t, "part def V; requirement R { subject s : V; }"); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

// A subject with no type, and one whose type does not resolve, belong to other
// tiers and must not be reported here.
func TestTypeCheckSubjectWithoutResolvableTypeIsNotATypeError(t *testing.T) {
	for _, src := range []string{
		"requirement def R { subject s; }",
		"requirement def R { subject s : Undeclared; }",
	} {
		for _, d := range typeDiags(t, src) {
			t.Errorf("%s: unexpected type diagnostic %v", src, d)
		}
	}
}
