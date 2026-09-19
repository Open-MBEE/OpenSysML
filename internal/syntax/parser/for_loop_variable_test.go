package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseForLoop parses an action body holding a single for loop and returns it.
func parseForLoop(t *testing.T, member string) *ast.WhileLoopActionNode {
	t.Helper()
	sf := source.New("for_loop.sysml", []byte("action def A { "+member+" }"))
	p := New(sf)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	def, ok := file.Members[0].(*ast.Membership).Member.(*ast.Definition)
	if !ok {
		t.Fatalf("expected a definition, got %T", file.Members[0].(*ast.Membership).Member)
	}
	for _, m := range def.Members {
		if loop, ok := m.(*ast.WhileLoopActionNode); ok {
			return loop
		}
	}
	t.Fatalf("no loop parsed from %q", member)
	return nil
}

// The loop variable is a usage declaration (SysML.xtext ForVariableDeclaration),
// so it names itself as any declaration does — a keyword may serve as the name.
func TestForLoopVariableMayBeKeyword(t *testing.T) {
	for _, name := range []string{"step", "item", "part", "state", "end", "action", "snapshot", "event", "i"} {
		t.Run(name, func(t *testing.T) {
			loop := parseForLoop(t, "for "+name+" in c { action inner; }")
			if loop.Kind != ast.LoopFor {
				t.Errorf("kind = %v, want %v", loop.Kind, ast.LoopFor)
			}
			if loop.Variable.Name != name {
				t.Errorf("variable = %q, want %q", loop.Variable.Name, name)
			}
			if loop.Collection == nil {
				t.Error("collection not parsed")
			}
		})
	}
}

// A ForVariableDeclaration is a UsageDeclaration, whose Identification admits a
// short name with no name after it.
func TestForLoopVariableMayBeShortNameOnly(t *testing.T) {
	loop := parseForLoop(t, "for <v> in c { action inner; }")
	if loop.Variable.ShortName != "v" {
		t.Errorf("short name = %q, want %q", loop.Variable.ShortName, "v")
	}
	if loop.Variable.Name != "" {
		t.Errorf("name = %q, want empty", loop.Variable.Name)
	}
	if loop.Collection == nil {
		t.Error("collection not parsed")
	}
}

// A keyword used as a name is still reported, so the author learns the quoted
// spelling is the well-formed one. `step` is a KerML.xtext literal only, so in
// a SysML source it is an ordinary name and must not be warned about.
func TestForLoopKeywordVariableWarns(t *testing.T) {
	for _, tc := range []struct {
		variable string
		want     bool
	}{{"state", true}, {"step", false}} {
		sf := source.New("for_loop.sysml", []byte("action def A { for "+tc.variable+" in c { action inner; } }"))
		p := New(sf)
		p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("%s: unexpected parse errors: %v", tc.variable, p.Diagnostics)
		}
		var found bool
		for _, w := range p.Warnings {
			if w.Code == codeReservedKeywordName {
				found = true
			}
		}
		if found != tc.want {
			t.Errorf("for %s: %s warning = %t, want %t (warnings %v)", tc.variable, codeReservedKeywordName, found, tc.want, p.Warnings)
		}
	}
}

// A `for` with no variable yields an error node and a diagnostic, never a panic.
func TestForLoopMalformedRecovers(t *testing.T) {
	for _, input := range []string{
		"action def A { for in c { action inner; } }",
		"action def A { for }",
		"action def A { for ; }",
	} {
		t.Run(input, func(t *testing.T) {
			sf := source.New("for_loop.sysml", []byte(input))
			p := New(sf)
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned no tree")
			}
			if len(p.Diagnostics) == 0 {
				t.Errorf("expected a diagnostic for %q", input)
			}
		})
	}
}
