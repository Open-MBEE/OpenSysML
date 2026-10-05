package lsp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// editorFactsModel exercises signature help, inlay hints and code lenses: a
// calc with an optional parameter, features typed only by their values, and
// behaviors at the package level with steps nested inside them.
const editorFactsModel = `package Demo {
    private import ScalarValues::*;
    calc def Fall {
        in h : Real;
        in g : Real = 9.81;
        return t : Real = h / g;
    }
    part def Rover {
        attribute mass : Real = 10.0;
        attribute halfMass = mass / 2;
    }
    attribute base : Real = 10.0;
    attribute twice = base * 2;
    attribute flag = true;
    attribute count = 3 + 4;
    calc drop : Fall { in h = 20.0; }
    attribute fell = Fall(20.0, 9.81);
    attribute named = Fall(g = 9.81, h = 20.0);
    action def Charge {
        action step1;
        then action step2;
    }
    perform action charge : Charge;
    state def Mission {
        entry; then idle;
        state idle;
    }
    requirement wagonSpec {
        subject unit : Rover;
    }
    constraint def Positive {
        in x : Real;
        x > 0
    }
    action def Move {
        in distance : Real;
        in speed : Real = 1.0;
    }
    action go = Move(10.0, 2.0);
}
`

func openEditorFactsModel(t *testing.T) (*Server, uri.URI) {
	t.Helper()
	s := NewServer(model.NewWorkspace())
	u := uri.File(filepath.Join(t.TempDir(), "editor_facts.sysml"))
	openDoc(t, s, u, editorFactsModel)
	return s, u
}

// positionAfter is the position just after the first occurrence of anchor.
func positionAfter(t *testing.T, src, anchor string) protocol.Position {
	t.Helper()
	off := strings.Index(src, anchor)
	if off < 0 {
		t.Fatalf("anchor %q not in source", anchor)
	}
	return positionsFor([]byte(src)).position(off + len(anchor))
}

func signatureHelpAt(t *testing.T, s *Server, u uri.URI, anchor string) *protocol.SignatureHelp {
	t.Helper()
	help, err := s.SignatureHelp(context.Background(), &protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Position:     positionAfter(t, editorFactsModel, anchor),
		},
	})
	if err != nil {
		t.Fatalf("SignatureHelp(%q): %v", anchor, err)
	}
	return help
}

func TestSignatureHelpListsParametersWithTypesAndActiveOne(t *testing.T) {
	s, u := openEditorFactsModel(t)
	help := signatureHelpAt(t, s, u, "fell = Fall(2")
	if help == nil || len(help.Signatures) != 1 {
		t.Fatalf("SignatureHelp = %+v, want one signature", help)
	}
	sig := help.Signatures[0]
	if sig.Label != "Fall(h : Real, [g : Real]) : Real" {
		t.Errorf("label = %q", sig.Label)
	}
	if len(sig.Parameters) != 2 || sig.Parameters[0].Label != "h : Real" || sig.Parameters[1].Label != "g : Real" {
		t.Errorf("parameters = %+v", sig.Parameters)
	}
	if sig.Parameters[1].Documentation != "has a default" || sig.Parameters[0].Documentation != "" {
		t.Errorf("parameter documentation = %+v, want the defaulted one marked", sig.Parameters)
	}
	if help.ActiveParameter != 0 {
		t.Errorf("active parameter on the first argument = %d", help.ActiveParameter)
	}
	if help = signatureHelpAt(t, s, u, "fell = Fall(20.0, 9."); help == nil || help.ActiveParameter != 1 {
		t.Errorf("active parameter on the second argument = %+v, want 1", help)
	}
	if help = signatureHelpAt(t, s, u, "fell = Fall(20.0,"); help == nil || help.ActiveParameter != 1 {
		t.Errorf("active parameter just after the comma = %+v, want 1", help)
	}
}

func TestSignatureHelpNamedArgumentSelectsItsParameter(t *testing.T) {
	s, u := openEditorFactsModel(t)
	if help := signatureHelpAt(t, s, u, "named = Fall(g = 9.81, h = 2"); help == nil || help.ActiveParameter != 0 {
		t.Errorf("active parameter in `h = 20.0` = %+v, want h (0)", help)
	}
	if help := signatureHelpAt(t, s, u, "named = Fall(g = 9."); help == nil || help.ActiveParameter != 1 {
		t.Errorf("active parameter in `g = 9.81` = %+v, want g (1)", help)
	}
}

func TestSignatureHelpCoversAPerformedAction(t *testing.T) {
	s, u := openEditorFactsModel(t)
	help := signatureHelpAt(t, s, u, "go = Move(10.0, 2")
	if help == nil || len(help.Signatures) != 1 {
		t.Fatalf("SignatureHelp = %+v, want one signature", help)
	}
	if help.Signatures[0].Label != "Move(distance : Real, [speed : Real])" {
		t.Errorf("label = %q", help.Signatures[0].Label)
	}
	if help.ActiveParameter != 1 {
		t.Errorf("active parameter = %d, want speed (1)", help.ActiveParameter)
	}
}

func TestSignatureHelpOutsideAnArgumentListIsEmpty(t *testing.T) {
	s, u := openEditorFactsModel(t)
	for _, anchor := range []string{"fell = Fa", "fell = Fall(20.0, 9.81)", "attribute twice = base"} {
		if help := signatureHelpAt(t, s, u, anchor); help != nil {
			t.Errorf("SignatureHelp after %q = %+v, want none", anchor, help)
		}
	}
}

func TestInlayHintsInferTypesAndModelLevelValues(t *testing.T) {
	s, u := openEditorFactsModel(t)
	lines := strings.Count(editorFactsModel, "\n")
	hints, err := s.InlayHint(&inlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
		Range:        protocol.Range{End: protocol.Position{Line: uint32(lines)}},
	})
	if err != nil {
		t.Fatalf("InlayHint: %v", err)
	}
	got := map[string]string{}
	pos := positionsFor([]byte(editorFactsModel))
	for _, h := range hints {
		got[h.Label] = lineText(editorFactsModel, h.Position)
		_ = pos
	}
	want := map[string]string{
		": Real":    "    attribute halfMass = mass / 2;",
		"= 20.0":    "    attribute twice = base * 2;",
		": Boolean": "    attribute flag = true;",
		"= 7":       "    attribute count = 3 + 4;",
	}
	for label, line := range want {
		if got[label] == "" {
			t.Errorf("no hint %q; got %v", label, hints)
		} else if got[label] != line && label != ": Real" {
			t.Errorf("hint %q on %q, want %q", label, got[label], line)
		}
	}
	for _, h := range hints {
		line := lineText(editorFactsModel, h.Position)
		if strings.Contains(line, "halfMass") && strings.HasPrefix(h.Label, "=") {
			t.Errorf("value hint %q on %q: mass is an instance feature, not a model-level value", h.Label, line)
		}
		if strings.Contains(line, "attribute base") || strings.Contains(line, "attribute mass") {
			t.Errorf("hint %q on %q, which declares its type and a literal value", h.Label, line)
		}
		if (strings.Contains(line, "attribute twice") || strings.Contains(line, "attribute count")) && strings.HasPrefix(h.Label, ":") {
			t.Errorf("type hint %q on %q: arithmetic is typed no closer than DataValue", h.Label, line)
		}
	}
	typed := 0
	for _, h := range hints {
		if h.Kind == inlayHintKindType {
			typed++
			if !h.PaddingLeft || !strings.HasPrefix(h.Label, ": ") {
				t.Errorf("type hint = %+v, want `: T` with left padding", h)
			}
		}
	}
	if typed == 0 {
		t.Error("no type hints")
	}
}

func TestInlayHintsAreLimitedToTheRange(t *testing.T) {
	s, u := openEditorFactsModel(t)
	at := positionAfter(t, editorFactsModel, "attribute count")
	hints, err := s.InlayHint(&inlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: u},
		Range:        protocol.Range{Start: protocol.Position{Line: at.Line}, End: protocol.Position{Line: at.Line + 1}},
	})
	if err != nil {
		t.Fatalf("InlayHint: %v", err)
	}
	for _, h := range hints {
		if h.Position.Line != at.Line {
			t.Errorf("hint %+v outside the requested line %d", h, at.Line)
		}
	}
	if len(hints) == 0 {
		t.Error("no hints on the `count` line")
	}
}

func lineText(src string, pos protocol.Position) string {
	lines := strings.Split(src, "\n")
	if int(pos.Line) >= len(lines) {
		return ""
	}
	return lines[pos.Line]
}

func TestCodeLensOffersReferenceCountsAndRunCommands(t *testing.T) {
	s, u := openEditorFactsModel(t)
	lenses, err := s.CodeLens(context.Background(), &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})
	if err != nil {
		t.Fatalf("CodeLens: %v", err)
	}
	runs := map[string]string{}
	counts := map[string]bool{}
	for _, lens := range lenses {
		line := lineText(editorFactsModel, lens.Range.Start)
		if lens.Command == nil {
			data, ok := lens.Data.(codeLensData)
			if !ok || data.URI != u || data.Element == "" {
				t.Errorf("reference lens on %q carries %v", line, lens.Data)
			}
			counts[data.Element] = true
			continue
		}
		if lens.Command.Command != runElementCommand || len(lens.Command.Arguments) != 1 {
			t.Errorf("run lens on %q = %+v", line, lens.Command)
			continue
		}
		args := lens.Command.Arguments[0].(runElementArgs)
		runs[args.Element] = lens.Command.Title + " (" + args.Kind + ")"
	}
	wantCounts := []string{"Demo::Fall", "Demo::Rover", "Demo::Charge", "Demo::Mission", "Demo::Positive"}
	for _, fqn := range wantCounts {
		if !counts[fqn] {
			t.Errorf("no reference lens on %s; got %v", fqn, counts)
		}
	}
	wantRuns := map[string]string{
		"Demo::Fall":      "Evaluate Fall (calc)",
		"Demo::drop":      "Evaluate drop (calc)",
		"Demo::Charge":    "Run Charge (action)",
		"Demo::charge":    "Run charge (action)",
		"Demo::Mission":   "Run Mission (state)",
		"Demo::wagonSpec": "Evaluate wagonSpec (requirement)",
		"Demo::Positive":  "Evaluate Positive (constraint)",
	}
	for fqn, want := range wantRuns {
		if runs[fqn] != want {
			t.Errorf("run lens on %s = %q, want %q", fqn, runs[fqn], want)
		}
	}
	for _, nested := range []string{"Demo::Charge::step1", "Demo::Charge::step2", "Demo::Mission::idle"} {
		if _, ok := runs[nested]; ok {
			t.Errorf("run lens on %s, a step of its behavior", nested)
		}
	}
	for _, fqn := range []string{"Demo::Rover", "Demo::twice", "Demo::Rover::mass"} {
		if _, ok := runs[fqn]; ok {
			t.Errorf("run lens on %s, which no check runs", fqn)
		}
	}
}

func TestCodeLensResolveCountsReferences(t *testing.T) {
	s, u := openEditorFactsModel(t)
	lenses, err := s.CodeLens(context.Background(), &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})
	if err != nil {
		t.Fatalf("CodeLens: %v", err)
	}
	var fall *protocol.CodeLens
	for i := range lenses {
		if data, ok := lenses[i].Data.(codeLensData); ok && data.Element == "Demo::Fall" {
			fall = &lenses[i]
		}
	}
	if fall == nil {
		t.Fatal("no reference lens on Demo::Fall")
	}
	// A client sends the lens back decoded generically.
	fall.Data = map[string]any{"uri": string(u), "element": "Demo::Fall", "name": fall.Data.(codeLensData).Name}
	resolved, err := s.CodeLensResolve(context.Background(), fall)
	if err != nil {
		t.Fatalf("CodeLensResolve: %v", err)
	}
	if resolved.Command == nil || resolved.Command.Command != showReferencesCommand {
		t.Fatalf("resolved = %+v, want a show-references command", resolved)
	}
	if resolved.Command.Title != "3 references" {
		t.Errorf("title = %q, want the typing of drop and the two calls", resolved.Command.Title)
	}
	locs, ok := resolved.Command.Arguments[2].([]protocol.Location)
	if !ok || len(locs) != 3 {
		t.Errorf("locations = %v", resolved.Command.Arguments[2])
	}
	if resolved.Command.Arguments[0] != u {
		t.Errorf("uri argument = %v", resolved.Command.Arguments[0])
	}
}

func TestCodeLensSkipsLibraryDocuments(t *testing.T) {
	s, _ := openEditorFactsModel(t)
	lenses, err := s.CodeLens(context.Background(), &protocol.CodeLensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "sysml-stdlib:/ScalarValues.kerml"},
	})
	if err != nil || len(lenses) != 0 {
		t.Errorf("CodeLens on a library document = %v, %v; want none", lenses, err)
	}
}

const overloadModel = `package Lib {
    private import ScalarValues::*;
    calc def pick { in n : Integer; return r : Integer = n; }
    calc def pick { in s : String; return r : String = s; }
    attribute a = pick(2);
    attribute b = pick("s");
    attribute c = pick("t");
}
`

func TestCodeLensTellsOverloadsApart(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File(filepath.Join(t.TempDir(), "overloads.sysml"))
	openDoc(t, s, u, overloadModel)
	lenses, err := s.CodeLens(context.Background(), &protocol.CodeLensParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}})
	if err != nil {
		t.Fatalf("CodeLens: %v", err)
	}
	var titles []string
	for _, lens := range lenses {
		if lens.Command != nil {
			t.Errorf("run lens %q on a name declared twice, which sysml cannot run", lens.Command.Title)
			continue
		}
		data := lens.Data.(codeLensData)
		lens.Data = map[string]any{"uri": string(u), "element": data.Element, "name": data.Name}
		resolved, err := s.CodeLensResolve(context.Background(), &lens)
		if err != nil || resolved.Command == nil {
			t.Fatalf("CodeLensResolve(%+v) = %+v, %v", data, resolved, err)
		}
		titles = append(titles, resolved.Command.Title)
	}
	if len(titles) != 2 || titles[0] != "1 reference" || titles[1] != "2 references" {
		t.Errorf("resolved titles = %v, want the Integer pick's one call and the String pick's two", titles)
	}
}

const typingModel = `package Typing {
    private import ScalarValues::*;
    calc def Fall { in h : Real; in g : Real = 9.81; return t : Real = h / g; }
    attribute commented = Fall(20.0 /* height, in metres */, 9.81);
    attribute lined = Fall(20.0, // the height,
        9.81);
    attribute partial = Fall(20.0,
    attribute named = Fall(h = 20.0, g
}
`

func TestSignatureHelpIgnoresCommasInComments(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File(filepath.Join(t.TempDir(), "typing.sysml"))
	openDoc(t, s, u, typingModel)
	for _, anchor := range []string{"metres */, 9.", "// the height,\n        9."} {
		pos := positionAfter(t, typingModel, anchor)
		help, err := s.SignatureHelp(context.Background(), &protocol.SignatureHelpParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}, Position: pos},
		})
		if err != nil || help == nil || help.ActiveParameter != 1 {
			t.Errorf("SignatureHelp(%q) = %+v, %v; want the second parameter active", anchor, help, err)
		}
	}
}

func TestSignatureHelpAnswersWhileAListIsBeingTyped(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File(filepath.Join(t.TempDir(), "typing.sysml"))
	openDoc(t, s, u, typingModel)
	for _, anchor := range []string{"partial = Fall(20.0,", "named = Fall(h = 20.0, g"} {
		pos := positionAfter(t, typingModel, anchor)
		help, err := s.SignatureHelp(context.Background(), &protocol.SignatureHelpParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}, Position: pos},
		})
		if err != nil || help == nil || len(help.Signatures) == 0 {
			t.Fatalf("SignatureHelp(%q) = %+v, %v; want the signature of the unfinished call", anchor, help, err)
		}
		if help.ActiveParameter != 1 {
			t.Errorf("SignatureHelp(%q) active parameter = %d, want g", anchor, help.ActiveParameter)
		}
	}
}

const multilineModel = `package Lines {
    private import ScalarValues::*;
    attribute base : Real = 10.0;
    attribute total =
        base;
}
`

func TestInlayHintsOutsideTheRangeAreDropped(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File(filepath.Join(t.TempDir(), "lines.sysml"))
	openDoc(t, s, u, multilineModel)
	first := positionAfter(t, multilineModel, "attribute total")
	hintsOn := func(line uint32) []string {
		hints, err := s.InlayHint(&inlayHintParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: u},
			Range:        protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line + 1}},
		})
		if err != nil {
			t.Fatalf("InlayHint: %v", err)
		}
		var labels []string
		for _, h := range hints {
			if h.Position.Line != line {
				t.Errorf("hint %+v outside the requested line %d", h, line)
			}
			labels = append(labels, h.Label)
		}
		return labels
	}
	if got := hintsOn(first.Line); len(got) != 1 || got[0] != ": Real" {
		t.Errorf("hints on the name's line = %v, want only its type", got)
	}
	if got := hintsOn(first.Line + 1); len(got) != 1 || got[0] != "= 10.0" {
		t.Errorf("hints on the value's line = %v, want only its value", got)
	}
}

func TestInlayHintAtTheRangeEndBelongsToTheNextRange(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	u := uri.File(filepath.Join(t.TempDir(), "lines.sysml"))
	openDoc(t, s, u, multilineModel)
	at := positionAfter(t, multilineModel, "attribute total")
	hintsIn := func(r protocol.Range) int {
		hints, err := s.InlayHint(&inlayHintParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}, Range: r})
		if err != nil {
			t.Fatalf("InlayHint: %v", err)
		}
		return len(hints)
	}
	start := protocol.Position{Line: at.Line}
	if n := hintsIn(protocol.Range{Start: start, End: at}); n != 0 {
		t.Errorf("%d hints in a range ending where the type hint sits; the end is excluded", n)
	}
	after := protocol.Position{Line: at.Line, Character: at.Character + 1}
	if n := hintsIn(protocol.Range{Start: start, End: after}); n != 1 {
		t.Errorf("%d hints in a range ending just past the type hint, want it alone", n)
	}
	if n := hintsIn(protocol.Range{Start: at, End: after}); n != 1 {
		t.Errorf("%d hints in a range starting on the type hint, want it included", n)
	}
}
