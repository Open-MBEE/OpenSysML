package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// endTargets names every end a declaration parsed, in order.
func endTargets(t *testing.T, src string) []string {
	t.Helper()
	member, diags := parseOneMemberWithDiags(t, src)
	if len(diags) > 0 {
		t.Fatalf("%q: unexpected diagnostics %v", src, diags)
	}
	def, ok := member.(*ast.Definition)
	if !ok {
		t.Fatalf("%q: expected a definition, got %T", src, member)
	}
	var names []string
	for _, m := range def.Members {
		mem, isMembership := m.(*ast.Membership)
		if !isMembership {
			continue
		}
		u, isUsage := mem.Member.(*ast.Usage)
		if !isUsage || len(u.ConnectorEnds) == 0 {
			continue
		}
		for _, end := range u.ConnectorEnds {
			names = append(names, endTargetName(end))
		}
	}
	return names
}

// endTargetName names the feature an end attaches to; a chain attaches to its
// last segment, the way lower.endName reads it.
func endTargetName(end *ast.ConnectorEnd) string {
	if chain, isChain := end.Target.(*ast.FeatureChainExpr); isChain {
		return ast.SimpleName(chain.Member)
	}
	return ast.SimpleName(end.Target)
}

// A parenthesized connector clause keeps every end it lists, not just the
// first: `connect (a, b, c)` is n-ary (SysML v2 §7.13.2, §8.3.13).
func TestParseNaryConnectorEndsKeepsEveryEnd(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			"binary control",
			"part def C { part a; part b; connection conn connect a to b; }",
			[]string{"a", "b"},
		},
		{
			"ternary connection",
			"part def C { part a; part b; part c; connection conn connect (a, b, c); }",
			[]string{"a", "b", "c"},
		},
		{
			"quaternary connection",
			"part def C { part a; part b; part c; part d; connection conn connect (a, b, c, d); }",
			[]string{"a", "b", "c", "d"},
		},
		{
			"ternary connector",
			"part def C { part a; part b; part c; connector conn connect (a, b, c); }",
			[]string{"a", "b", "c"},
		},
		{
			"ternary interface",
			"part def C { port p; port q; port r; interface i connect (p, q, r); }",
			[]string{"p", "q", "r"},
		},
		{
			"ternary allocation",
			"part def C { part f; part g; part h; allocation al allocate (f, g, h); }",
			[]string{"f", "g", "h"},
		},
		{
			"ends given as feature chains",
			"part def C { part a; part b; part c; connection conn connect (a.out, b.in, c.in); }",
			[]string{"out", "in", "in"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := endTargets(t, tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("parsed %d ends %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("end %d = %q, want %q (all: %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

func TestParseNaryNamedConnectorEndsKeepReferenceSubsettings(t *testing.T) {
	u := parseOneMember(t, "allocation a allocate (logical ::> l, physical references p);").(*ast.Usage)
	if len(u.ConnectorEnds) != 2 {
		t.Fatalf("expected 2 ends, got %d", len(u.ConnectorEnds))
	}
	for i, want := range []struct {
		name string
		ref  string
	}{
		{name: "logical", ref: "l"},
		{name: "physical", ref: "p"},
	} {
		end := u.ConnectorEnds[i]
		decl, ok := end.DeclaredName()
		if !ok || decl.Name != want.name {
			t.Fatalf("end %d declared name = %#v, %v; want %q", i, decl, ok, want.name)
		}
		if len(end.Relationships) != 1 || end.Relationships[0].Kind != ast.RelReferences {
			t.Fatalf("end %d relationships = %#v, want one references relationship", i, end.Relationships)
		}
		if got := ast.SimpleName(end.ReferencedTarget()); got != want.ref {
			t.Fatalf("end %d reference target = %q, want %q", i, got, want.ref)
		}
	}
}

// A malformed n-ary clause keeps the ends parsed so far and reports, rather
// than dropping ends silently or panicking.
func TestParseNaryConnectorEndsMalformedKeepsPartialEnds(t *testing.T) {
	src := "part def C { part a; part b; connection conn connect (a, b, ); }"
	p := New(source.New("<t>", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for a trailing comma in a connector clause")
	}
	if root == nil {
		t.Fatalf("parser returned no tree")
	}
}

// The anonymous inline form declares no connection name but is the same
// connector clause, so it keeps every end a parenthesized list gives it.
func TestParseAnonymousInlineConnectKeepsEveryEnd(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			"binary",
			"part def C { part a; part b; connect a to b; }",
			[]string{"a", "b"},
		},
		{
			"ternary",
			"part def C { part a; part b; part c; connect (a, b, c); }",
			[]string{"a", "b", "c"},
		},
		{
			"quaternary",
			"part def C { part a; part b; part c; part d; connect (a, b, c, d); }",
			[]string{"a", "b", "c", "d"},
		},
		{
			"ends given as feature chains",
			"part def C { part a; part b; part c; connect (a.out, b.in, c.in); }",
			[]string{"out", "in", "in"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := endTargets(t, tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("parsed %d ends %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("end %d = %q, want %q (all: %v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// The anonymous form still reports a malformed clause rather than accepting it.
func TestParseAnonymousInlineConnectMalformedReports(t *testing.T) {
	for _, src := range []string{
		"part def C { part a; part b; connect (a, b; }",
		"part def C { part a; part b; connect a b; }",
		"part def C { connect (); }",
		"part def C { part a; connect a to; }",
	} {
		p := New(source.New("<t>", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) == 0 {
			t.Errorf("%q: expected a diagnostic", src)
		}
		if root == nil {
			t.Errorf("%q: parser returned no tree", src)
		}
	}
}
