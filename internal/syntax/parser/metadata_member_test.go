package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// `@` introduces a metadata usage (SysML v2 grammar: MetadataUsageKeyword), so
// `@Security;` is a member of its own wherever a member may be written.
func TestMetadataUsageMember(t *testing.T) {
	src := `package P {
	metadata def Security;
	@Security;
	part x {
		@Security;
	}
	part y;
	@Security;
}`
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
	}

	var annotations, parts int
	var walk func(nodes []ast.Node)
	walk = func(nodes []ast.Node) {
		for _, node := range nodes {
			if membership, ok := node.(*ast.Membership); ok {
				node = membership.Member
			}
			switch n := node.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.PrefixMetadata:
				annotations++
				if got := ast.SimpleName(n.Type); got != "Security" {
					t.Errorf("expected the metadata type Security, got %q", got)
				}
			case *ast.Usage:
				if n.Kind == ast.UsagePart {
					parts++
					if len(n.Prefixes) != 0 {
						t.Errorf("a metadata member is not a prefix of the member after it: %v", n.Prefixes)
					}
					walk(n.Members)
				}
			}
		}
	}
	walk(root.Members)

	if annotations != 3 || parts != 2 {
		t.Errorf("expected 3 metadata members and 2 parts, got %d and %d", annotations, parts)
	}
}

// A prefix names its definition by any qualified name, the global `$::` form
// included, wherever a member may be written (SysML.xtext PrefixMetadataUsage).
func TestGloballyQualifiedPrefixMetadata(t *testing.T) {
	sources := map[string]string{
		"package member":   "package P { metadata def M; #$::P::M part x; #$::P::M part def D; }",
		"root declaration": "metadata def M; #$::M part x; #$::M part def D;",
		"nested member":    "package P { metadata def M; part def D { #$::P::M part x; } }",
		"root package":     "metadata def M; #$::M package Q;",
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
			}
			var prefixes []*ast.PrefixMetadata
			var walk func(nodes []ast.Node)
			walk = func(nodes []ast.Node) {
				for _, node := range nodes {
					if membership, ok := node.(*ast.Membership); ok {
						node = membership.Member
					}
					switch n := node.(type) {
					case *ast.Package:
						prefixes = append(prefixes, n.Prefixes...)
						walk(n.Members)
					case *ast.Definition:
						prefixes = append(prefixes, n.Prefixes...)
						walk(n.Members)
					case *ast.Usage:
						prefixes = append(prefixes, n.Prefixes...)
					}
				}
			}
			walk(root.Members)
			if len(prefixes) != strings.Count(src, "#") {
				t.Fatalf("expected %d prefixes, got %d", strings.Count(src, "#"), len(prefixes))
			}
			for _, prefix := range prefixes {
				if prefix.Type == nil || !prefix.Type.Global || ast.SimpleName(prefix.Type) != "M" {
					t.Errorf("expected the global name $::…::M, got %v", prefix.Type)
				}
			}
		})
	}
}

// An objective takes prefix metadata after its keyword, as a subject does
// (SysML.xtext ObjectiveRequirementUsage `UsageExtensionKeyword* …`).
func TestObjectivePrefixMetadataFollowsTheKeyword(t *testing.T) {
	src := "package P { metadata def M; requirement def R; use case def U { objective #M o : R { subject s; } } }"
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
	}
	useCase := memberAt(t, root, 0, 2).(*ast.Definition)
	objective, ok := unwrapMember(t, useCase.Members[0]).(*ast.Usage)
	if !ok || objective.Kind != ast.UsageObjective {
		t.Fatalf("first member = %T %v, want an objective usage", useCase.Members[0], objective)
	}
	if len(objective.Prefixes) != 1 || ast.SimpleName(objective.Prefixes[0].Type) != "M" {
		t.Fatalf("objective prefixes = %v, want #M", objective.Prefixes)
	}
	if objective.Ident.Name != "o" || len(objective.Relationships) != 1 || len(objective.Members) != 1 {
		t.Errorf("objective declaration lost: name %q, %d relationships, %d members", objective.Ident.Name, len(objective.Relationships), len(objective.Members))
	}
}

// An unterminated metadata usage is reported rather than read as a prefix of the
// declaration after it; `#Type` is the prefix spelling.
func TestMetadataUsageRequiresTerminator(t *testing.T) {
	p := New(source.New("test.sysml", []byte("package P { metadata def M; @M part def Car; }")))
	root := p.ParseFile()
	if len(p.Diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %v", p.Diagnostics)
	}
	if !strings.Contains(p.Diagnostics[0].Message, "after a metadata usage") {
		t.Errorf("unexpected message %q", p.Diagnostics[0].Message)
	}

	// Recovery leaves the declaration itself parsed.
	var parts int
	for _, member := range root.Members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		pkg, ok := member.(*ast.Package)
		if !ok {
			continue
		}
		for _, m := range pkg.Members {
			if membership, ok := m.(*ast.Membership); ok {
				m = membership.Member
			}
			if def, ok := m.(*ast.Definition); ok && def.Kind == ast.DefPart {
				parts++
			}
		}
	}
	if parts != 1 {
		t.Errorf("expected the part def to still be parsed, got %d", parts)
	}
}
