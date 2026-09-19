package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestDefinitionJumpsToDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Absolute name so uri.File(name).Filename() round-trips back to name.
	name := uri.File("/tmp/def.sysml").Filename()
	src := "package P { namespace N; }\nimport P::N;"
	ws.Open(name, []byte(src), 1)

	// Cursor on the "N" inside "import P::N;".
	off := strings.LastIndex(src, "N")
	pos := offsetToPosition([]byte(src), off)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if locs[0].URI != uri.File(name) {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File(name))
	}
	// Declaration "N" is on line 0.
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("decl line = %d, want 0", locs[0].Range.Start.Line)
	}
}

// Go-to-definition on a body-expression parameter must land on the parameter's
// own identifier, not on the body's brace or the same-named outer feature; on
// the declaration itself there is nothing to jump to.
func TestDefinitionBodyExpressionParameter(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_bodyparam.sysml").Filename()
	src := `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	attribute s : Integer = 1;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
	}
}
`
	ws.Open(name, []byte(src), 1)

	definitionAt := func(anchor string) []protocol.Location {
		t.Helper()
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), strings.Index(src, anchor)),
			},
		})
		if err != nil {
			t.Fatalf("Definition err = %v", err)
		}
		return locs
	}

	locs := definitionAt("s > 0")
	if len(locs) != 1 {
		t.Fatalf("locations from use = %d, want 1", len(locs))
	}
	if got, want := positionToOffset([]byte(src), locs[0].Range.Start), strings.Index(src, "s : Real; s > 0"); got != want {
		t.Errorf("definition offset = %d, want %d (the parameter's own name)", got, want)
	}

	if locs := definitionAt("s : Real; s > 0"); len(locs) != 0 {
		t.Errorf("definition on the declaration returned %d locations, want 0", len(locs))
	}
}

func TestDefinitionCrossFile(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// Two docs under distinct absolute round-trip names.
	libName := uri.File("/tmp/lib.sysml").Filename()
	useName := uri.File("/tmp/use.sysml").Filename()
	ws.Open(libName, []byte("package P { namespace N; }"), 1)
	useSrc := "import P::N;"
	ws.Open(useName, []byte(useSrc), 1)

	// Cursor on the "N" inside "import P::N;" in the using doc.
	off := strings.LastIndex(useSrc, "N")
	pos := offsetToPosition([]byte(useSrc), off)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(useName)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	// The declaring file is libName, not the requesting useName.
	if locs[0].URI != uri.File(libName) {
		t.Errorf("URI = %q, want %q", locs[0].URI, uri.File(libName))
	}
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("decl line = %d, want 0", locs[0].Range.Start.Line)
	}
}

// The name in a perform statement names the action performed, not the perform
// statement — which now carries that name too (symbols.effectiveIdent).
func TestDefinitionPerformReference(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_perform.sysml").Filename()
	src := `package P {
	action providePower;
	part vehicle {
		perform providePower;
	}
}
`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "providePower")
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
	// `action providePower;` is on line 1; the perform statement is on line 3.
	if locs[0].Range.Start.Line != 1 {
		t.Errorf("decl line = %d, want 1 (the action, not the perform statement)", locs[0].Range.Start.Line)
	}
}

// The last segment of `perform providePower.generateTorque;` names a member of
// the referenced action, not of the enclosing part — where the perform statement
// binds that very name.
func TestDefinitionPerformChainMember(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_perform_chain.sysml").Filename()
	src := `package P {
	action providePower {
		action generateTorque;
	}
	part torqueGenerator {
		perform providePower.generateTorque;
	}
}
`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "generateTorque")
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
	// `action generateTorque;` is on line 2; the perform statement is on line 5.
	if locs[0].Range.Start.Line != 2 {
		t.Errorf("decl line = %d, want 2 (the action's member, not the perform statement)", locs[0].Range.Start.Line)
	}
}

// The feature a connector end attaches to is a feature of the connector's
// owner, so a name it shares with a sibling end names the owner's feature.
func TestDefinitionConnectorEndReferenceIsNotASiblingEnd(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_end_sibling.sysml").Filename()
	src := `package P {
	part def TireBead;
	connection def PressureSeat {
		end [1] part bead : TireBead;
		end [1] part rim : TireBead;
	}
	part wheelAssy {
		part outer : TireBead;
		part bead : TireBead;
		connection : PressureSeat connect
			bead references outer to
			rim references bead;
	}
}
`
	ws.Open(name, []byte(src), 1)

	off := strings.LastIndex(src, "references bead") + len("references ")
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
	// `part bead : TireBead;` is on line 8; the sibling end is on line 10.
	if locs[0].Range.Start.Line != 8 {
		t.Errorf("decl line = %d, want 8 (the part, not the sibling end)", locs[0].Range.Start.Line)
	}
}

// The first end of `connector eng to t;` and the `eng` a named end references
// both go to the featuring type's feature (KerML.xtext:836).
func TestDefinitionKerMLBinaryConnectorFirstEnd(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_kerml_first_end.kerml").Filename()
	src := `package P {
	class V {
		feature eng;
		feature t;
		connector eng to t;
		connector a ::> eng to t;
		connector [0..1] eng to [1..*] t;
	}
}
`
	ws.Open(name, []byte(src), 1)

	for _, probe := range []string{"connector eng to", "::> eng to", "[0..1] eng to"} {
		off := strings.Index(src, probe) + strings.Index(probe, "eng")
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), off),
			},
		})
		if err != nil {
			t.Fatalf("%s: Definition err = %v", probe, err)
		}
		if len(locs) != 1 {
			t.Fatalf("%s: locations = %d, want 1", probe, len(locs))
		}
		// `feature eng;` is on line 2.
		if locs[0].Range.Start.Line != 2 {
			t.Errorf("%s: decl line = %d, want 2 (the feature, not the connector)", probe, locs[0].Range.Start.Line)
		}
	}
}

// Both ends of a keyword-first Disjoining go to the element each names, the
// disjoined end and the disjoining type alike (KerML.xtext:426).
func TestDefinitionKerMLDisjoiningEnds(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_kerml_disjoining.kerml").Filename()
	src := `package P {
	classifier A;
	classifier B { feature next : B; }
	feature b : B;
	disjoining D disjoint A from B;
	disjoint b.next from A;
}
`
	ws.Open(name, []byte(src), 1)
	for _, d := range ws.Diagnostics(name) {
		t.Fatalf("the source reports %q", d.Message)
	}

	// Each probe is a unique snippet of src; the cursor lands on its last word.
	for _, tc := range []struct {
		probe string
		decl  uint32
	}{
		{"disjoint A", 1},
		{"A from B", 2},
		{"disjoint b", 3},
		{"b.next", 2},
		{"next from A", 1},
	} {
		off := strings.Index(src, tc.probe) + strings.LastIndexAny(tc.probe, " .") + 1
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), off),
			},
		})
		if err != nil {
			t.Fatalf("%s: Definition err = %v", tc.probe, err)
		}
		if len(locs) != 1 {
			t.Fatalf("%s: locations = %d, want 1", tc.probe, len(locs))
		}
		if locs[0].Range.Start.Line != tc.decl {
			t.Errorf("%s: decl line = %d, want %d", tc.probe, locs[0].Range.Start.Line, tc.decl)
		}
	}
}

// A name in a filter condition resolves through the imports of its own
// namespace, which the namespace's filters must not restrict — the diagnostics
// pass resolves it that way, and the editor has to agree or rename skips it.
func TestDefinitionMetadataTypeInsideAFilterCondition(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_filter.sysml").Filename()
	src := `package Lib {
	metadata def Safety;
	metadata def Other;
	part seatBelt;
}
package P {
	private import Lib::*[@Other];
	filter @Safety;
}`
	ws.Open(name, []byte(src), 1)
	for _, d := range ws.Diagnostics(name) {
		t.Fatalf("the source reports %q", d.Message)
	}

	// Line 1 declares Safety, line 2 Other; each is named again in P.
	for _, tc := range []struct {
		name string
		decl uint32
	}{{"Other", 2}, {"Safety", 1}} {
		pos := offsetToPosition([]byte(src), strings.LastIndex(src, tc.name))
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     pos,
			},
		})
		if err != nil {
			t.Fatalf("Definition(%s) err = %v", tc.name, err)
		}
		if len(locs) != 1 || locs[0].Range.Start.Line != tc.decl {
			t.Errorf("Definition(%s) = %+v, want its declaration on line %d", tc.name, locs, tc.decl)
		}
	}
}

// The feature a binding end references (`bind e1 ::> a = b;`) is defined by the
// owner's feature, whether the end is named or bare; the end name itself is
// the end's own declaration.
func TestDefinitionBindingConnectorEnds(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/def_binding_ends.sysml").Filename()
	src := `package P {
	part def V {
		attribute a;
		attribute b;
		bind a = b;
		bind e1 ::> a = e2 references b;
		binding bb bind [1] e3 ::> a = b;
	}
}
`
	ws.Open(name, []byte(src), 1)

	for _, probe := range []string{"bind a = b", "e1 ::> a", "[1] e3 ::> a"} {
		off := strings.Index(src, probe) + strings.LastIndex(probe, "a")
		locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
				Position:     offsetToPosition([]byte(src), off),
			},
		})
		if err != nil {
			t.Fatalf("%s: Definition err = %v", probe, err)
		}
		if len(locs) != 1 {
			t.Fatalf("%s: locations = %d, want 1", probe, len(locs))
		}
		// `attribute a;` is on line 2.
		if locs[0].Range.Start.Line != 2 {
			t.Errorf("%s: decl line = %d, want 2 (the attribute, not the binding)", probe, locs[0].Range.Start.Line)
		}
	}
	off := strings.Index(src, "e2 references b") + len("e2 references ")
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("references b: Definition err = %v", err)
	}
	if len(locs) != 1 || locs[0].Range.Start.Line != 3 {
		t.Errorf("references b: locations = %v, want the attribute b on line 3", locs)
	}
}
