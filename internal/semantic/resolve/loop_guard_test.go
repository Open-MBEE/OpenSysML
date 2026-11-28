package resolve

import (
	"strings"
	"testing"
)

// A loop owns its conditions beside its body (SysML 8.3.17.19, KerML 8.2.3.5.4), so a `while` or
// `until` condition does not read what a `{ … }` body declares.
func TestResolveLoopConditionDoesNotReadBodyDeclarations(t *testing.T) {
	for name, src := range map[string]string{
		"until attribute": `package P { action def A {
			loop { attribute bodyOnly; } until bodyOnly;
		} }`,
		"while attribute": `package P { action def A {
			while bodyOnly { attribute bodyOnly; }
		} }`,
		"until action": `package P { action def A {
			loop { action bodyOnly { out attribute done; } } until bodyOnly.done;
		} }`,
		"named body": `package P { action def A {
			loop action body1 { attribute bodyOnly; } until bodyOnly;
		} }`,
	} {
		t.Run(name, func(t *testing.T) {
			r := resolveDoc(t, "d.sysml", src)
			if len(r.Diagnostics) != 1 {
				t.Fatalf("expected one diagnostic for bodyOnly, got %v", r.Diagnostics)
			}
			d := r.Diagnostics[0]
			guard := strings.Index(src, "until bodyOnly")
			if guard < 0 {
				guard = strings.Index(src, "while bodyOnly")
			}
			if want := guard + len("until "); d.Span.Offset != want || !strings.Contains(d.Message, "bodyOnly") {
				t.Errorf("got %q at offset %d, want bodyOnly unresolved at %d", d.Message, d.Span.Offset, want)
			}
		})
	}
}

// What the condition does read: the enclosing action's features, which a body's declaration of the
// same name does not hide, and a body written as an action, by that action's name.
func TestResolveLoopConditionReadsEnclosingAndNamedBody(t *testing.T) {
	src := `package P { action def A {
		attribute flag;
		loop { attribute flag; } until flag;
		while flag { attribute flag; }
		loop action body1 { attribute x; } until body1.x;
	} }`
	r := resolveDocWithModel(t, "d.sysml", src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
	enclosing := r.Index().LookupQualified("P::A::flag")
	if len(enclosing) != 1 {
		t.Fatalf("P::A::flag declares %d symbols, want 1", len(enclosing))
	}
	conditions := 0
	for qn, syms := range r.parts {
		if len(qn.Parts) != 1 || qn.Parts[0].Text != "flag" {
			continue
		}
		conditions++
		if syms[0] != enclosing[0] {
			t.Errorf("condition flag at offset %d does not resolve to the enclosing A::flag", qn.Span().Offset)
		}
	}
	if conditions != 2 {
		t.Errorf("resolved %d flag conditions, want 2", conditions)
	}
}
