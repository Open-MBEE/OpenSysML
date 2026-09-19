package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F60: prefix metadata as a member's only keyword. ExtendedUsage is
// UnextendedUsagePrefix UsageExtensionKeyword+ Usage (SysML.xtext:730), so one
// or more `#M` annotations may stand where a kind keyword would — alone and
// after modifiers — and the member is a plain Usage, not an attribute usage.
func TestF60PrefixMetadataUsage(t *testing.T) {
	parseOne := func(t *testing.T, src string) *ast.Usage {
		t.Helper()
		sf := source.New("f60.sysml", []byte(src))
		p := New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
		}
		mem := root.Members[0].(*ast.Membership)
		pkg := mem.Member.(*ast.Package)
		u := findFirstUsageWithPrefixes(pkg.Members)
		if u == nil {
			t.Fatalf("no prefixed usage found in %q", src)
		}
		return u
	}

	t.Run("prefix_before_connect", func(t *testing.T) {
		u := parseOne(t, "package B { metadata def M; part a; part b; #M connect a to b; }")
		if u.Kind != ast.UsageConnection {
			t.Errorf("kind = %v, want connection", u.Kind)
		}
		if len(u.Prefixes) != 1 || ast.SimpleName(u.Prefixes[0].Type) != "M" {
			t.Errorf("prefixes = %v, want [#M]", u.Prefixes)
		}
	})

	t.Run("prefix_after_modifier", func(t *testing.T) {
		u := parseOne(t, "package B { metadata def Classified; abstract #Classified z; }")
		if !u.IsAbstract {
			t.Error("IsAbstract = false, want true")
		}
		if len(u.Prefixes) != 1 || ast.SimpleName(u.Prefixes[0].Type) != "Classified" {
			t.Errorf("prefixes = %v, want [#Classified]", u.Prefixes)
		}
	})

	t.Run("prefix_after_end_keeps_end", func(t *testing.T) {
		u := parseOne(t, "package B { metadata def original; requirement def Req1; connection def D { end #original r1 : Req1; } }")
		if !u.IsEnd {
			t.Error("IsEnd = false, want true")
		}
		if len(u.Prefixes) != 1 || ast.SimpleName(u.Prefixes[0].Type) != "original" {
			t.Errorf("prefixes = %v, want [#original]", u.Prefixes)
		}
	})

	t.Run("prefix_with_redefinition", func(t *testing.T) {
		u := parseOne(t, "package B { metadata def service; port def PD; part def P { port sd : PD; } part def Q :> P { #service :>> sd : PD; } }")
		if len(u.Prefixes) != 1 || ast.SimpleName(u.Prefixes[0].Type) != "service" {
			t.Errorf("prefixes = %v, want [#service]", u.Prefixes)
		}
		hasRedef := false
		for _, r := range u.Relationships {
			if r.Kind == ast.RelRedefines {
				hasRedef = true
			}
		}
		if !hasRedef {
			t.Error("no redefines relationship parsed")
		}
	})
}

// findFirstUsageWithPrefixes walks members depth-first for a prefixed usage.
func findFirstUsageWithPrefixes(members []ast.Node) *ast.Usage {
	for _, m := range members {
		n := m
		if mem, ok := n.(*ast.Membership); ok {
			n = mem.Member
		}
		switch d := n.(type) {
		case *ast.Usage:
			if len(d.Prefixes) > 0 {
				return d
			}
			if u := findFirstUsageWithPrefixes(d.Members); u != nil {
				return u
			}
		case *ast.Definition:
			if u := findFirstUsageWithPrefixes(d.Members); u != nil {
				return u
			}
		}
	}
	return nil
}

// Malformed prefix-metadata members must produce diagnostics, never a panic.
func TestF60Negative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"hash_alone", "package B { # ; }"},
		{"hash_no_member", "package B { #M }"},
		{"modifier_hash_no_member", "part def P { abstract #Classified }"},
		{"modifier_hash_nothing", "part def P { abstract # ; }"},
		{"end_hash_dangling", "connection def D { end #original }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New("f60_neg.sysml", []byte(tt.input))
			p := New(sf)
			p.ParseFile()
			if len(p.Diagnostics) == 0 {
				t.Errorf("expected diagnostics for %q, got none", tt.input)
			}
		})
	}
}
