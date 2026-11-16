package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// effectiveTypingModel has features written without a typing: what they redefine, subset
// or reference types them (KerML 8.3.3.3.4 deriveFeatureType), else the value they bind.
const effectiveTypingModel = `package Typing {
    private import ScalarValues::*;
    attribute def Label :> String;
    attribute def Tag :> String;
    attribute def Record {
        attribute name : Label;
        attribute alt : Tag;
        attribute plain;
    }
    attribute example : Record {
        attribute redefines name = "example";
        attribute nick :> name = "nick";
        attribute both :> name, alt;
        attribute redefines plain = "p";
        attribute wide : String :> name;
    }
    attribute deeper :> example {
        attribute redefines name = "deeper";
    }
    attribute flag = true;
    attribute free;
    part def Engine;
    part def Vehicle { part engine : Engine; }
    part car : Vehicle { part redefines engine; }
    part viaRef references car;
}
`

// typeHints maps each source line carrying a type hint to its label and tooltip.
func typeHints(t *testing.T, src string) map[string][2]string {
	t.Helper()
	s := NewServer(model.NewWorkspace())
	u := uri.File(filepath.Join(t.TempDir(), "typing.sysml"))
	openDoc(t, s, u, src)
	hints, err := s.InlayHint(context.Background(), &protocol.InlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
		Range:        protocol.Range{End: protocol.Position{Line: uint32(strings.Count(src, "\n"))}},
	})
	if err != nil {
		t.Fatalf("InlayHint: %v", err)
	}
	out := map[string][2]string{}
	for _, h := range hints {
		if h.Kind != protocol.InlayHintKindType {
			continue
		}
		tip, _ := h.Tooltip.(protocol.String)
		line := strings.TrimSpace(lineText(src, h.Position))
		if _, dup := out[line]; dup {
			t.Errorf("two type hints on %q", line)
		}
		out[line] = [2]string{hintLabel(h), string(tip)}
	}
	return out
}

func TestInlayHintStatesInheritedTypingOverTheValueType(t *testing.T) {
	got := typeHints(t, effectiveTypingModel)
	want := map[string][2]string{
		`attribute redefines name = "example";`:         {": Label", "inherited type Typing::Label"},
		`attribute nick :> name = "nick";`:              {": Label", "inherited type Typing::Label"},
		`attribute both :> name, alt;`:                  {": Label, Tag", "inherited type Typing::Label, Typing::Tag"},
		`attribute redefines plain = "p";`:              {": String", "type of the value: ScalarValues::String"},
		`attribute deeper :> example {`:                 {": Record", "inherited type Typing::Record"},
		`attribute redefines name = "deeper";`:          {": Label", "inherited type Typing::Label"},
		`attribute flag = true;`:                        {": Boolean", "type of the value: ScalarValues::Boolean"},
		`part car : Vehicle { part redefines engine; }`: {": Engine", "inherited type Typing::Engine"},
		`part viaRef references car;`:                   {": Vehicle", "inherited type Typing::Vehicle"},
	}
	for line, hint := range want {
		if got[line] != hint {
			t.Errorf("hint on %q = %q, want %q", line, got[line], hint)
		}
	}
	for line, hint := range got {
		if _, ok := want[line]; !ok {
			t.Errorf("unexpected type hint %q on %q", hint, line)
		}
	}
}

// The issue's model as reported: a library type named by its qualified name.
func TestInlayHintOnRedefinitionOfACustomScalarSubtype(t *testing.T) {
	src := `package Example {
    attribute def Label :> ScalarValues::String;
    attribute def Record {
        attribute name : Label;
    }
    attribute example : Record {
        attribute redefines name = "example";
    }
}
`
	got := typeHints(t, src)
	want := [2]string{": Label", "inherited type Example::Label"}
	if hint := got[`attribute redefines name = "example";`]; hint != want {
		t.Errorf("hint = %q, want %q", hint, want)
	}
}

func TestHoverOnADeclarationStatesItsInheritedTyping(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/typing_hover.sysml").FsPath()
	ws.Open(name, []byte(effectiveTypingModel), 1)
	for anchor, want := range map[string]string{
		`attribute redefines name = "example"`: "attribute name : Label",
		"attribute both":                       "attribute both : Label, Tag",
		"part redefines engine":                "part engine : Engine",
		"attribute wide":                       "attribute wide",
		"attribute free":                       "attribute free",
		"attribute flag":                       "attribute flag",
	} {
		off := strings.Index(effectiveTypingModel, anchor)
		if off < 0 {
			t.Fatalf("anchor %q not in source", anchor)
		}
		got := hoverMarkup(t, hoverInSrc(t, s, name, effectiveTypingModel, off+len(anchor))).Value
		if got != want {
			t.Errorf("hover on %q = %q, want %q", anchor, got, want)
		}
	}
}

func TestCompletionDetailStatesInheritedTyping(t *testing.T) {
	src := strings.Replace(effectiveTypingModel, `attribute wide : String :> name;`, `attribute wide : String :> name;
        attribute last = `, 1)
	items := completionAt(t, src, "attribute last = ")
	for label, want := range map[string]string{
		"name": "attribute : Label",
		"nick": "attribute : Label",
		"both": "attribute : Label, Tag",
		"wide": "attribute : String",
	} {
		if got := optString(items[label].Detail); got != want {
			t.Errorf("%q detail = %q, want %q", label, got, want)
		}
	}
}

// A document held as its record keeps no declarations, so the completion detail of
// its features reads their typing, written or inherited, from the model.
func TestCompletionDetailOfARecordedFeatureStatesItsTyping(t *testing.T) {
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib.sysml")
	main := filepath.Join(dir, "main.sysml")
	const libSource = `package Lib {
    attribute def Label :> ScalarValues::String;
    attribute def Record { attribute name : Label; }
    attribute example : Record { attribute redefines name = "example"; }
}
`
	const mainSource = "package Main { attribute a = Lib::example. }\n"
	for path, text := range map[string]string{lib: libSource, main: mainSource} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ws := model.NewWorkspace(model.WithRecordCache(cache))
	s := NewServer(ws)
	s.client = &fakeClient{}
	ctx := context.Background()
	if _, err := s.Initialize(ctx, &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: workspaceFolders(protocol.WorkspaceFolder{URI: uri.File(dir), Name: "records"}),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatal(err)
	}
	for path, text := range map[string]string{lib: libSource, main: mainSource} {
		if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{
			URI: uri.File(path), LanguageID: "sysml", Version: 1, Text: text,
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DidSave(ctx, &protocol.DidSaveTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)}}); err != nil {
		t.Fatal(err)
	}
	if !ws.Recorded(lib) {
		t.Fatal("lib.sysml closed unchanged is not held as its record: the fixture is vacuous")
	}
	complete := func(marker string) map[string]string {
		list, err := completions(t, s, ctx, &protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(main)},
				Position:     offsetToPosition([]byte(mainSource), strings.Index(mainSource, marker)+len(marker)),
			},
		})
		if err != nil || list == nil {
			t.Fatalf("Completion = %v, %v", list, err)
		}
		details := map[string]string{}
		for _, it := range list.Items {
			details[it.Label] = optString(it.Detail)
		}
		return details
	}
	if got := complete("Lib::example.")["name"]; got != "attribute : Label" {
		t.Errorf("recorded redefining 'name' detail = %q, want %q", got, "attribute : Label")
	}
	if got := complete("Lib::")["example"]; got != "attribute : Record" {
		t.Errorf("recorded 'example' detail = %q, want %q", got, "attribute : Record")
	}
	if !ws.Recorded(lib) {
		t.Fatal("completion hydrated lib.sysml: the details were not read from its record")
	}
}
