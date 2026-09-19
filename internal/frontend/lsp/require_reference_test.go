package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// The requirement named by `require Q::r` is a reference like any other, so the
// editor must jump to its declaration.
func TestDefinitionQualifiedRequireReference(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_require.sysml").Filename()
	src := `package P {
	requirement def FuelMass {
		requirement fuelMassRequirement;
	}
	analysis def A {
		objective o {
			require FuelMass::fuelMassRequirement { }
		}
	}
}`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "fuelMassRequirement")
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	// The declaration is on line 2 (0-based).
	if locs[0].Range.Start.Line != 2 {
		t.Errorf("decl line = %d, want 2", locs[0].Range.Start.Line)
	}
}

// The constraint usage an assume or require member owns is a declaration, so a
// `:>>` naming it jumps to the member.
func TestDefinitionOfRequirementConstraintMember(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_require_member.sysml").Filename()
	src := `package P {
	constraint def C;
	requirement def R {
		assume constraint assumed : C;
		require constraint required : C;
	}
	requirement def S :> R {
		assume constraint assumed2 :>> assumed;
		require constraint required2 :>> required;
	}
}`
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct {
		ref  string
		line uint32
	}{{":>> assumed", 3}, {":>> required", 4}} {
		off := strings.Index(src, tc.ref) + len(":>> ")
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), off),
			},
		})
		if err != nil {
			t.Fatalf("%s: Definition err = %v", tc.ref, err)
		}
		if len(locs) != 1 {
			t.Fatalf("%s: locations = %d, want 1", tc.ref, len(locs))
		}
		if locs[0].Range.Start.Line != tc.line {
			t.Errorf("%s: decl line = %d, want %d", tc.ref, locs[0].Range.Start.Line, tc.line)
		}
	}
}
