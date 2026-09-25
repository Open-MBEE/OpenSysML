package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A succession stating a guard between its ends is a TransitionUsage, not a
// connector (SysML.xtext:1719 GuardedSuccession).
func TestGuardedSuccessionIsATransition(t *testing.T) {
	sf := source.New("w8g.sysml", []byte(`action def A {
		attribute x = 1;
		action A1;
		action A2;
		succession S first A1 if x == 0 then A2;
		succession T first A1 then A2;
	}`))
	p := New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
	members := def.Members

	guarded, ok := members[len(members)-2].(*ast.Membership).Member.(*ast.TransitionMember)
	if !ok {
		t.Fatalf("guarded succession = %T, want *ast.TransitionMember", members[len(members)-2])
	}
	if guarded.Name != "S" || guarded.Guard == nil {
		t.Errorf("name=%q guard=%v, want S with a guard", guarded.Name, guarded.Guard)
	}
	if guarded.Source == nil || guarded.Source.Parts[0].Text != "A1" ||
		guarded.Target == nil || guarded.Target.Parts[0].Text != "A2" {
		t.Errorf("source=%v target=%v, want A1 then A2", guarded.Source, guarded.Target)
	}

	plain, ok := members[len(members)-1].(*ast.Membership).Member.(*ast.Usage)
	if !ok {
		t.Fatalf("unguarded succession = %T, want *ast.Usage", members[len(members)-1])
	}
	if plain.Kind != ast.UsageSuccession || len(plain.ConnectorEnds) != 2 {
		t.Errorf("kind=%v ends=%d, want a succession with two ends", plain.Kind, len(plain.ConnectorEnds))
	}
}
