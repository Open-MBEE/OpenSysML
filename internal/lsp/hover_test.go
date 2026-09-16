package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

func TestHoverShowsKindAndName(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/h.sysml").Filename()
	src := "package P { namespace N; }"
	ws.Open(name, []byte(src), 1)

	// Cursor on "N" (offset of 'N' in src).
	off := strings.Index(src, "N")
	pos := offsetToPosition([]byte(src), off)

	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want content")
	}
	if !strings.Contains(res.Contents.Value, "namespace") || !strings.Contains(res.Contents.Value, "N") {
		t.Errorf("hover value = %q, want kind+name", res.Contents.Value)
	}
}

func TestHoverIncludesDocComment(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/hd.sysml").Filename()
	src := "// hello docs\npackage P { namespace N; }"
	ws.Open(name, []byte(src), 1)

	// Cursor on "P" (the package declaration).
	off := strings.Index(src, "P")
	pos := offsetToPosition([]byte(src), off)

	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want content")
	}
	if !strings.Contains(res.Contents.Value, "hello docs") {
		t.Errorf("hover value = %q, want doc-comment text", res.Contents.Value)
	}
}

// initMarkdownHover initializes s as a client that renders Markdown hovers.
func initMarkdownHover(t *testing.T, s *Server) {
	t.Helper()
	_, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Hover: &protocol.HoverTextDocumentClientCapabilities{
					ContentFormat: []protocol.MarkupKind{protocol.Markdown, protocol.PlainText},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
}

func hoverInSrc(t *testing.T, s *Server, name, src string, off int) *protocol.Hover {
	t.Helper()
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil {
		t.Fatal("Hover result = nil, want content")
	}
	return res
}

// What follows `connector` in KerML is the connector's name only ahead of
// `from`; otherwise it is the first end, itself or the end name it declares.
func TestHoverKerMLBinaryConnectorFirstToken(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/hover_first_end.kerml").Filename()
	src := "package P {\n\tclass V {\n\t\tfeature eng;\n\t\tfeature t;\n\t\tconnector eng to t;\n\t\tconnector a ::> eng to t;\n\t\tconnector c from eng to t;\n\t}\n}\n"
	ws.Open(name, []byte(src), 1)

	for probe, want := range map[string]string{
		"connector eng": "feature eng",
		"connector a":   "connector end a",
		"connector c":   "connector c",
	} {
		res := hoverInSrc(t, s, name, src, strings.Index(src, probe)+len("connector "))
		if !strings.Contains(res.Contents.Value, want) {
			t.Errorf("%s: hover = %q, want %q", probe, res.Contents.Value, want)
		}
	}
}

// Both ends of a keyword-first Disjoining are references, so hovering either
// names the type it resolves to rather than the member it sits in.
func TestHoverKerMLDisjoiningEnds(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/hover_disjoining.kerml").Filename()
	src := "package P {\n\tclassifier A;\n\tclassifier B { feature next : B; }\n\tfeature b : B;\n\tdisjoining D disjoint A from B;\n\tdisjoint b.next from A;\n}\n"
	ws.Open(name, []byte(src), 1)

	for probe, want := range map[string]string{
		"disjoint A":  "classifier A",
		"A from B":    "classifier B",
		"disjoint b":  "feature b",
		"b.next":      "feature next",
		"next from A": "classifier A",
	} {
		off := strings.Index(src, probe) + strings.LastIndexAny(probe, " .") + 1
		res := hoverInSrc(t, s, name, src, off)
		if res == nil || !strings.Contains(res.Contents.Value, want) {
			t.Errorf("%s: hover = %+v, want %q", probe, res, want)
		}
	}
}

func TestHoverRendersMarkdownWhenClientSupportsIt(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hm.sysml").Filename()
	src := "package P {\n    doc /*\n     * A wheel.\n     */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel"))
	if res.Contents.Kind != protocol.Markdown {
		t.Errorf("hover kind = %q, want %q", res.Contents.Kind, protocol.Markdown)
	}
	if !strings.Contains(res.Contents.Value, "```sysml") {
		t.Errorf("hover value = %q, want a fenced sysml block", res.Contents.Value)
	}
	if !strings.Contains(res.Contents.Value, "Wheel") {
		t.Errorf("hover value = %q, want the signature", res.Contents.Value)
	}
	if !strings.Contains(res.Contents.Value, "A wheel.") {
		t.Errorf("hover value = %q, want the doc text", res.Contents.Value)
	}
	for _, marker := range []string{"/*", "*/", " * "} {
		if strings.Contains(res.Contents.Value, marker) {
			t.Errorf("hover value = %q, still carries the comment marker %q", res.Contents.Value, marker)
		}
	}
}

func TestHoverSignatureUsesNotationKeywords(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hk.sysml").Filename()
	src := "package P {\n    part def Wheel;\n    part w : Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct {
		at   string
		want string
	}{
		{at: "Wheel;", want: "```sysml\npart def Wheel\n```"},
		{at: "w :", want: "```sysml\npart w\n```"},
	} {
		res := hoverInSrc(t, s, name, src, strings.Index(src, tc.at))
		if res.Contents.Value != tc.want {
			t.Errorf("hover on %q = %q, want %q", tc.at, res.Contents.Value, tc.want)
		}
	}
}

func TestHoverStripsDelimitersOfEveryLeadingComment(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hc.sysml").Filename()
	src := "package P {\n    /* first */\n    /* second */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel;"))
	want := "```sysml\npart def Wheel\n```\n\nfirst\n\nsecond"
	if res.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", res.Contents.Value, want)
	}
}

func TestHoverQuotesAnUnrestrictedName(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hq.sysml").Filename()
	src := "package P {\n    part def 'my wheel';\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "my wheel"))
	want := "```sysml\npart def 'my wheel'\n```"
	if res.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", res.Contents.Value, want)
	}
}

func TestHoverStripsBlockNoteDelimiters(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hn.sysml").Filename()
	src := "package P {\n    //* a note */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel;"))
	want := "```sysml\npart def Wheel\n```\n\na note"
	if res.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", res.Contents.Value, want)
	}
}

func TestHoverKeepsDocCommentLineBreaks(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hl.sysml").Filename()
	src := "package P {\n    doc /*\n     * First line.\n     * Second line.\n     */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel;"))
	want := "```sysml\npart def Wheel\n```\n\nFirst line.  \nSecond line."
	if res.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", res.Contents.Value, want)
	}
}

// The star a block comment runs down its edge is decoration; the doubled star
// an author writes is emphasis, and survives.
func TestHoverKeepsAuthoredMarkdownEmphasis(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hm.sysml").Filename()
	src := "package P {\n    /*\n     * **Warning** load-bearing.\n     */\n    part def Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "Wheel;"))
	want := "```sysml\npart def Wheel\n```\n\n**Warning** load-bearing."
	if res.Contents.Value != want {
		t.Errorf("hover value = %q, want %q", res.Contents.Value, want)
	}
}

func TestHoverFallsBackToPlainTextWithoutMarkdownCapability(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	// No initialize: a client that never advertised Markdown must get plaintext.
	name := uri.File("/tmp/hp.sysml").Filename()
	src := "package P { namespace N; }"
	ws.Open(name, []byte(src), 1)

	res := hoverInSrc(t, s, name, src, strings.Index(src, "N"))
	if res.Contents.Kind != protocol.PlainText {
		t.Errorf("hover kind = %q, want %q", res.Contents.Kind, protocol.PlainText)
	}
	if strings.Contains(res.Contents.Value, "```") {
		t.Errorf("plaintext hover value = %q, should carry no Markdown fence", res.Contents.Value)
	}
}

func TestHoverMissWhenNoSymbol(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/h2.sysml").Filename()
	ws.Open(name, []byte("package P;"), 1)
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     protocol.Position{Line: 5, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res != nil {
		t.Errorf("expected nil hover for out-of-range position, got %+v", res)
	}
}

// A binding's ends are connector ends (SysML.xtext:1000): a named end hovers as
// the end it declares, the feature after `::>` as the feature it references.
func TestHoverBindingConnectorEnds(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/hover_binding_ends.sysml").Filename()
	src := "package P {\n\tpart def V {\n\t\tattribute a;\n\t\tattribute b;\n\t\tbind a = b;\n\t\tbind e1 ::> a = e2 references b;\n\t\tbinding bb bind e3 ::> a = b;\n\t}\n}\n"
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct{ before, token, want string }{
		{"bind ", "a = b", "attribute a"},
		{"bind ", "e1 ::>", "connector end e1"},
		{"::> ", "a = e2", "attribute a"},
		{"= ", "e2 references", "connector end e2"},
		{"binding ", "bb bind", "binding bb"},
		{"bind ", "e3 ::>", "connector end e3"},
	} {
		off := strings.Index(src, tc.before+tc.token) + len(tc.before)
		res := hoverInSrc(t, s, name, src, off)
		if !strings.Contains(res.Contents.Value, tc.want) {
			t.Errorf("%s: hover = %q, want %q", tc.token, res.Contents.Value, tc.want)
		}
	}
}

// An id fixed by an annotation or by the norm is stated; a derived id, which is
// just the encoded name, is not.
func TestHoverStatesDeclaredAndNormativeIdentity(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	initMarkdownHover(t, s)
	name := uri.File("/tmp/hid.sysml").Filename()
	src := "package P {\n    part def Wheel {\n        @IdentityMetadata::ElementId { id = \"wheel-id\"; }\n    }\n    attribute mass : ScalarValues::Real;\n    part w : Wheel;\n}\n"
	ws.Open(name, []byte(src), 1)

	for _, tc := range []struct {
		at   string
		want string
	}{
		{at: "Wheel {", want: "\n\nElement id `wheel-id` (declared)"},
		{at: "Real;", want: "\n\nElement id `14c0aa22-5489-59b5-b438-ded26e83ba31` (normative, KerML)"},
	} {
		res := hoverInSrc(t, s, name, src, strings.Index(src, tc.at))
		if !strings.HasSuffix(res.Contents.Value, tc.want) {
			t.Errorf("hover on %q = %q, want it to end in %q", tc.at, res.Contents.Value, tc.want)
		}
	}
	if res := hoverInSrc(t, s, name, src, strings.Index(src, "w :")); strings.Contains(res.Contents.Value, "Element id") {
		t.Errorf("hover on a derived id = %q, want no identity line", res.Contents.Value)
	}
}

func TestHoverStatesIdentityInPlainText(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/hidp.sysml").Filename()
	src := "package P { attribute mass : ScalarValues::Real; }"
	ws.Open(name, []byte(src), 1)
	res := hoverInSrc(t, s, name, src, strings.Index(src, "Real;"))
	if want := "\n\nElement id 14c0aa22-5489-59b5-b438-ded26e83ba31 (normative, KerML)"; !strings.HasSuffix(res.Contents.Value, want) {
		t.Errorf("hover = %q, want it to end in %q", res.Contents.Value, want)
	}
}

// Hovering a declaration in a workspace copy of a library file, rooted at the
// library's package, states the norm's id; a reference to it from another file
// resolves to the copy and states the same.
func TestHoverWorkspaceCopyOfLibraryFileStatesNormativeIdentity(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	lib := ws.LibraryDocument("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml")
	if lib == nil {
		t.Fatal("ScalarValues.kerml not bundled")
	}
	copyName := uri.File("/tmp/ScalarValues.kerml").Filename()
	src := string(lib.Content)
	ws.Open(copyName, lib.Content, 1)
	const want = "Element id 14c0aa22-5489-59b5-b438-ded26e83ba31 (normative, KerML)"
	if res := hoverInSrc(t, s, copyName, src, strings.Index(src, "datatype Real ")+len("datatype ")); !strings.Contains(res.Contents.Value, want) {
		t.Errorf("hover on the copy's Real = %q, want %q", res.Contents.Value, want)
	}
	user := uri.File("/tmp/uses.sysml").Filename()
	use := "package P { attribute mass : ScalarValues::Real; }"
	ws.Open(user, []byte(use), 1)
	if res := hoverInSrc(t, s, user, use, strings.Index(use, "Real;")); !strings.Contains(res.Contents.Value, want) {
		t.Errorf("hover on a reference to the copy's Real = %q, want %q", res.Contents.Value, want)
	}
}

// Hovering a library declaration in its own bundled document states the norm's id.
func TestHoverLibraryDeclarationStatesNormativeIdentity(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	const file = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"
	doc := ws.LibraryDocument(file)
	if doc == nil {
		t.Fatalf("library document %q not bundled", file)
	}
	src := string(doc.Content)
	res, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: libraryURI(file)},
			Position:     offsetToPosition(doc.Content, strings.Index(src, "datatype Real ")+len("datatype ")),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if res == nil || !strings.Contains(res.Contents.Value, "Element id 14c0aa22-5489-59b5-b438-ded26e83ba31 (normative, KerML)") {
		t.Errorf("hover = %+v, want the norm's id for ScalarValues::Real", res)
	}
}
