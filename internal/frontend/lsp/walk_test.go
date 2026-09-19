package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// bodyRefSource uses `speed` from every expression position a behavioral body
// offers. The declaration plus 13 uses means rename must produce 14 edits and
// references must report 13.
const bodyRefSource = `package P {
	private import ScalarValues::*;
	attribute speed : Integer = 0;
	attribute bare : Integer = speed;
	attribute paren : Integer = (speed);
	attribute operand : Integer = speed + 1;
	calc plain { return r = speed; }
	calc def Plain { return r = speed; }
	constraint inRange { speed > 0 }
	requirement Req {
		assume constraint { speed > 0 }
		require constraint { speed < 100 }
	}
	action drive {
		attribute local : Integer = 0;
		assign local := speed;
	}
	state Machine {
		attribute seen : Integer = 0;
		entry; then i;
		state i;
		state running {
			entry { assign seen := speed; }
			do { assign seen := speed; }
			exit { assign seen := speed; }
		}
		state stopped;
		succession first i then running;
		transition first running if speed > 90 then stopped;
	}
}
`

const wantBodyRefUses = 13

func TestRenameEditsBareReferencesInBehaviorBodies(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/walk_rename.sysml", bodyRefSource)

	edits := renameEdits(t, ws, name, "speed :", "velocity")
	if len(edits) != wantBodyRefUses+1 {
		t.Fatalf("Rename produced %d edit(s), want %d (declaration + %d uses)",
			len(edits), wantBodyRefUses+1, wantBodyRefUses)
	}

	got, err := applyRename(t, ws, name, "speed :", "velocity")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if strings.Contains(got[name], "speed") {
		t.Errorf("speed still referenced after rename:\n%s", got[name])
	}
}

func TestReferencesFindsBareReferencesInBehaviorBodies(t *testing.T) {
	ws := model.NewWorkspace()
	name := openRenameDoc(t, ws, "/tmp/walk_refs.sysml", bodyRefSource)

	s := NewServer(ws)
	doc := ws.Document(name)
	off := strings.Index(string(doc.Content), "speed :")
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(doc.Content, off),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: false},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	if len(locs) != wantBodyRefUses {
		t.Fatalf("References found %d use(s), want %d:\n%v", len(locs), wantBodyRefUses, locs)
	}
}

// A bare trigger name is a signal injected by the event source, not a reference
// to the like-named declaration, so renaming that declaration must leave it be.
func TestRenameLeavesSignalTriggerNames(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	private import ScalarValues::*;
	attribute go : Integer = 0;
	state Machine {
		entry; then i;
		state i;
		state a;
		state b;
		succession first i then a;
		transition first a when go then b;
	}
}
`
	name := openRenameDoc(t, ws, "/tmp/walk_trigger.sysml", src)

	got, err := applyRename(t, ws, name, "go :", "start")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "attribute start : Integer = 0;") {
		t.Errorf("declaration not renamed:\n%s", got[name])
	}
	if !strings.Contains(got[name], "when go then b;") {
		t.Errorf("signal trigger name was rewritten:\n%s", got[name])
	}
}

// A connector end that declares its own name refers to the feature it
// reference-subsets, so renaming that feature must rewrite the reference and
// leave the end's name alone.
func TestRenameRewritesConnectorEndReferenceTarget(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	port def OutPort;
	port def InPort;
	interface def FuelInterface {
		end supplierPort : OutPort;
		end consumerPort : InPort;
	}
	part vehicle {
		part tankAssy { port fuelTankPort : OutPort; }
		part eng { port engineFuelPort : InPort; }
		interface : FuelInterface connect
			supplierPort ::> tankAssy.fuelTankPort to
			consumerPort ::> eng.engineFuelPort;
	}
}
`
	name := openRenameDoc(t, ws, "/tmp/walk_connectorend.sysml", src)

	got, err := applyRename(t, ws, name, "tankAssy {", "tank")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "supplierPort ::> tank.fuelTankPort") {
		t.Errorf("the feature the end attaches to was not renamed:\n%s", got[name])
	}

	got, err = applyRename(t, ws, name, "supplierPort : OutPort", "supply")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "end supply : OutPort;") {
		t.Errorf("the definition's end was not renamed:\n%s", got[name])
	}
}

// An explicit `:>>` on a connector end names an end of the connector's type, so
// renaming that end must rewrite the clause even in the plain-name spelling.
func TestRenameRewritesConnectorEndRedefinitionTarget(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	part def TireBead;
	connection def PressureSeat {
		end [1] part bead : TireBead;
		end [1] part rim;
	}
	part wheelAssy {
		part t { part bead : TireBead; }
		part w { part rim; }
		connection : PressureSeat connect
			seatRim :>> rim references w.rim to
			seatBead :>> bead references t.bead;
	}
}
`
	name := openRenameDoc(t, ws, "/tmp/walk_endredef.sysml", src)

	got, err := applyRename(t, ws, name, "rim;", "mountingRim")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "seatRim :>> mountingRim references w.rim") {
		t.Errorf("the redefinition target was not renamed:\n%s", got[name])
	}
}

// A body expression's parameter is its own declaration, so renaming a
// same-named outer feature must not rewrite the parameter's uses inside the
// body, while a name the body only reads from outside is still rewritten.
func TestRenameLeavesBodyExpressionParameters(t *testing.T) {
	ws := model.NewWorkspace()
	src := `package P {
	private import ScalarValues::*;
	private import ControlFunctions::*;
	attribute s : Integer = 1;
	action def Sample {
		in attribute samples : Real[*];
		assert constraint { samples->forAll { in s : Real; s > 0 } }
		assert constraint { samples->forAll { in x : Real; x > s } }
	}
}
`
	name := openRenameDoc(t, ws, "/tmp/walk_bodyexpr.sysml", src)

	got, err := applyRename(t, ws, name, "s : Integer", "threshold")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "attribute threshold : Integer = 1;") {
		t.Errorf("declaration not renamed:\n%s", got[name])
	}
	if !strings.Contains(got[name], "in s : Real; s > 0") {
		t.Errorf("body-expression parameter was rewritten:\n%s", got[name])
	}
	if !strings.Contains(got[name], "x > threshold") {
		t.Errorf("reference to the outer feature was not renamed:\n%s", got[name])
	}
}

// Renaming from a use of a body-expression parameter must edit the parameter's
// own declaration, not the body's opening brace, and must leave the same-named
// outer feature alone.
func TestRenameBodyExpressionParameterFromUse(t *testing.T) {
	ws := model.NewWorkspace()
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
	name := openRenameDoc(t, ws, "/tmp/walk_bodyparam.sysml", src)

	got, err := applyRename(t, ws, name, "s > 0", "sample")
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	if !strings.Contains(got[name], "in sample : Real; sample > 0") {
		t.Errorf("parameter declaration and use not both renamed:\n%s", got[name])
	}
	if !strings.Contains(got[name], "attribute s : Integer = 1;") {
		t.Errorf("outer feature was rewritten:\n%s", got[name])
	}
}
