package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// An entry/do/exit action given by reference is a performed action usage whose
// reference subsetting names an action declared elsewhere, so it binds through
// the ordinary name-resolution tier.
func TestResolveStateSubactionReferenceClean(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def S {
			action warmUp;
			action monitor;
			action coolDown;
			state running {
				entry warmUp;
				do monitor;
				exit coolDown;
			}
		}
	`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %v", r.Diagnostics)
	}
}

func TestResolveStateSubactionReferenceUnresolved(t *testing.T) {
	r := resolveDoc(t, "d.sysml", `
		state def S {
			action warmUp;
			state running {
				entry warmUp;
				exit coolDown;
			}
		}
	`)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected one unresolved diagnostic for coolDown, got %v", r.Diagnostics)
	}
}

// The referenced action binds to the action's own symbol, not merely to
// something that silences the diagnostic.
func TestResolveStateSubactionReferenceBindsSymbol(t *testing.T) {
	const src = `
		state def S {
			action warmUp;
			state running {
				entry warmUp;
			}
		}
	`
	p := parser.New(source.New("d.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}

	idx := symbols.NewIndexFromDoc("d.sysml", root)
	r := New(idx)
	r.ResolveDocument("d.sysml", root)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolve diagnostics: %v", r.Diagnostics)
	}

	entry := findEntryMember(root)
	if entry == nil {
		t.Fatal("no entry member in the parsed state")
	}
	if len(entry.Actions) != 1 {
		t.Fatalf("expected one entry action, got %d", len(entry.Actions))
	}
	usage, ok := entry.Actions[0].(*ast.Usage)
	if !ok {
		t.Fatalf("expected the entry action to be a performed action usage, got %T", entry.Actions[0])
	}
	if len(usage.Relationships) != 1 || usage.Relationships[0].Kind != ast.RelReferences {
		t.Fatalf("expected a single reference subsetting, got %v", usage.Relationships)
	}

	qn, ok := usage.Relationships[0].Target.(*ast.QualifiedName)
	if !ok {
		t.Fatalf("expected a qualified name target, got %T", usage.Relationships[0].Target)
	}
	sym, ok := r.PartSymbol(qn, 0)
	if !ok {
		t.Fatal("the referenced action has no resolved symbol")
	}
	if sym.Name != "warmUp" {
		t.Errorf("referenced symbol = %q, want warmUp", sym.Name)
	}
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		t.Errorf("referenced declaration = %T, want *ast.Usage", sym.Decl)
	}
}

// findEntryMember returns the first entry member declared under n, descending
// through the namespace/definition/usage members that can enclose one.
func findEntryMember(n ast.Node) *ast.EntryMember {
	var members []ast.Node
	switch v := n.(type) {
	case *ast.EntryMember:
		return v
	case *ast.RootNamespace:
		members = v.Members
	case *ast.Membership:
		members = []ast.Node{v.Member}
	case *ast.Package:
		members = v.Members
	case *ast.Definition:
		members = v.Members
	case *ast.Usage:
		members = v.Members
	}
	for _, m := range members {
		if entry := findEntryMember(m); entry != nil {
			return entry
		}
	}
	return nil
}
