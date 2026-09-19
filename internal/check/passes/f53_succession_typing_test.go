package passes

import (
	"strings"
	"testing"
)

// F53: the pilot enforces no succession-specific typing kind — only the
// generic `A usage must be typed by definitions` — so a definition of any
// kind types a succession (SysML.xtext:1033 SuccessionAsUsage).

func TestF53SuccessionTypedByAnyDefinition(t *testing.T) {
	for _, src := range []string{
		"part def A; part p1 : A; part p2 : A; succession s : A first p1 then p2;",
		"action def AD; action paint; action dry; succession s : AD first paint then dry;",
		"attribute def M; action paint; action dry; succession s : M first paint then dry;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// A binding also types through a plain UsageDeclaration (SysML.xtext:1020
// BindingConnectorAsUsage), so the same rule applies.
func TestF53BindingTypedByAnyDefinition(t *testing.T) {
	for _, src := range []string{
		"part def A; connection def AB { end e1 : A; end e2 : A; } part a; part b; binding ab1 : AB bind a = b;",
		"part def A; attribute def M; part a; part b; binding ab1 : M bind a = b;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

func TestF53BindingTypedByUsageRejected(t *testing.T) {
	// The pilot rejects this with `A usage must be typed by definitions`.
	diags := typeDiags(t, "part def A; part pu : A; part a; part b; binding ab1 : pu bind a = b;")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "must be a definition") {
		t.Errorf("got %q", diags[0].Message)
	}
}

func TestF53SuccessionTypedByUsageRejected(t *testing.T) {
	// The pilot rejects this with `A usage must be typed by definitions`.
	diags := typeDiags(t, "part def A; part pu : A; part p1 : A; part p2 : A; succession s : pu first p1 then p2;")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if !strings.Contains(diags[0].Message, "must be a definition") {
		t.Errorf("got %q", diags[0].Message)
	}
}

func TestF53SuccessionUnresolvedTypeNoPanic(t *testing.T) {
	diags := typeDiags(t, "action paint; action dry; succession s : NoSuchDef first paint then dry;")
	for _, d := range diags {
		if strings.Contains(d.Message, "kind mismatch") {
			t.Fatalf("unresolved type must not draw a kind mismatch, got %v", diags)
		}
	}
}
