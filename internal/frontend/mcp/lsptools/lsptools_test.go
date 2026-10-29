package lsptools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const vehicles = `package Vehicles {
    part def Wheel {
        attribute diameter : ScalarValues::Real = 17.0;
    }
    calc def Area { in r : ScalarValues::Real; r * r }
    part def Car {
        part wheels : Wheel[4];
        attribute mass : ScalarValues::Real = 1200.0;
        attribute count = 4;
        attribute footprint = Area(3.0);
    }
}
`

const fleet = `package Fleet {
    private import Vehicles::*;
    part myCar : Car {
        part frontLeft : Wheel;
    }
    part spare : Vehicles::Wheel;
}
`

// accents puts non-ASCII text before the names on a line, so a character
// column and a UTF-16 column differ there.
const accents = `package 'Größe' {
    part def Rad;
    /* Ø ∅ 😀 */ part rad : Rad;
}
`

const broken = `package Bad {
    part p : Nope;
}
`

type fixture struct {
	t    *testing.T
	root string
	host *Host
	cs   *mcp.ClientSession
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	write(t, root, "a.sysml", vehicles)
	write(t, root, "b.sysml", fleet)
	write(t, root, "c.sysml", accents)
	write(t, root, "bad.sysml", broken)
	ctx := context.Background()
	host, err := NewHost(ctx, root, model.NewWorkspace(), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close(ctx) })
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	Register(server, host)
	st, ct := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &fixture{t: t, root: root, host: host, cs: cs}
}

func write(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) read(name string) string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(b)
}

// call invokes a tool, decoding the structured content into out when the
// call succeeded, and returns the result for the error and text checks.
func (f *fixture) call(name string, args any, out any) *mcp.CallToolResult {
	f.t.Helper()
	res, err := f.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		f.t.Fatalf("%s: %v", name, err)
	}
	if res.IsError || out == nil {
		return res
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		f.t.Fatal(err)
	}
	// Decode into a zeroed value, so a field the result omits does not keep
	// what an earlier call left there.
	target := reflect.ValueOf(out).Elem()
	target.Set(reflect.Zero(target.Type()))
	if err := json.Unmarshal(raw, out); err != nil {
		f.t.Fatalf("%s: decoding %s: %v", name, raw, err)
	}
	return res
}

// ok invokes a tool that must succeed.
func (f *fixture) ok(name string, args any, out any) *mcp.CallToolResult {
	f.t.Helper()
	res := f.call(name, args, out)
	if res.IsError {
		f.t.Fatalf("%s(%v) failed: %s", name, args, textOf(res))
	}
	return res
}

// fails invokes a tool that must fail, returning the error text.
func (f *fixture) fails(name string, args any) string {
	f.t.Helper()
	res := f.call(name, args, nil)
	if !res.IsError {
		f.t.Fatalf("%s(%v) succeeded, want an error", name, args)
	}
	return textOf(res)
}

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

// snapshot is every file under the root with its text, to prove tools did not
// write.
func (f *fixture) snapshot() map[string]string {
	f.t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(f.root)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			out[e.Name()] = f.read(e.Name())
		}
	}
	return out
}

func (f *fixture) unchanged(before map[string]string) {
	f.t.Helper()
	after := f.snapshot()
	if len(after) != len(before) {
		f.t.Fatalf("files changed: %d before, %d after", len(before), len(after))
	}
	for name, text := range before {
		if after[name] != text {
			f.t.Errorf("%s was written without apply", name)
		}
	}
}

func pos(line, col int) toolkit.Position { return toolkit.Position{Line: line, Column: col} }

func TestDiagnosticsOfADocumentAndTheWorkspace(t *testing.T) {
	f := newFixture(t)
	var res DiagnosticsResult
	f.ok("workspace_diagnostics", map[string]any{"path": "bad.sysml"}, &res)
	if res.Errors == 0 || len(res.Diagnostics) == 0 {
		t.Fatalf("no error for the unresolved name: %+v", res)
	}
	d := res.Diagnostics[0]
	if d.Path != "bad.sysml" || d.Severity != "error" || d.Range.Start.Line != 2 || d.Range.Start.Column != 14 {
		t.Errorf("diagnostic = %+v, want bad.sysml:2:14 error", d)
	}
	if !strings.Contains(d.Message, "Nope") || d.Code == "" {
		t.Errorf("diagnostic lacks the name or a code: %+v", d)
	}

	var clean DiagnosticsResult
	res2 := f.ok("workspace_diagnostics", map[string]any{"path": "a.sysml"}, &clean)
	if clean.Errors != 0 {
		t.Errorf("a.sysml has errors: %s", textOf(res2))
	}

	var all DiagnosticsResult
	res3 := f.ok("workspace_diagnostics", map[string]any{}, &all)
	if all.Documents != 4 || all.Errors < 1 {
		t.Errorf("workspace diagnostics = %d documents, %d errors; want 4 and >= 1", all.Documents, all.Errors)
	}
	if !strings.Contains(textOf(res3), "bad.sysml:2:14: error:") {
		t.Errorf("text rendering = %q", textOf(res3))
	}
}

func TestUnknownPathsAndPositionsOffTheEndFail(t *testing.T) {
	f := newFixture(t)
	if msg := f.fails("workspace_diagnostics", map[string]any{"path": "missing.sysml"}); !strings.Contains(msg, "missing.sysml") {
		t.Errorf("error = %q", msg)
	}
	f.fails("hover", map[string]any{"path": "missing.sysml", "position": pos(1, 1)})
	if msg := f.fails("hover", map[string]any{"path": "a.sysml", "position": pos(999, 1)}); !strings.Contains(msg, "line 999") {
		t.Errorf("error = %q", msg)
	}
	if msg := f.fails("definition", map[string]any{"path": "a.sysml", "position": pos(1, 500)}); !strings.Contains(msg, "column 500") {
		t.Errorf("error = %q", msg)
	}
	f.fails("rename", map[string]any{"path": "a.sysml", "position": pos(2, 14), "newName": ""})
	f.fails("update_document", map[string]any{"path": "a.sysml", "text": "package X;"})
	f.fails("close_document", map[string]any{"path": "a.sysml"})
	f.fails("render_document", map[string]any{"name": "Nowhere::Doc"})
	f.fails("hover", map[string]any{"path": "sysml://stdlib/Nope.kerml", "position": pos(1, 1)})

	outside := filepath.Join(filepath.Dir(f.root), "outside.sysml")
	write(t, filepath.Dir(f.root), "outside.sysml", "package Outside;\n")
	t.Cleanup(func() { _ = os.Remove(outside) })
	for _, path := range []string{"../outside.sysml", outside} {
		if msg := f.fails("workspace_diagnostics", map[string]any{"path": path}); !strings.Contains(msg, "outside the workspace root") {
			t.Errorf("error for %s = %q", path, msg)
		}
	}
	f.fails("open_document", map[string]any{"path": "../outside.sysml", "text": "package X;"})
	if err := os.Symlink(outside, filepath.Join(f.root, "link.sysml")); err != nil {
		t.Fatal(err)
	}
	if msg := f.fails("workspace_diagnostics", map[string]any{"path": "link.sysml"}); !strings.Contains(msg, "outside the workspace root") {
		t.Errorf("error for a symlink out of the root = %q", msg)
	}
	f.fails("format", map[string]any{"path": "link.sysml", "apply": true})
	if _, err := f.cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "sysml://workspace/../outside.sysml"}); err == nil {
		t.Error("reading a resource outside the root succeeded")
	}
}

func TestNavigation(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()

	var hover HoverResult
	f.ok("hover", map[string]any{"path": "b.sysml", "position": pos(3, 18)}, &hover)
	if !strings.Contains(hover.Markdown, "Car") {
		t.Errorf("hover = %+v", hover)
	}

	var def LocationsResult
	f.ok("definition", map[string]any{"path": "b.sysml", "position": pos(3, 18)}, &def)
	if len(def.Locations) != 1 || def.Locations[0].Path != "a.sysml" || def.Locations[0].Range.Start.Line != 6 {
		t.Errorf("definition of Car = %+v, want a.sysml:6", def)
	}
	f.ok("definition", map[string]any{"path": "a.sysml", "position": pos(3, 44)}, &def)
	if len(def.Locations) != 1 || !strings.HasPrefix(def.Locations[0].Path, "sysml://stdlib/") {
		t.Errorf("definition of Real = %+v, want a stdlib resource", def)
	}
	var stdHover HoverResult
	f.ok("hover", map[string]any{"path": def.Locations[0].Path, "position": def.Locations[0].Range.Start}, &stdHover)
	if !strings.Contains(stdHover.Markdown, "Real") {
		t.Errorf("hover in the library = %+v", stdHover)
	}

	var refs LocationsResult
	f.ok("references", map[string]any{"path": "a.sysml", "position": pos(2, 14), "includeDeclaration": false}, &refs)
	paths := map[string]int{}
	for _, l := range refs.Locations {
		paths[l.Path]++
	}
	if paths["a.sysml"] != 1 || paths["b.sysml"] != 2 {
		t.Errorf("references to Wheel = %+v", refs.Locations)
	}

	var syms DocumentSymbolsResult
	f.ok("document_symbols", map[string]any{"path": "a.sysml"}, &syms)
	if len(syms.Symbols) != 1 || syms.Symbols[0].Name != "Vehicles" || len(syms.Symbols[0].Children) != 3 {
		t.Errorf("outline = %+v", syms)
	}
	if car := syms.Symbols[0].Children[2]; car.Name != "Car" || car.Range.Start.Line != 6 || len(car.Children) != 4 {
		t.Errorf("Car symbol = %+v", car)
	}

	var ws WorkspaceSymbolsResult
	f.ok("workspace_symbols", map[string]any{"query": "wheel"}, &ws)
	found := false
	for _, s := range ws.Symbols {
		if s.Name == "Wheel" && s.Location.Path == "a.sysml" && s.Location.Range.Start.Line == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("workspace symbols for wheel = %+v", ws)
	}
	f.ok("workspace_symbols", map[string]any{"query": "", "limit": 2}, &ws)
	if len(ws.Symbols) != 2 || ws.Total <= 2 {
		t.Errorf("limited symbols = %d of %d", len(ws.Symbols), ws.Total)
	}

	var comp CompletionResult
	f.ok("completion", map[string]any{"path": "b.sysml", "position": pos(6, 28)}, &comp)
	labels := map[string]bool{}
	for _, item := range comp.Items {
		labels[item.Label] = true
	}
	if !labels["Wheel"] || !labels["Car"] {
		t.Errorf("completion after Vehicles:: = %v", labels)
	}

	var sig SignatureHelpResult
	f.ok("signature_help", map[string]any{"path": "a.sysml", "position": pos(10, 36)}, &sig)
	if len(sig.Signatures) != 1 || !strings.Contains(sig.Signatures[0].Label, "Area") || len(sig.Signatures[0].Parameters) != 1 {
		t.Errorf("signature help = %+v", sig)
	}

	var hints InlayHintsResult
	f.ok("inlay_hints", map[string]any{"path": "a.sysml"}, &hints)
	hinted := false
	for _, h := range hints.Hints {
		if h.Position.Line == 9 && h.Kind == "type" {
			hinted = true
		}
	}
	if !hinted {
		t.Errorf("no type hint for count on line 9: %+v", hints)
	}
	f.ok("inlay_hints", map[string]any{"path": "a.sysml", "range": Range{Start: pos(1, 1), End: pos(2, 1)}}, &hints)
	if len(hints.Hints) != 0 {
		t.Errorf("hints in lines 1-2 = %+v", hints)
	}

	var actions CodeActionsResult
	f.ok("code_actions", map[string]any{"path": "bad.sysml", "range": Range{Start: pos(2, 1), End: pos(3, 1)}}, &actions)
	for _, a := range actions.Actions {
		if a.Title == "" {
			t.Errorf("action without a title: %+v", a)
		}
	}
	f.unchanged(before)
}

func TestNonASCIILinesConvertThroughUTF16(t *testing.T) {
	f := newFixture(t)
	// "    /* Ø ∅ 😀 */ part rad : Rad;": Rad starts at character column 28.
	var def LocationsResult
	f.ok("definition", map[string]any{"path": "c.sysml", "position": pos(3, 28)}, &def)
	if len(def.Locations) != 1 || def.Locations[0].Range.Start != pos(2, 5) || def.Locations[0].Range.End != pos(2, 18) {
		t.Fatalf("definition of Rad = %+v, want c.sysml:2:5-2:18", def)
	}
	var refs LocationsResult
	f.ok("references", map[string]any{"path": "c.sysml", "position": pos(2, 14)}, &refs)
	if len(refs.Locations) != 1 || refs.Locations[0].Range.Start != pos(3, 28) || refs.Locations[0].Range.End != pos(3, 31) {
		t.Errorf("reference to Rad = %+v, want 3:28-3:31", refs.Locations)
	}
	var hover HoverResult
	f.ok("hover", map[string]any{"path": "c.sysml", "position": pos(3, 22)}, &hover)
	if !strings.Contains(hover.Markdown, "rad") {
		t.Errorf("hover on rad = %+v", hover)
	}
	var ws WorkspaceSymbolsResult
	f.ok("workspace_symbols", map[string]any{"query": "Größe"}, &ws)
	if len(ws.Symbols) != 1 || ws.Symbols[0].Location.Range.Start != pos(1, 1) || ws.Symbols[0].Location.Range.End != pos(4, 2) {
		t.Errorf("Größe = %+v", ws)
	}
}

func TestRenameAcrossFiles(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	var res WorkspaceEditResult
	f.ok("rename", map[string]any{"path": "a.sysml", "position": pos(2, 14), "newName": "Tyre"}, &res)
	if res.Applied || len(res.Changes) != 2 {
		t.Fatalf("rename = %+v", res)
	}
	byPath := map[string]FileEdits{}
	for _, c := range res.Changes {
		byPath[c.Path] = c
	}
	if a := byPath["a.sysml"]; len(a.Edits) != 2 || a.Edits[0].Range.Start != pos(2, 14) || a.Edits[0].NewText != "Tyre" {
		t.Errorf("a.sysml edits = %+v", a.Edits)
	}
	if b := byPath["b.sysml"]; len(b.Edits) != 2 || b.Edits[0].Range.Start != pos(4, 26) || b.Edits[1].Range.Start != pos(6, 28) {
		t.Errorf("b.sysml edits = %+v", b.Edits)
	}
	f.unchanged(before)

	// An edit made with another tool after indexing must survive the apply and
	// be renamed with the rest.
	write(t, f.root, "b.sysml", fleet+"package More { part extra : Vehicles::Wheel; }\n")
	f.ok("rename", map[string]any{"path": "a.sysml", "position": pos(2, 14), "newName": "Tyre", "apply": true}, &res)
	if !res.Applied {
		t.Fatalf("rename with apply = %+v", res)
	}
	if a := f.read("a.sysml"); !strings.Contains(a, "part def Tyre {") || !strings.Contains(a, "part wheels : Tyre[4];") || strings.Contains(a, "Wheel") {
		t.Errorf("a.sysml after rename:\n%s", a)
	}
	if b := f.read("b.sysml"); !strings.Contains(b, "part frontLeft : Tyre;") || !strings.Contains(b, "Vehicles::Tyre;") || !strings.Contains(b, "part extra : Vehicles::Tyre;") {
		t.Errorf("b.sysml after rename:\n%s", b)
	}
	var diags DiagnosticsResult
	f.ok("workspace_diagnostics", map[string]any{}, &diags)
	for _, d := range diags.Diagnostics {
		if d.Path != "bad.sysml" {
			t.Errorf("diagnostic after rename: %+v", d)
		}
	}
	var hover HoverResult
	f.ok("hover", map[string]any{"path": "b.sysml", "position": pos(4, 26)}, &hover)
	if !strings.Contains(hover.Markdown, "Tyre") {
		t.Errorf("hover after rename = %+v", hover)
	}
}

func TestAFailedWriteLeavesNoFileChanged(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write into a read-only directory")
	}
	f := newFixture(t)
	// A third document in a folder that cannot be written into: the rename
	// reaches it last, after a.sysml and b.sysml were written, so those two
	// must be put back.
	sub := filepath.Join(f.root, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	write(t, sub, "more.sysml", "package More {\n    private import Vehicles::*;\n    part extra : Wheel;\n}\n")
	before := f.snapshot()
	moreBefore := f.read("sub/more.sysml")
	if err := os.Chmod(sub, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o750) })
	msg := f.fails("rename", map[string]any{"path": "a.sysml", "position": pos(2, 14), "newName": "Tyre", "apply": true})
	if !strings.Contains(msg, "sub/more.sysml") || !strings.Contains(msg, "no file was changed") {
		t.Errorf("error = %q", msg)
	}
	f.unchanged(before)
	if f.read("sub/more.sysml") != moreBefore {
		t.Error("sub/more.sysml changed")
	}
	var hover HoverResult
	f.ok("hover", map[string]any{"path": "b.sysml", "position": pos(4, 26)}, &hover)
	if !strings.Contains(hover.Markdown, "Wheel") {
		t.Errorf("index after the failed rename = %+v", hover)
	}
}

func TestFormatReturnsEditsAndWritesOnlyWithApply(t *testing.T) {
	f := newFixture(t)
	write(t, f.root, "ugly.sysml", "package Ugly {\npart def A {\n      attribute x : ScalarValues::Real;\n}\n}\n")
	before := f.snapshot()
	var res FormatResult
	f.ok("format", map[string]any{"path": "ugly.sysml"}, &res)
	if res.Applied || len(res.Edits) == 0 || !strings.Contains(res.Text, "    part def A {\n        attribute x") {
		t.Fatalf("format = %+v", res)
	}
	f.unchanged(before)
	f.ok("format", map[string]any{"path": "ugly.sysml", "range": Range{Start: pos(3, 1), End: pos(3, 1)}}, &res)
	if res.Applied || len(res.Edits) != 1 || res.Edits[0].Range.Start.Line != 3 {
		t.Errorf("range format = %+v", res)
	}
	f.unchanged(before)
	f.ok("format", map[string]any{"path": "ugly.sysml", "apply": true}, &res)
	if !res.Applied || f.read("ugly.sysml") != res.Text {
		t.Errorf("formatted file = %q, want %q", f.read("ugly.sysml"), res.Text)
	}
	f.ok("format", map[string]any{"path": "ugly.sysml", "apply": true}, &res)
	if res.Applied || len(res.Edits) != 0 {
		t.Errorf("second format = %+v", res)
	}

	// Writing through a symlink inside the root changes the target and keeps
	// the link.
	write(t, f.root, "target.sysml", "package Linked {\npart def L;\n}\n")
	if err := os.Symlink(filepath.Join(f.root, "target.sysml"), filepath.Join(f.root, "alias.sysml")); err != nil {
		t.Fatal(err)
	}
	f.ok("format", map[string]any{"path": "alias.sysml", "apply": true}, &res)
	if !res.Applied || f.read("target.sysml") != res.Text {
		t.Errorf("target through the link = %q, want %q", f.read("target.sysml"), res.Text)
	}
	if info, err := os.Lstat(filepath.Join(f.root, "alias.sysml")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("alias.sysml is no longer a symlink: %v %v", info, err)
	}
}

func TestApplyModelEdit(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	var res ModelEditResult
	ops := []map[string]any{{"kind": "setValue", "target": "Vehicles::Car::mass", "value": "1500.0"}}
	f.ok("apply_model_edit", map[string]any{"path": "a.sysml", "operations": ops}, &res)
	if res.Applied || len(res.Refused) != 0 || len(res.Changes) != 1 || res.Changes[0].Path != "a.sysml" || len(res.Changes[0].Edits) != 1 {
		t.Fatalf("setValue = %+v", res)
	}
	if e := res.Changes[0].Edits[0]; e.Range.Start.Line != 8 || e.Range.Start.Column < 47 || e.Range.End.Column > 53 {
		t.Errorf("edit range = %+v, want within the value on line 8", e.Range)
	}
	f.unchanged(before)
	f.ok("apply_model_edit", map[string]any{"path": "a.sysml", "operations": ops, "apply": true}, &res)
	if !res.Applied || !strings.Contains(f.read("a.sysml"), "attribute mass : ScalarValues::Real = 1500.0;") {
		t.Fatalf("setValue with apply = %+v:\n%s", res, f.read("a.sysml"))
	}

	two := []map[string]any{
		{"kind": "addMember", "owner": "Vehicles", "memberKind": "part def", "name": "Truck"},
		{"kind": "addMember", "owner": "Vehicles::Truck", "memberKind": "attribute", "name": "load", "type": "ScalarValues::Real"},
	}
	f.ok("apply_model_edit", map[string]any{"path": "a.sysml", "operations": two, "apply": true}, &res)
	if !res.Applied || len(res.Refused) != 0 {
		t.Fatalf("addMember = %+v (%s)", res, f.read("a.sysml"))
	}
	if a := f.read("a.sysml"); !strings.Contains(a, "part def Truck") || !strings.Contains(a, "attribute load : ScalarValues::Real;") {
		t.Errorf("a.sysml after addMember:\n%s", a)
	}
	var diags DiagnosticsResult
	f.ok("workspace_diagnostics", map[string]any{"path": "a.sysml"}, &diags)
	if diags.Errors != 0 {
		t.Errorf("errors after the edit: %+v", diags.Diagnostics)
	}

	write(t, f.root, "act.sysml", "package Acts {\n    action def Run {\n        attribute x : ScalarValues::Integer;\n    }\n}\n")
	seq := []map[string]any{{"kind": "addSequence", "owner": "Acts::Run", "memberKind": "if", "condition": "x > 0", "body": []map[string]any{
		{"kind": "addSequence", "memberKind": "assign", "target": "x", "value": "0"},
	}}}
	f.ok("apply_model_edit", map[string]any{"path": "act.sysml", "operations": seq}, &res)
	if len(res.Refused) != 0 || len(res.Changes) != 1 || !strings.Contains(res.Changes[0].Edits[0].NewText, "if x > 0") {
		t.Errorf("nested addSequence = %+v", res)
	}

	f.ok("apply_model_edit", map[string]any{"path": "a.sysml", "operations": []map[string]any{{"kind": "setValue", "target": "Vehicles::Nothing", "value": "1"}}}, &res)
	if len(res.Refused) != 1 || len(res.Changes) != 0 {
		t.Errorf("unknown target = %+v", res)
	}
	f.fails("apply_model_edit", map[string]any{"path": "a.sysml", "operations": []map[string]any{}})
}

func TestRendering(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	var views struct {
		Views []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"views"`
		PseudoViews []string `json:"pseudoViews"`
	}
	res0 := f.ok("views", map[string]any{"path": "a.sysml"}, &views)
	if len(views.Views) != 0 || len(views.PseudoViews) == 0 || !strings.Contains(textOf(res0), "#interconnection") {
		t.Fatalf("views = %+v (%s)", views, textOf(res0))
	}
	var render struct {
		Kind     string `json:"kind"`
		Form     string `json:"form"`
		Artifact string `json:"artifact"`
		Nodes    []struct {
			FQN string `json:"fqn"`
		} `json:"nodes"`
	}
	f.fails("render_view", map[string]any{"path": "a.sysml", "form": "dot"})
	res := f.ok("render_view", map[string]any{"path": "a.sysml", "form": "dot", "view": "#tree"}, &render)
	if render.Form != "dot" || !strings.Contains(render.Artifact, "digraph") || textOf(res) != render.Artifact {
		t.Errorf("dot rendering = %+v", render)
	}
	fqns := map[string]bool{}
	for _, n := range render.Nodes {
		fqns[n.FQN] = true
	}
	if !fqns["Vehicles::Car"] {
		t.Errorf("nodes = %v", fqns)
	}
	for _, form := range []string{"mermaid", "plantuml", "d2"} {
		f.ok("render_view", map[string]any{"path": "a.sysml", "form": form, "view": "#interconnection"}, &render)
		if render.Form != form || render.Artifact == "" {
			t.Errorf("%s rendering = %+v", form, render)
		}
	}
	f.fails("render_view", map[string]any{"path": "a.sysml", "form": "jpeg"})
	// The rendering controls reach the server: a routing, the optional
	// notation elements and a depth bound; a routing it lacks is refused.
	f.ok("render_view", map[string]any{"path": "a.sysml", "form": "dot", "view": "#interconnection", "lines": "ortho", "show": []string{"all"}, "depth": 1}, &render)
	if !strings.Contains(render.Artifact, "splines=ortho") {
		t.Errorf("dot with lines=ortho lacks splines=ortho:\n%s", render.Artifact)
	}
	if msg := f.fails("render_view", map[string]any{"path": "a.sysml", "view": "#tree", "lines": "curvy"}); !strings.Contains(msg, "curvy") {
		t.Errorf("error = %q", msg)
	}
	if msg := f.fails("render_view", map[string]any{"path": "a.sysml", "view": "#tree", "show": []string{"halos"}}); !strings.Contains(msg, "halos") {
		t.Errorf("error = %q", msg)
	}
	f.unchanged(before)
}

func TestOpenUpdateCloseHoldAnUnsavedBuffer(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	var state DocumentState
	f.ok("open_document", map[string]any{"path": "b.sysml", "text": strings.Replace(fleet, "Wheel;", "Wheel2;", 1)}, &state)
	if !state.Open || state.Version != 1 || len(state.Diagnostics) != 1 || !strings.Contains(state.Diagnostics[0].Message, "Wheel2") {
		t.Fatalf("open = %+v", state)
	}
	var diags DiagnosticsResult
	f.ok("workspace_diagnostics", map[string]any{"path": "b.sysml"}, &diags)
	if diags.Errors != 1 {
		t.Errorf("diagnostics of the open buffer = %+v", diags)
	}
	var info WorkspaceInfo
	f.ok("workspace_info", map[string]any{}, &info)
	if len(info.Open) != 1 || !strings.HasSuffix(info.Open[0], "b.sysml") {
		t.Errorf("open documents = %v", info.Open)
	}

	f.ok("update_document", map[string]any{"path": "b.sysml", "text": fleet + "package Extra { part broken : Missing; }\n"}, &state)
	if state.Version != 2 || len(state.Diagnostics) != 1 || state.Diagnostics[0].Range.Start.Line != 8 {
		t.Errorf("update = %+v", state)
	}
	f.ok("workspace_diagnostics", map[string]any{}, &diags)
	byPath := map[string]int{}
	for _, d := range diags.Diagnostics {
		byPath[d.Path]++
	}
	if byPath["b.sysml"] != 1 || byPath["bad.sysml"] != 1 {
		t.Errorf("workspace diagnostics while b is open = %v", byPath)
	}

	res, err := f.cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "sysml://workspace/b.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "package Extra") || res.Contents[0].MIMEType != "text/x-sysml" {
		t.Errorf("resource of the open buffer = %+v", res.Contents)
	}
	f.unchanged(before)

	f.ok("close_document", map[string]any{"path": "b.sysml"}, &state)
	if state.Open || len(state.Diagnostics) != 0 {
		t.Errorf("close = %+v", state)
	}
	f.ok("workspace_diagnostics", map[string]any{"path": "b.sysml"}, &diags)
	if diags.Errors != 0 {
		t.Errorf("diagnostics after close = %+v", diags)
	}
	f.unchanged(before)

	f.ok("open_document", map[string]any{"path": "new.sysml", "text": "package New { part def N; }\n"}, &state)
	var syms DocumentSymbolsResult
	f.ok("document_symbols", map[string]any{"path": "new.sysml"}, &syms)
	if len(syms.Symbols) != 1 || syms.Symbols[0].Name != "New" {
		t.Errorf("symbols of an unsaved document = %+v", syms)
	}
	f.ok("close_document", map[string]any{"path": "new.sysml"}, &state)
	f.fails("document_symbols", map[string]any{"path": "new.sysml"})
	f.unchanged(before)
}

func TestExternalEditsAreSeenWithoutBeingAnnounced(t *testing.T) {
	f := newFixture(t)
	var info WorkspaceInfo
	f.ok("workspace_info", map[string]any{}, &info)
	if len(info.Documents) != 4 {
		t.Fatalf("documents = %v", info.Documents)
	}

	write(t, f.root, "bad.sysml", "package Bad {\n    part def Fixed;\n    part p : Vehicles::Wheel;\n}\n")
	var ws WorkspaceSymbolsResult
	f.ok("workspace_symbols", map[string]any{"query": "Fixed"}, &ws)
	if len(ws.Symbols) != 1 {
		t.Errorf("workspace symbols after an external edit = %+v", ws)
	}
	var diags DiagnosticsResult
	f.ok("workspace_diagnostics", map[string]any{"path": "bad.sysml"}, &diags)
	if diags.Errors != 0 {
		t.Errorf("diagnostics after an external fix = %+v", diags.Diagnostics)
	}

	write(t, f.root, "d.sysml", "package D { part def Fresh; }\n")
	if err := os.Remove(filepath.Join(f.root, "c.sysml")); err != nil {
		t.Fatal(err)
	}
	f.ok("workspace_symbols", map[string]any{"query": "Fresh"}, &ws)
	if len(ws.Symbols) != 1 || ws.Symbols[0].Location.Path != "d.sysml" {
		t.Errorf("Fresh = %+v", ws)
	}
	f.ok("workspace_symbols", map[string]any{"query": "Größe"}, &ws)
	if len(ws.Symbols) != 0 {
		t.Errorf("deleted document still indexed: %+v", ws)
	}
	f.fails("document_symbols", map[string]any{"path": "c.sysml"})
	f.ok("workspace_info", map[string]any{}, &info)
	if len(info.Documents) != 4 {
		t.Errorf("documents after the external changes = %v", info.Documents)
	}
}

func TestResources(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	list, err := f.cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var base *mcp.Resource
	for _, r := range list.Resources {
		if !strings.HasPrefix(r.URI, "sysml://stdlib/") || r.MIMEType == "" || r.Name == "" {
			t.Errorf("resource = %+v", r)
		}
		if strings.HasSuffix(r.URI, "/Base.kerml") {
			base = r
		}
	}
	if base == nil {
		t.Fatalf("no Base.kerml among %d resources", len(list.Resources))
	}
	res, err := f.cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: base.URI})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "package Base") || res.Contents[0].MIMEType != "text/x-kerml" {
		t.Errorf("Base.kerml = %+v", res.Contents)
	}

	templates, err := f.cs.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates.ResourceTemplates) != 1 || templates.ResourceTemplates[0].URITemplate != "sysml://workspace/{+path}" {
		t.Errorf("templates = %+v", templates.ResourceTemplates)
	}
	res, err = f.cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sysml://workspace/a.sysml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 || res.Contents[0].Text != vehicles || res.Contents[0].MIMEType != "text/x-sysml" {
		t.Errorf("a.sysml resource = %+v", res.Contents)
	}
	if _, err := f.cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "sysml://workspace/missing.sysml"}); err == nil {
		t.Error("reading a missing document succeeded")
	}
}

func TestAWriteRechecksWhereALinkPoints(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	outside := filepath.Join(filepath.Dir(f.root), "outside-target.sysml")
	write(t, filepath.Dir(f.root), "outside-target.sysml", "package Outside;\n")
	t.Cleanup(func() { _ = os.Remove(outside) })
	write(t, f.root, "target.sysml", "package Linked;\n")
	alias := filepath.Join(f.root, "alias.sysml")
	if err := os.Symlink(filepath.Join(f.root, "target.sysml"), alias); err != nil {
		t.Fatal(err)
	}
	ref, err := f.host.document(ctx, "alias.sysml")
	if err != nil {
		t.Fatal(err)
	}
	// The link is repointed out of the root after the path was resolved and
	// before the write, as a concurrent actor could do.
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}
	err = f.host.writeAll(ctx, []pendingWrite{{name: ref.name, content: []byte("package Hijacked;\n"), base: []byte("package Outside;\n")}})
	if err == nil || !strings.Contains(err.Error(), "outside the workspace root") {
		t.Errorf("write through a repointed link: %v", err)
	}
	if got, _ := os.ReadFile(outside); string(got) != "package Outside;\n" {
		t.Errorf("the file outside the root was written: %q", got)
	}
}

func TestAWorkspaceEditOutsideTheRootIsRefused(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(filepath.Dir(f.root), "outside-edit.sysml")
	write(t, filepath.Dir(f.root), "outside-edit.sysml", "package Outside;\n")
	t.Cleanup(func() { _ = os.Remove(outside) })
	edit := &protocol.WorkspaceEdit{Changes: map[uri.URI][]protocol.TextEdit{
		uri.File(outside): {{Range: protocol.Range{}, NewText: "x"}},
	}}
	for _, apply := range []bool{false, true} {
		_, _, err := f.host.workspaceEdit(context.Background(), edit, apply, "rename")
		if err == nil || !strings.Contains(err.Error(), "outside the workspace root") {
			t.Errorf("apply=%v: %v", apply, err)
		}
	}
	if got, _ := os.ReadFile(outside); string(got) != "package Outside;\n" {
		t.Errorf("the file outside the root was written: %q", got)
	}
}

func TestAWriteOverTextItWasNotComputedFromIsRefused(t *testing.T) {
	f := newFixture(t)
	before := f.snapshot()
	err := f.host.writeAll(context.Background(), []pendingWrite{{name: filepath.Join(f.root, "a.sysml"), content: []byte("package A;\n"), base: []byte(vehicles + "// changed meanwhile\n")}})
	if err == nil || !strings.Contains(err.Error(), "changed on disk") {
		t.Errorf("write over changed text: %v", err)
	}
	f.unchanged(before)
}

func TestEditsWaitForEachOther(t *testing.T) {
	f := newFixture(t)
	write(t, f.root, "ugly.sysml", "package Ugly {\npart def  P;\n}\n")
	// Hold the workspace as another edit would, from its read to its write.
	f.host.mu.Lock()
	done := make(chan FormatResult, 1)
	go func() {
		var res FormatResult
		f.ok("format", map[string]any{"path": "ugly.sysml", "apply": true}, &res)
		done <- res
	}()
	select {
	case <-done:
		t.Fatal("format ran while another edit held the workspace")
	case <-time.After(200 * time.Millisecond):
	}
	f.host.mu.Unlock()
	select {
	case res := <-done:
		if !res.Applied || f.read("ugly.sysml") != res.Text {
			t.Errorf("format after the other edit = %+v", res)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("format did not run once the workspace was released")
	}
}

func TestWritesCannotLeaveTheRoot(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(filepath.Dir(f.root), "outside-write.sysml")
	write(t, filepath.Dir(f.root), "outside-write.sysml", "package Outside;\n")
	t.Cleanup(func() { _ = os.Remove(outside) })
	if err := os.Symlink(filepath.Dir(f.root), filepath.Join(f.root, "up")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	// A path that leaves the root lexically or through a link the write finds
	// in place is refused by the root the writes go through.
	for _, name := range []string{"../outside-write.sysml", "up/outside-write.sysml"} {
		if err := toolkit.WriteFile(root, name, []byte("package Hijacked;\n"), 0o600); err == nil {
			t.Errorf("writing %s through the root succeeded", name)
		}
	}
	if got, _ := os.ReadFile(outside); string(got) != "package Outside;\n" {
		t.Errorf("the file outside the root was written: %q", got)
	}
}

func TestAnOpenBufferIsNotWrittenOverAnEditMadeMeanwhile(t *testing.T) {
	f := newFixture(t)
	var state DocumentState
	f.ok("open_document", map[string]any{"path": "b.sysml", "text": fleet}, &state)
	// Writing the buffer is fine while the file is as it was when opened, and
	// again after our own write.
	var res FormatResult
	f.ok("format", map[string]any{"path": "b.sysml", "apply": true}, &res)
	f.ok("format", map[string]any{"path": "b.sysml", "apply": true}, &res)
	if res.Applied {
		t.Errorf("second format of a formatted buffer = %+v", res)
	}
	// An edit made on disk while the buffer is held is not overwritten.
	external := "package Fleet {\n    private import Vehicles::*;\n    part edited : Wheel;\n}\n"
	write(t, f.root, "b.sysml", external)
	f.ok("update_document", map[string]any{"path": "b.sysml", "text": "package Fleet {\npart p : Vehicles::Wheel;\n}\n"}, &state)
	msg := f.fails("format", map[string]any{"path": "b.sysml", "apply": true})
	if !strings.Contains(msg, "changed on disk while it was held open") {
		t.Errorf("error = %q", msg)
	}
	// Opening the document again does not take the external edit as the new
	// baseline: it is refused too, the buffer and its snapshot are kept.
	if msg := f.fails("open_document", map[string]any{"path": "b.sysml", "text": fleet}); !strings.Contains(msg, "changed on disk while it was held open") {
		t.Errorf("reopen error = %q", msg)
	}
	if msg := f.fails("format", map[string]any{"path": "b.sysml", "apply": true}); !strings.Contains(msg, "changed on disk while it was held open") {
		t.Errorf("error after reopen = %q", msg)
	}
	if got, _ := os.ReadFile(filepath.Join(f.root, "b.sysml")); string(got) != external {
		t.Errorf("b.sysml = %q, want the external edit kept", got)
	}
	var rename WorkspaceEditResult
	if res := f.call("rename", map[string]any{"path": "a.sysml", "position": pos(2, 14), "newName": "Tyre", "apply": true}, &rename); !res.IsError || !strings.Contains(textOf(res), "b.sysml") {
		t.Errorf("rename reaching the open buffer = %+v", textOf(res))
	}
	if f.read("b.sysml") != external || f.read("a.sysml") != vehicles {
		t.Error("a file was written over the external edit")
	}
	// Closing releases the buffer; the external text is what the tools see,
	// and may be written again.
	f.ok("close_document", map[string]any{"path": "b.sysml"}, &state)
	f.ok("format", map[string]any{"path": "b.sysml", "apply": true}, &res)
	if res.Applied {
		t.Errorf("formatting the external text = %+v", res)
	}
	// A buffer for a file that does not exist yet is refused once the file
	// appears underneath it.
	f.ok("open_document", map[string]any{"path": "new.sysml", "text": "package New {\npart def  N;\n}\n"}, &state)
	write(t, f.root, "new.sysml", "package New;\n")
	if msg := f.fails("format", map[string]any{"path": "new.sysml", "apply": true}); !strings.Contains(msg, "changed on disk while it was held open") {
		t.Errorf("error for a file created under a buffer = %q", msg)
	}
	if f.read("new.sysml") != "package New;\n" {
		t.Error("new.sysml was overwritten")
	}
}

func TestADanglingLinkIsJudgedByWhereItPoints(t *testing.T) {
	f := newFixture(t)
	if err := os.Symlink(filepath.Join(filepath.Dir(f.root), "not-yet.sysml"), filepath.Join(f.root, "dangle.sysml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := f.host.resolve("dangle.sysml"); err == nil || !strings.Contains(err.Error(), "leads through a symlink to") {
		t.Errorf("a dangling link out of the root: %v", err)
	}
	if msg := f.fails("open_document", map[string]any{"path": "dangle.sysml", "text": "package D;\n"}); !strings.Contains(msg, "outside the workspace root") {
		t.Errorf("open_document through a dangling link out of the root: %s", msg)
	}
	if err := os.Symlink("later.sysml", filepath.Join(f.root, "soon.sysml")); err != nil {
		t.Fatal(err)
	}
	ref, err := f.host.resolve("soon.sysml")
	if err != nil || ref.name != filepath.Join(f.root, "soon.sysml") {
		t.Errorf("a dangling link staying inside the root = %+v, %v; want it accepted as a file not yet written", ref, err)
	}
}
