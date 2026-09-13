package lsp

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	modeledit "github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// editModel is a document with comments and blank lines an edit must not touch.
const editModel = `package Vehicle {
	// The connectable units.
	part def Tank {
		port fuelOut : FuelPort;
	}
	part def Engine {
		port fuelIn : FuelPort;
	}
	port def FuelPort;

	part def Car {
		part tank : Tank; // holds the fuel
		part engine : Engine;
	}
}
`

// applyModelEdit is one opensysml/applyModelEdit request, decoded.
func applyModelEdit(t *testing.T, s *Server, docURI uri.URI, version int, ops ...modelEditOperation) *applyModelEditResult {
	t.Helper()
	raw, err := call(t, s, MethodApplyModelEdit, &applyModelEditParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Version:      version,
		Operations:   ops,
	})
	if err != nil {
		t.Fatalf("applyModelEdit: %v", err)
	}
	var out applyModelEditResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode applyModelEdit result: %v", err)
	}
	return &out
}

// applyWorkspaceEdit applies the edit to content the way a client does: each
// text edit at its range in the original text, later ranges first.
func applyWorkspaceEdit(t *testing.T, content string, edit *protocol.WorkspaceEdit, docURI uri.URI) string {
	t.Helper()
	if edit == nil {
		t.Fatal("no edit in result")
	}
	if len(edit.DocumentChanges) != 1 {
		t.Fatalf("documentChanges = %d, want 1", len(edit.DocumentChanges))
	}
	change := edit.DocumentChanges[0]
	if change.TextDocument.URI != docURI {
		t.Errorf("edit uri = %s, want %s", change.TextDocument.URI, docURI)
	}
	if change.TextDocument.Version == nil {
		t.Error("edit names no document version")
	}
	edits := append([]protocol.TextEdit(nil), change.Edits...)
	sort.Slice(edits, func(i, j int) bool {
		a, b := edits[i].Range.Start, edits[j].Range.Start
		return a.Line > b.Line || (a.Line == b.Line && a.Character > b.Character)
	})
	out := []byte(content)
	for _, e := range edits {
		start := positionToOffset([]byte(content), e.Range.Start)
		end := positionToOffset([]byte(content), e.Range.End)
		out = append(out[:start], append([]byte(e.NewText), out[end:]...)...)
	}
	return string(out)
}

// golden is what the edit layer itself produces for the operations, which the
// WorkspaceEdit must reproduce byte for byte.
func golden(t *testing.T, s *Server, name string, ops ...modelEditOperation) string {
	t.Helper()
	converted := make([]modeledit.Operation, 0, len(ops))
	for _, op := range ops {
		c, err := op.operation()
		if err != nil {
			t.Fatalf("convert %+v: %v", op, err)
		}
		converted = append(converted, c)
	}
	result, _, ok, err := s.ws.ApplyEdit(name, converted)
	if !ok || err != nil {
		t.Fatalf("edit.Apply: ok=%v err=%v", ok, err)
	}
	return string(result.Content)
}

func TestApplyModelEditAddsMemberAsWorkspaceEdit(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	op := modelEditOperation{Kind: EditAddMember, Owner: "Vehicle::Car", MemberKind: "part", Name: "battery", Type: "Tank"}
	out := applyModelEdit(t, s, docURI, 1, op)
	if out.Stale || out.Refused != nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	want := golden(t, s, docURI.Filename(), op)
	if got != want {
		t.Errorf("applied edit:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(got, "\t\tpart engine : Engine;\n\t\tpart battery : Tank;\n\t}") {
		t.Errorf("member not inserted at the end of Car's body:\n%s", got)
	}
	if !strings.Contains(got, "// holds the fuel") || !strings.Contains(got, "// The connectable units.") {
		t.Errorf("comments did not survive:\n%s", got)
	}
	if out.Version != 1 {
		t.Errorf("version = %d, want 1", out.Version)
	}
}

func TestApplyModelEditAddsConnection(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	op := modelEditOperation{Kind: EditAddConnection, Owner: "Vehicle::Car", MemberKind: "connection",
		From: "tank.fuelOut", To: "engine.fuelIn", Name: "fuelLine"}
	out := applyModelEdit(t, s, docURI, 1, op)
	if out.Edit == nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	if want := golden(t, s, docURI.Filename(), op); got != want {
		t.Errorf("applied edit:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(got, "\t\tconnection fuelLine connect tank.fuelOut to engine.fuelIn;\n") {
		t.Errorf("connection not written:\n%s", got)
	}
}

// A connection between ports whose names hold `::` is spelled from the names
// the rendering itself hands the client, as the client does: the owner is the
// parent's fqn, each endpoint the node's own name.
func TestApplyModelEditConnectsRenderedNamesHoldingTheSeparator(t *testing.T) {
	const src = "part 'x::y' {\n    port 'fuel::out';\n    port sink;\n}\n"
	s, docURI := renderServer(t, "sep.sysml", src)
	res := render(t, s, docURI, "#tree")
	var owner renderNode
	ports := map[string]renderNode{}
	for _, n := range res.Nodes {
		switch n.Kind {
		case "part":
			owner = n
		case "port":
			ports[n.Name] = n
		}
	}
	if owner.Name != "'x::y'" || owner.FQN != "'x::y'" || len(owner.Owners) != 0 {
		t.Fatalf("part rendered as %+v, want name and fqn 'x::y' with no owner", owner)
	}
	from, ok := ports["'fuel::out'"]
	if !ok || from.Parent != owner.ID || from.FQN != "'x::y'::'fuel::out'" {
		t.Fatalf("ports rendered as %v, want 'fuel::out' under the part", ports)
	}
	if want := []renderOwner{{FQN: "'x::y'", Feature: true}}; !reflect.DeepEqual(from.Owners, want) {
		t.Fatalf("port owners = %+v, want %+v", from.Owners, want)
	}
	op := modelEditOperation{Kind: EditAddConnection, Owner: owner.FQN, MemberKind: "connection",
		From: from.Name, To: ports["sink"].Name, Name: "'fuel::line'"}
	out := applyModelEdit(t, s, docURI, 1, op)
	if out.Edit == nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, src, out.Edit, docURI)
	if !strings.Contains(got, "    connection 'fuel::line' connect 'fuel::out' to sink;\n") {
		t.Errorf("connection not written:\n%s", got)
	}
}

// A name holding `::` and a nested name spelling the same joined text are two
// nodes with two fqns, each of which targets its own declaration.
func TestRenderTellsNamesHoldingTheSeparatorFromNestedOnes(t *testing.T) {
	const src = "part def 'x::y';\npackage x {\n    part def y;\n}\n"
	s, docURI := renderServer(t, "collide.sysml", src)
	res := render(t, s, docURI, "#tree")
	fqns := map[string]renderNode{}
	for _, n := range res.Nodes {
		fqns[n.FQN] = n
	}
	whole, nested := fqns["'x::y'"], fqns["x::y"]
	if whole.Name != "'x::y'" || len(whole.Owners) != 0 || nested.Name != "x::y" ||
		!reflect.DeepEqual(nested.Owners, []renderOwner{{FQN: "x"}}) {
		t.Fatalf("nodes rendered as %+v, want 'x::y' and x::y apart", res.Nodes)
	}
	for _, tc := range []struct{ node, want string }{{whole.FQN, "part def Whole;\npackage x {\n    part def y;"}, {nested.FQN, "part def 'x::y';\npackage x {\n    part def Nested;"}} {
		newName := "Whole"
		if tc.node == nested.FQN {
			newName = "Nested"
		}
		out := applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditRename, Target: tc.node, NewName: newName})
		if out.Edit == nil {
			t.Fatalf("rename %s: result = %+v, want an edit", tc.node, out)
		}
		if got := applyWorkspaceEdit(t, src, out.Edit, docURI); !strings.HasPrefix(got, tc.want) {
			t.Errorf("rename %s wrote:\n%s\nwant a prefix of:\n%s", tc.node, got, tc.want)
		}
	}
}

// A view exposing two features apart from the declaration holding them draws
// them as roots; their owners still name that declaration, so a connection
// between them is written there, spelled from its scope.
func TestRenderOwnersReachTheDeclarationTheViewLeavesOut(t *testing.T) {
	const src = "package Vehicle {\n    part def Car {\n        part tank { port fuelOut; }\n        part engine { port fuelIn; }\n    }\n    package Views {\n        view ports {\n            expose Vehicle::Car::tank;\n            expose Vehicle::Car::engine;\n        }\n    }\n}\n"
	s, docURI := renderServer(t, "siblings.sysml", src)
	res := render(t, s, docURI, "Vehicle::Views::ports")
	fqns := map[string]renderNode{}
	for _, n := range res.Nodes {
		fqns[n.FQN] = n
	}
	fuelOut, fuelIn := fqns["Vehicle::Car::tank::fuelOut"], fqns["Vehicle::Car::engine::fuelIn"]
	if fuelOut.ID == "" || fuelIn.ID == "" || fqns["Vehicle::Car"].ID != "" {
		t.Fatalf("nodes rendered as %+v, want the ports without Car", res.Nodes)
	}
	if fqns[fuelOut.Owners[0].FQN].Parent != "" {
		t.Fatalf("tank rendered as %+v, want a root", fqns[fuelOut.Owners[0].FQN])
	}
	want := []renderOwner{{FQN: "Vehicle::Car::tank", Feature: true}, {FQN: "Vehicle::Car"}, {FQN: "Vehicle"}}
	if !reflect.DeepEqual(fuelOut.Owners, want) {
		t.Fatalf("fuelOut owners = %+v, want %+v", fuelOut.Owners, want)
	}
	if fuelIn.Owners[1] != want[1] {
		t.Fatalf("fuelIn owners = %+v, want Car second", fuelIn.Owners)
	}
	op := modelEditOperation{Kind: EditAddConnection, Owner: want[1].FQN, MemberKind: "connection",
		From: "tank.fuelOut", To: "engine.fuelIn", Name: "fuelLine"}
	out := applyModelEdit(t, s, docURI, 1, op)
	if out.Edit == nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	if got := applyWorkspaceEdit(t, src, out.Edit, docURI); !strings.Contains(got, "        part engine { port fuelIn; }\n        connection fuelLine connect tank.fuelOut to engine.fuelIn;\n    }\n") {
		t.Errorf("connection not written in Car:\n%s", got)
	}
}

func TestApplyModelEditRenamesEveryReference(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	op := modelEditOperation{Kind: EditRename, Target: "Vehicle::Tank", NewName: "FuelTank"}
	out := applyModelEdit(t, s, docURI, 1, op)
	if out.Edit == nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	if want := golden(t, s, docURI.Filename(), op); got != want {
		t.Errorf("applied edit:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "Tank;") && !strings.Contains(got, "FuelTank;") {
		t.Errorf("reference not renamed:\n%s", got)
	}
	if n := len(out.Edit.DocumentChanges[0].Edits); n != 2 {
		t.Errorf("edits = %d, want one per changed line (declaration and usage)", n)
	}
}

func TestApplyModelEditDeletes(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	op := modelEditOperation{Kind: EditDelete, Target: "Vehicle::Car::engine"}
	out := applyModelEdit(t, s, docURI, 1, op)
	if out.Edit == nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	if want := golden(t, s, docURI.Filename(), op); got != want {
		t.Errorf("applied edit:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "part engine") {
		t.Errorf("engine not deleted:\n%s", got)
	}
}

// A delete whose target is referenced is refused without cascade and names the
// referents; with cascade it removes them too.
func TestApplyModelEditDeleteCascade(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	out := applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditDelete, Target: "Vehicle::Engine"})
	if len(out.Refused) != 1 || out.Edit != nil {
		t.Fatalf("result = %+v, want one refusal", out)
	}
	if r := out.Refused[0]; r.Failure != "delete-referenced" || r.Operation != 0 {
		t.Errorf("refusal = %+v, want delete-referenced of operation 0", r)
	}
	out = applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditDelete, Target: "Vehicle::Engine", Cascade: true})
	if out.Edit == nil {
		t.Fatalf("cascade result = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	if strings.Contains(got, "Engine") {
		t.Errorf("cascade left a reference:\n%s", got)
	}
}

// A move carries the declaration, its body and its comment into the new owner
// and respells the references the move breaks, as the edit layer does.
func TestApplyModelEditMoves(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	op := modelEditOperation{Kind: EditMove, Target: "Vehicle::Car::tank", Owner: "Vehicle::Engine"}
	out := applyModelEdit(t, s, docURI, 1, op)
	if out.Stale || out.Refused != nil || out.Edit == nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	if want := golden(t, s, docURI.Filename(), op); got != want {
		t.Errorf("applied edit:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(got, "\tpart def Engine {\n\t\tport fuelIn : FuelPort;\n\t\tpart tank : Tank; // holds the fuel\n\t}") {
		t.Errorf("tank not moved to the end of Engine's body with its comment:\n%s", got)
	}
	if strings.Contains(got, "\tpart def Car {\n\t\tpart tank") {
		t.Errorf("tank still declared in Car:\n%s", got)
	}
	if out.Version != 1 {
		t.Errorf("version = %d, want 1", out.Version)
	}
}

// A move to the document root and one into a bodyless owner both land where an
// addMember would put a new member.
func TestApplyModelEditMovesToRootAndOpensBody(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	out := applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditMove, Target: "Vehicle::Car", Owner: ""})
	if out.Edit == nil {
		t.Fatalf("root move = %+v, want an edit", out)
	}
	got := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	if !strings.HasSuffix(got, "}\npart def Car {\n\tpart tank : Vehicle::Tank; // holds the fuel\n\tpart engine : Vehicle::Engine;\n}\n") {
		t.Errorf("Car not moved after the package with its references qualified:\n%s", got)
	}
	out = applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditMove, Target: "Vehicle::Engine::fuelIn", Owner: "Vehicle::FuelPort"})
	if out.Edit == nil {
		t.Fatalf("bodyless move = %+v, want an edit", out)
	}
	got = applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	if !strings.Contains(got, "\tpart def Engine {\n\t}\n\tport def FuelPort {\n\t\tport fuelIn : FuelPort;\n\t}\n") {
		t.Errorf("FuelPort not given a body holding fuelIn:\n%s", got)
	}
}

// A move is refused, with the edit layer's failure name, when the owner is the
// target or inside it, when it would clash, and when another document refers to
// the target; the document is left as it was.
func TestApplyModelEditRefusesImpossibleMoves(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	for _, tc := range []struct {
		target, owner, failure string
	}{
		{"Vehicle::Car", "Vehicle::Car", "owner-inside-target"},
		{"Vehicle::Car", "Vehicle::Car::tank", "owner-inside-target"},
		{"Vehicle::Car::engine", "Vehicle::Nowhere", "owner-unknown"},
		{"Vehicle::Nobody", "Vehicle::Car", "unknown-target"},
	} {
		out := applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditMove, Target: tc.target, Owner: tc.owner})
		if len(out.Refused) != 1 || out.Edit != nil {
			t.Fatalf("move %s into %q = %+v, want one refusal", tc.target, tc.owner, out)
		}
		if r := out.Refused[0]; r.Failure != tc.failure || r.Operation != 0 {
			t.Errorf("move %s into %q refused as %+v, want %s of operation 0", tc.target, tc.owner, r, tc.failure)
		}
	}
	clash := "package P {\n\tpart def A {\n\t\tpart x;\n\t}\n\tpart def B {\n\t\tpart x;\n\t}\n}\n"
	s, docURI = renderServer(t, "clash.sysml", clash)
	out := applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditMove, Target: "P::A::x", Owner: "P::B"})
	if len(out.Refused) != 1 || out.Refused[0].Failure != "member-name-taken" {
		t.Errorf("clashing move = %+v, want member-name-taken", out)
	}
	s, docURI = renderServer(t, "vehicle.sysml", editModel)
	fleetURI := uri.File("fleet.sysml")
	openDoc(t, s, fleetURI, "package Fleet {\n    part truck : Vehicle::Car;\n}\n")
	out = applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditMove, Target: "Vehicle::Car", Owner: "Vehicle::Tank"})
	if len(out.Refused) != 1 || out.Refused[0].Failure != "referenced-elsewhere" {
		t.Fatalf("move referenced from another document = %+v, want referenced-elsewhere", out)
	}
	if want := "Fleet::truck (" + fleetURI.Filename() + ")"; strings.Join(out.Refused[0].Referring, ",") != want {
		t.Errorf("referring = %v, want %q", out.Refused[0].Referring, want)
	}
}

// Every declared node reports the keyword it was declared with, which is the
// member kind a move asks the palette to admit.
func TestRenderNodesCarryNotation(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	r := render(t, s, docURI, "#tree")
	notations := map[string]string{}
	for _, n := range r.Nodes {
		notations[n.FQN] = n.Notation
	}
	for fqn, want := range map[string]string{
		"Vehicle::FuelPort":      "port def",
		"Vehicle::Car":           "part def",
		"Vehicle::Car::tank":     "part",
		"Vehicle::Tank::fuelOut": "port",
	} {
		if notations[fqn] != want {
			t.Errorf("notation of %s = %q, want %q", fqn, notations[fqn], want)
		}
	}
}

// A delete or rename that another open document refers to is refused, whether
// or not it cascades: the edit rewrites one document, so the other would break.
func TestApplyModelEditRefusesWhatAnotherDocumentRefersTo(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	fleetURI := uri.File("fleet.sysml")
	openDoc(t, s, fleetURI, "package Fleet {\n    part truck : Vehicle::Car;\n}\n")
	for _, op := range []modelEditOperation{
		{Kind: EditDelete, Target: "Vehicle::Car"},
		{Kind: EditDelete, Target: "Vehicle::Car", Cascade: true},
		{Kind: EditRename, Target: "Vehicle::Car", NewName: "Auto"},
	} {
		out := applyModelEdit(t, s, docURI, 1, op)
		if len(out.Refused) != 1 || out.Edit != nil {
			t.Fatalf("%+v: result = %+v, want one refusal", op, out)
		}
		r := out.Refused[0]
		if r.Failure != "referenced-elsewhere" || r.Operation != 0 {
			t.Errorf("%+v: refusal = %+v, want referenced-elsewhere of operation 0", op, r)
		}
		if want := []string{"Fleet::truck (" + fleetURI.Filename() + ")"}; strings.Join(r.Referring, ",") != strings.Join(want, ",") {
			t.Errorf("%+v: referring = %v, want %v", op, r.Referring, want)
		}
	}
}

func TestApplyModelEditRejectsStaleVersion(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	out := applyModelEdit(t, s, docURI, 7, modelEditOperation{Kind: EditAddMember, Owner: "Vehicle::Car", MemberKind: "part", Name: "b", Type: "Tank"})
	if !out.Stale || out.Edit != nil || out.Refused != nil {
		t.Fatalf("result = %+v, want stale", out)
	}
	if out.Version != 1 {
		t.Errorf("version = %d, want the server's 1", out.Version)
	}
}

// A client pins an action to the version of the rendering it was taken on. When
// the document has since replaced the target with a namesake, the request is
// stale and nothing is written; only an action taken on the redrawn diagram is.
func TestApplyModelEditPinnedToRenderingRejectsNamesakeReplacement(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	drawn := render(t, s, docURI, "#interconnection:Vehicle::Car")
	if drawn.Version != 1 {
		t.Fatalf("render version = %d, want 1", drawn.Version)
	}
	var target string
	for _, n := range drawn.Nodes {
		if n.Name == "tank" {
			target = n.FQN
		}
	}
	if target == "" {
		t.Fatal("rendering has no tank node")
	}

	replaced := strings.Replace(editModel, "part tank : Tank; // holds the fuel", "part tank : Engine;", 1)
	encoded, _ := json.Marshal(map[string]string{"text": replaced})
	sendDidChange(t, s, docURI, 2, []json.RawMessage{encoded})

	out := applyModelEdit(t, s, docURI, drawn.Version, modelEditOperation{Kind: EditDelete, Target: target})
	if !out.Stale || out.Edit != nil || out.Refused != nil {
		t.Fatalf("result at the rendering's version = %+v, want stale", out)
	}
	if got := string(s.ws.Document(docURI.Filename()).Content); got != replaced {
		t.Error("server document changed on a stale request")
	}

	redrawn := render(t, s, docURI, "#interconnection:Vehicle::Car")
	if redrawn.Version != 2 {
		t.Fatalf("redrawn version = %d, want 2", redrawn.Version)
	}
	out = applyModelEdit(t, s, docURI, redrawn.Version, modelEditOperation{Kind: EditDelete, Target: target})
	if out.Stale || out.Refused != nil || out.Edit == nil {
		t.Fatalf("result at the redrawn version = %+v, want an edit", out)
	}
	if got := applyWorkspaceEdit(t, replaced, out.Edit, docURI); strings.Contains(got, "part tank") {
		t.Errorf("the redrawn action left tank in place:\n%s", got)
	}
}

// A refused operation is reported with its index and the edit layer's failure
// name; the request as a whole applies nothing, so an earlier valid operation
// yields no edit either.
func TestApplyModelEditRefusalShapeIsAtomic(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	out := applyModelEdit(t, s, docURI, 1,
		modelEditOperation{Kind: EditAddMember, Owner: "Vehicle::Car", MemberKind: "part", Name: "spare", Type: "Tank"},
		modelEditOperation{Kind: EditAddMember, Owner: "Vehicle::Nowhere", MemberKind: "part", Name: "x"},
	)
	if out.Edit != nil || out.Stale {
		t.Fatalf("result = %+v, want a refusal", out)
	}
	if len(out.Refused) != 1 {
		t.Fatalf("refused = %+v, want one", out.Refused)
	}
	r := out.Refused[0]
	if r.Operation != 1 || r.Failure != "owner-unknown" || !strings.Contains(r.Message, "Vehicle::Nowhere") {
		t.Errorf("refusal = %+v", r)
	}
	if got := string(s.ws.Document(docURI.Filename()).Content); got != editModel {
		t.Error("server document changed on a refused request")
	}
}

// A member whose type does not resolve is refused with the diagnostic the
// edited notation would have had, located in the refused text.
func TestApplyModelEditRefusalCarriesDiagnostics(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	out := applyModelEdit(t, s, docURI, 1,
		modelEditOperation{Kind: EditAddMember, Owner: "Vehicle::Car", MemberKind: "part", Name: "w", Type: "Wheel"})
	if len(out.Refused) != 1 {
		t.Fatalf("result = %+v, want one refusal", out)
	}
	r := out.Refused[0]
	if r.Failure != "result-invalid" || len(r.Diagnostics) == 0 {
		t.Fatalf("refusal = %+v, want result-invalid with diagnostics", r)
	}
	if !strings.Contains(r.Diagnostics[0].Message, "Wheel") {
		t.Errorf("diagnostic = %q, want it to name Wheel", r.Diagnostics[0].Message)
	}
}

func TestApplyModelEditRejectsUnknownKind(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	_, err := call(t, s, MethodApplyModelEdit, &applyModelEditParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Version:      1,
		Operations:   []modelEditOperation{{Kind: "explode", Target: "Vehicle"}},
	})
	if err == nil || !strings.Contains(err.Error(), "explode") {
		t.Errorf("err = %v, want an invalid-params error naming the kind", err)
	}
}

// The server does not write: the edit is the client's to apply, and the server
// learns of it as it learns of any change.
func TestApplyModelEditThenChangeRendersNewMember(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	out := applyModelEdit(t, s, docURI, 1, modelEditOperation{Kind: EditAddMember, Owner: "Vehicle::Car", MemberKind: "part", Name: "battery", Type: "Tank"})
	applied := applyWorkspaceEdit(t, editModel, out.Edit, docURI)
	encoded, _ := json.Marshal(map[string]string{"text": applied})
	sendDidChange(t, s, docURI, 2, []json.RawMessage{encoded})
	r := render(t, s, docURI, "#interconnection:Vehicle::Car")
	var names []string
	for _, n := range r.Nodes {
		names = append(names, n.Name)
	}
	if !contains(names, "battery") {
		t.Errorf("nodes after the edit = %v, want battery among them", names)
	}
	if r.Version != 2 {
		t.Errorf("render version = %d, want 2", r.Version)
	}
}

// Every node built from a declaration of the document carries the qualified
// name an edit targets it by, and the rendering offers the palette of its kind.
func TestRenderNodesCarryFQNAndPalette(t *testing.T) {
	s, docURI := renderServer(t, "vehicle.sysml", editModel)
	r := render(t, s, docURI, "#interconnection:Vehicle::Car")
	fqns := map[string]string{}
	for _, n := range r.Nodes {
		fqns[n.Name] = n.FQN
	}
	for name, want := range map[string]string{"Vehicle::Car": "Vehicle::Car", "tank": "Vehicle::Car::tank", "engine": "Vehicle::Car::engine"} {
		if fqns[name] != want {
			t.Errorf("fqn of %s = %q, want %q", name, fqns[name], want)
		}
	}
	if r.Palette == nil {
		t.Fatal("no palette for an interconnection rendering")
	}
	if !contains(r.Palette.Members, "part") || !contains(r.Palette.Members, "port") || contains(r.Palette.Members, "state") {
		t.Errorf("interconnection members = %v", r.Palette.Members)
	}
	if !contains(r.Palette.Connections, "connection") || contains(r.Palette.Connections, "transition") {
		t.Errorf("interconnection connections = %v", r.Palette.Connections)
	}
}

// Owners names, for each member only some bodies offer, the nodes that open one:
// `subject` for the requirement and use case, `objective` for the use case alone.
func TestRenderPaletteOwnersFollowTheDeclaration(t *testing.T) {
	src := "package Kit {\n\tpart def Widget;\n\trequirement def Fit;\n\tuse case def Assemble;\n}\n"
	s, docURI := renderServer(t, "kit.sysml", src)
	out := render(t, s, docURI, "#tree")
	ids := map[string]string{}
	for _, n := range out.Nodes {
		ids[n.Name] = n.ID
	}
	if out.Palette == nil || out.Palette.Owners == nil {
		t.Fatalf("palette = %+v, want owners", out.Palette)
	}
	for kind, want := range map[string][]string{
		"subject":     {"Kit::Fit", "Kit::Assemble"},
		"actor":       {"Kit::Fit", "Kit::Assemble"},
		"stakeholder": {"Kit::Fit"},
		"objective":   {"Kit::Assemble"},
	} {
		got := out.Palette.Owners[kind]
		if len(got) != len(want) {
			t.Errorf("owners of %s = %v, want %v", kind, got, want)
			continue
		}
		for _, name := range want {
			if !contains(got, ids[name]) {
				t.Errorf("owners of %s = %v lack %s (%s)", kind, got, name, ids[name])
			}
		}
		if contains(got, ids["Kit::Widget"]) || contains(got, ids["Kit"]) {
			t.Errorf("owners of %s = %v admit a part or package", kind, got)
		}
	}
	if _, ok := out.Palette.Owners["part"]; ok {
		t.Error("part is owner-bound")
	}
	// A state diagram offers no owner-bound member, so it lists no owners.
	if p := palette(view.KindState, source.KindSysML); p.Owners != nil {
		t.Errorf("state owners = %v", p.Owners)
	}
}

// A drawn declaration only some bodies offer (`entry action`) is listed in Owners
// under its notation, though no palette offers to add one, so a move of it is
// offered the bodies that take it.
func TestRenderPaletteOwnersCoverDrawnNotations(t *testing.T) {
	src := "package Ops {\n\tstate def Run {\n\t\tentry action boot;\n\t\tstate idle;\n\t}\n\tpart def Widget;\n}\n"
	s, docURI := renderServer(t, "ops.sysml", src)
	out := render(t, s, docURI, "#tree")
	ids := map[string]string{}
	for _, n := range out.Nodes {
		ids[n.FQN] = n.ID
	}
	got := out.Palette.Owners["entry action"]
	if len(got) != 1 || got[0] != ids["Ops::Run"] {
		t.Fatalf("owners of entry action = %v, want [%s] (Ops::Run)", got, ids["Ops::Run"])
	}
	if _, ok := out.Palette.Owners["state"]; ok {
		t.Error("state is owner-bound")
	}
}

func TestPaletteFollowsRenderingKind(t *testing.T) {
	for _, tc := range []struct {
		kind        view.Kind
		lang        source.Kind
		member, not string
		conn        string
	}{
		{view.KindState, source.KindSysML, "state", "part", "transition"},
		{view.KindAction, source.KindSysML, "action", "state", "succession"},
		{view.KindAction, source.KindKerML, "step", "action", "succession"},
		{view.KindInterconnection, source.KindKerML, "feature", "part", "connector"},
		{view.KindTree, source.KindSysML, "requirement def", "", "flow"},
	} {
		p := palette(tc.kind, tc.lang)
		if p == nil {
			t.Fatalf("%s/%s: no palette", tc.kind, tc.lang)
		}
		if !contains(p.Members, tc.member) || (tc.not != "" && contains(p.Members, tc.not)) {
			t.Errorf("%s/%s members = %v", tc.kind, tc.lang, p.Members)
		}
		if !contains(p.Connections, tc.conn) {
			t.Errorf("%s/%s connections = %v", tc.kind, tc.lang, p.Connections)
		}
		for _, typed := range p.Typed {
			if !contains(p.Members, typed) {
				t.Errorf("%s/%s typed %q is not a member", tc.kind, tc.lang, typed)
			}
		}
	}
	// Usages take a type, definitions and control nodes do not.
	p := palette(view.KindTree, source.KindSysML)
	if !contains(p.Typed, "part") || contains(p.Typed, "part def") || contains(p.Typed, "fork") {
		t.Errorf("tree/sysml typed = %v", p.Typed)
	}
	if p = palette(view.KindTree, source.KindKerML); !contains(p.Typed, "feature") || contains(p.Typed, "class") {
		t.Errorf("tree/kerml typed = %v", p.Typed)
	}
	if palette(view.KindGeometry, source.KindSysML) != nil {
		t.Error("an unsupported kind offers a palette")
	}
	// A table draws rows, not nodes, so there is no owner or endpoint to pick.
	if palette(view.KindTable, source.KindSysML) != nil {
		t.Error("a table offers a palette")
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
