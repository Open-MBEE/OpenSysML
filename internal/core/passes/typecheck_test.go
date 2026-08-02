package passes

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// typeDiags parses src, indexes it, runs the full default registry, and returns
// only diagnostics whose Source is "type".
func typeDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx := symbols.NewIndex()
	idx.AddDocument("<t>", root)
	all := Analyze("<t>", root, nil, idx)
	var out []Diagnostic
	for _, d := range all {
		if d.Source == "type" {
			out = append(out, d)
		}
	}
	return out
}

func TestTypeCheckSpecializesSameKindOK(t *testing.T) {
	diags := typeDiags(t, "part def Vehicle; part def Car specializes Vehicle;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckSpecializesCrossKindError(t *testing.T) {
	diags := typeDiags(t, "attribute def Mass; part def Car specializes Mass;")
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Code != "type" {
		t.Fatalf("expected code %q, got %q", "type", diags[0].Code)
	}
}

func TestTypeCheckTypingWantsMatchingDef(t *testing.T) {
	// Structural kinds (part/attribute/item/occurrence) can cross-type
	// This is now allowed for compatibility
	diags := typeDiags(t, "attribute def Mass; part def Car { part p : Mass; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics (structural cross-typing allowed), got %v", diags)
	}
}

func TestTypeCheckTypingMatchingDefOK(t *testing.T) {
	diags := typeDiags(t, "part def Engine; part def Car { part e : Engine; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckUnresolvedTargetSkipped(t *testing.T) {
	diags := typeDiags(t, "part def Car specializes Missing;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics (gated), got %v", diags)
	}
}
