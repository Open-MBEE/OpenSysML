package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func TestPassLevelString(t *testing.T) {
	cases := map[PassLevel]string{
		LevelSyntax:         "syntax",
		LevelNameResolution: "name-resolution",
		LevelType:           "type",
		LevelConstraint:     "constraint",
		PassLevel(999):      "unknown",
	}
	for lvl, want := range cases {
		if got := lvl.String(); got != want {
			t.Errorf("PassLevel(%d).String() = %q, want %q", int(lvl), got, want)
		}
	}
}

func TestPassLevelOrdering(t *testing.T) {
	if !(LevelSyntax < LevelNameResolution && LevelNameResolution < LevelType && LevelType < LevelConstraint) {
		t.Fatal("pass levels must be strictly increasing in dependency order")
	}
}

func TestNewContext(t *testing.T) {
	root := &ast.RootNamespace{}
	idx := newTestIndexFromDoc("d.sysml", root)
	ctx := NewContext("d.sysml", idx, []diag.Diagnostic{{Source: "syntax"}})
	if ctx.Name != "d.sysml" {
		t.Fatalf("Name = %q", ctx.Name)
	}
	if ctx.Index != idx {
		t.Fatal("Index not stored")
	}
	if len(ctx.ParseDiagnostics) != 1 || ctx.ParseDiagnostics[0].Source != "syntax" {
		t.Fatalf("ParseDiagnostics = %+v", ctx.ParseDiagnostics)
	}
	if ctx.Resolver() == nil {
		t.Fatal("Resolver() returned nil")
	}
	if first, second := ctx.Resolver(), ctx.Resolver(); first != second {
		t.Fatal("Resolver() must return the same shared instance")
	}
}
