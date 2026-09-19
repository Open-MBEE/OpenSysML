package repl

import "testing"

// A requirement's subject, assume and require members are reachable by their
// short names in the REPL as a `part <p>` is, and a short-name-only member is
// listed under that name.
func TestRequirementMemberShortNames(t *testing.T) {
	s := NewSession()
	res := s.Submit("package B { part def T; constraint def C; requirement def R { subject <s> x : T; assume constraint <a> ac : C; require constraint <r> rc : C; } requirement def R2 { subject <q> : T; } }")
	if len(res.Diagnostics) > 0 {
		t.Fatalf("declaration has diagnostics: %v", res.Diagnostics)
	}
	for _, tc := range []struct{ line, want string }{
		{"%print B::R::s", "subject <s> x : T;"},
		{"%print B::R::a", "assume constraint <a> ac : C;"},
		{"%print B::R::r", "require constraint <r> rc : C;"},
		{"%print B::R2::q", "subject <q> : T;"},
	} {
		wants(t, run(t, s, tc.line), tc.want)
	}
	got := run(t, s, "%search B::R")
	wants(t, got, "B::R::x  subject", "B::R::ac  assume constraint", "B::R::rc  require constraint", "B::R2::q  subject")
}
