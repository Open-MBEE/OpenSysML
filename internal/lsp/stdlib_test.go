package lsp

import (
	"bufio"
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const (
	scalarValuesName = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"
	baseName         = "Kernel Libraries/Kernel Semantic Library/Base.kerml"
	occurrencesName  = "Kernel Libraries/Kernel Semantic Library/Occurrences.kerml"
)

// libraryText is the bundled text of a library file, read as the server does.
func libraryText(t *testing.T, name string) string {
	t.Helper()
	content, err := libs.EmbeddedSource().Read(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

// lineOf is the zero-based line the first occurrence of sub sits on.
func lineOf(t *testing.T, text, sub string) uint32 {
	t.Helper()
	off := strings.Index(text, sub)
	if off < 0 {
		t.Fatalf("%q not found", sub)
	}
	return uint32(strings.Count(text[:off], "\n"))
}

func TestLibraryURIRoundTrip(t *testing.T) {
	u := libraryURI(baseName)
	want := uri.URI("sysml-stdlib:///Kernel%20Libraries/Kernel%20Semantic%20Library/Base.kerml")
	if u != want {
		t.Errorf("libraryURI = %q, want %q", u, want)
	}
	if got := uriToName(u); got != baseName {
		t.Errorf("uriToName = %q, want %q", got, baseName)
	}
	if !isLibraryURI(u) {
		t.Error("isLibraryURI = false for a library URI")
	}

	fileURI := uri.File("/tmp/Base.kerml")
	if isLibraryURI(fileURI) {
		t.Error("isLibraryURI = true for a file URI")
	}
	if got := uriToName(fileURI); got != fileURI.Filename() {
		t.Errorf("uriToName(file) = %q, want %q", got, fileURI.Filename())
	}

	for _, bad := range []string{
		"sysml-stdlib:///",
		"sysml-stdlib:///../etc/passwd",
		"sysml-stdlib:///Kernel%20Libraries/../../x.kerml",
		"sysml-stdlib:///Kernel%20Libraries//Base.kerml",
		"sysml-stdlib://host/Base.kerml",
		"stdlib:///Base.kerml",
	} {
		if _, ok := libraryURIName(uri.URI(bad)); ok {
			t.Errorf("libraryURIName(%q) accepted, want rejected", bad)
		}
	}
}

// Definition on a standard-library name lands in the library's own document,
// on the line the bundled text declares it.
func TestDefinitionOfLibrarySymbolOpensVirtualDocument(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/stdlib_def.sysml").Filename()
	src := "package P { attribute x : ScalarValues::Integer; }\n"
	ws.Open(name, []byte(src), 1)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), strings.Index(src, "Integer")),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if locs[0].URI != libraryURI(scalarValuesName) {
		t.Errorf("URI = %q, want %q", locs[0].URI, libraryURI(scalarValuesName))
	}
	text := libraryText(t, scalarValuesName)
	if want := lineOf(t, text, "datatype Integer"); locs[0].Range.Start.Line != want {
		t.Errorf("decl line = %d, want %d", locs[0].Range.Start.Line, want)
	}
	if locs[0].Range.End == locs[0].Range.Start {
		t.Error("decl range is empty")
	}
}

// Navigation within the library: a reference in one library document resolves
// to the declaration in another.
func TestDefinitionWithinLibraryDocuments(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	text := libraryText(t, occurrencesName)
	off := strings.Index(text, "Base::Anything")
	if off < 0 {
		t.Fatal("Occurrences.kerml no longer imports Base::Anything")
	}
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: libraryURI(occurrencesName)},
			Position:     offsetToPosition([]byte(text), off+len("Base::")),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if locs[0].URI != libraryURI(baseName) {
		t.Errorf("URI = %q, want %q", locs[0].URI, libraryURI(baseName))
	}
	base := libraryText(t, baseName)
	if want := lineOf(t, base, "abstract classifier Anything"); locs[0].Range.Start.Line != want {
		t.Errorf("decl line = %d, want %d", locs[0].Range.Start.Line, want)
	}
}

// Hover, document symbols and semantic tokens answer against a library URI from
// the bundled text.
func TestRequestsAgainstLibraryURI(t *testing.T) {
	ctx := context.Background()
	s := NewServer(model.NewWorkspace())
	text := libraryText(t, baseName)
	docURI := libraryURI(baseName)

	hov, err := s.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     offsetToPosition([]byte(text), strings.Index(text, "Anything")),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hov == nil || !strings.Contains(hov.Contents.Value, "classifier Anything") {
		t.Errorf("hover = %+v, want the classifier's signature", hov)
	}

	syms, err := s.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})
	if err != nil {
		t.Fatalf("DocumentSymbol err = %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("top-level symbols = %d, want the Base package alone", len(syms))
	}
	pkg, ok := syms[0].(protocol.DocumentSymbol)
	if !ok || pkg.Name != "Base" {
		t.Fatalf("symbol = %#v, want package Base", syms[0])
	}
	var names []string
	for _, child := range pkg.Children {
		names = append(names, child.Name)
	}
	sort.Strings(names)
	if i := sort.SearchStrings(names, "Anything"); i >= len(names) || names[i] != "Anything" {
		t.Errorf("Base members = %v, want Anything among them", names)
	}

	tokens, err := s.SemanticTokensFull(ctx, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})
	if err != nil {
		t.Fatalf("SemanticTokensFull err = %v", err)
	}
	if len(tokens.Data) == 0 || len(tokens.Data)%5 != 0 {
		t.Errorf("semantic tokens = %d integers, want a non-empty multiple of 5", len(tokens.Data))
	}
}

// References with the declaration included locate a library declaration in its
// library document, next to the workspace uses.
func TestReferencesLocateLibraryDeclaration(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/stdlib_refs.sysml").Filename()
	src := "package P { attribute x : ScalarValues::Integer; attribute y : ScalarValues::Integer; }\n"
	ws.Open(name, []byte(src), 1)

	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), strings.Index(src, "Integer")),
		},
		Context: protocol.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	var library, workspace int
	text := libraryText(t, scalarValuesName)
	for _, loc := range locs {
		switch loc.URI {
		case libraryURI(scalarValuesName):
			library++
			if want := lineOf(t, text, "datatype Integer"); loc.Range.Start.Line != want {
				t.Errorf("declaration line = %d, want %d", loc.Range.Start.Line, want)
			}
		case uri.File(name):
			workspace++
		default:
			t.Errorf("unexpected location %+v", loc)
		}
	}
	if library != 1 || workspace != 2 {
		t.Errorf("library = %d, workspace = %d locations, want 1 and 2", library, workspace)
	}
}

func TestStdlibContentServesBundledText(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.StdlibContent(&stdlibContentParams{URI: libraryURI(baseName)})
	if err != nil {
		t.Fatalf("StdlibContent err = %v", err)
	}
	if res.Text != libraryText(t, baseName) {
		t.Error("content differs from the bundled Base.kerml")
	}

	for _, bad := range []uri.URI{
		libraryURI("Kernel Libraries/Nowhere.kerml"),
		uri.File("/tmp/Base.kerml"),
		"sysml-stdlib:///../Base.kerml",
	} {
		_, err := s.StdlibContent(&stdlibContentParams{URI: bad})
		var rpcErr *jsonrpc2.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidParams {
			t.Errorf("StdlibContent(%q) err = %v, want an invalid-params error", bad, err)
		}
	}
}

// A client may open and close a library document; the bundled text stays what
// is served, and a change is refused rather than applied.
func TestLibraryDocumentIsReadOnly(t *testing.T) {
	ctx := context.Background()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	docURI := libraryURI(baseName)
	text := libraryText(t, baseName)

	if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, LanguageID: "kerml", Version: 1, Text: "package Bogus;"},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}
	if ws.Document(baseName) != nil {
		t.Error("opening a library document added it to the workspace")
	}

	err := s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI}, Version: 2},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: "package Bogus;"}},
	})
	var rpcErr *jsonrpc2.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc2.InvalidRequest || !strings.Contains(rpcErr.Message, "read-only") {
		t.Fatalf("DidChange err = %v, want a read-only refusal", err)
	}
	if string(ws.LibraryDocument(baseName).Content) != text {
		t.Error("a refused change altered the library document")
	}
	if ws.Document(baseName) != nil {
		t.Error("a refused change added the library document to the workspace")
	}

	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	if got := ws.LookupQualified("Base::Anything"); len(got) != 1 {
		t.Errorf("Base::Anything resolves to %d symbols after closing Base.kerml, want 1", len(got))
	}
	res, err := s.StdlibContent(&stdlibContentParams{URI: docURI})
	if err != nil || res.Text != text {
		t.Errorf("StdlibContent after close = (%v, %v), want the bundled text", err, res)
	}
}

// mapSource is an in-memory library for a test that controls the library text.
type mapSource map[string]string

func (m mapSource) List() []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m mapSource) Read(name string) ([]byte, error) {
	text, ok := m[name]
	if !ok {
		return nil, errors.New("no such file")
	}
	return []byte(text), nil
}

// A library position is reported in UTF-16 code units of the library text, as
// a workspace position is.
func TestLibraryLocationsUseUTF16Positions(t *testing.T) {
	const libName = "Test Library/Fake.kerml"
	src := mapSource{libName: "package Fake {\n\t/* 😀 */ datatype Thing;\n}\n"}
	idx := symbols.NewIndex()
	if err := libs.NewLoader(src, nil).LoadAll(idx); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	ws := model.NewWorkspaceWithIndex(idx, model.WithLibrarySource(src))
	s := NewServer(ws)
	name := uri.File("/tmp/utf16.sysml").Filename()
	text := "package P { attribute x : Fake::Thing; }\n"
	ws.Open(name, []byte(text), 1)

	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(text), strings.Index(text, "Thing")),
		},
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	if locs[0].URI != libraryURI(libName) {
		t.Errorf("URI = %q, want %q", locs[0].URI, libraryURI(libName))
	}
	// "\t/* 😀 */ " is 10 UTF-16 code units (13 bytes): the emoji is a surrogate pair.
	want := protocol.Position{Line: 1, Character: 10}
	if locs[0].Range.Start != want {
		t.Errorf("start = %+v, want %+v", locs[0].Range.Start, want)
	}
	res, err := s.StdlibContent(&stdlibContentParams{URI: locs[0].URI})
	if err != nil || res.Text != src[libName] {
		t.Errorf("StdlibContent = (%v, %v), want the library text", res, err)
	}
}

// The whole round trip over one JSON-RPC stream: the capability is advertised,
// definition lands in a library URI, the content request serves the text, a
// change to it is refused, and requests against the URI still answer.
func TestRunServesLibraryDocumentsOverStream(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	s := NewServer(model.NewWorkspace())
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background(), server) }()
	if err := client.SetDeadline(time.Now().Add(60 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	r := bufio.NewReader(client)

	// Messages are written in order by one goroutine: net.Pipe is unbuffered and
	// the server may notify before it reads.
	outbox := make(chan map[string]any, 64)
	go func() {
		for msg := range outbox {
			_ = writeMessage(client, msg)
		}
	}()
	t.Cleanup(func() { close(outbox) })
	send := func(msg map[string]any) { outbox <- msg }
	// await reads until the response to id arrives, collecting the
	// notifications passed on the way by method.
	await := func(id int, notes map[string][]map[string]any) map[string]any {
		t.Helper()
		for {
			msg := readMessage(t, r)
			if got, ok := msg["id"].(float64); ok && int(got) == id {
				return msg
			}
			if method, ok := msg["method"].(string); ok && notes != nil {
				notes[method] = append(notes[method], msg)
			}
		}
	}

	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"capabilities": map[string]any{}}})
	res := await(1, nil)
	caps := res["result"].(map[string]any)["capabilities"].(map[string]any)
	experimental, _ := caps["experimental"].(map[string]any)
	if experimental["openSysmlStdlibContent"] != true {
		t.Errorf("experimental = %v, want openSysmlStdlibContent: true", experimental)
	}
	send(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})

	src := "package P { attribute x : ScalarValues::Integer; }\n"
	send(map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{
			"uri": "file:///tmp/stream.sysml", "languageId": "sysml", "version": 1, "text": src,
		}},
	})
	pos := offsetToPosition([]byte(src), strings.Index(src, "Integer"))
	send(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "textDocument/definition",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": "file:///tmp/stream.sysml"},
			"position":     map[string]any{"line": pos.Line, "character": pos.Character},
		},
	})
	res = await(2, nil)
	locs, _ := res["result"].([]any)
	if len(locs) != 1 {
		t.Fatalf("definition result = %v, want one location", res["result"])
	}
	loc := locs[0].(map[string]any)
	libURI, _ := loc["uri"].(string)
	if libURI != string(libraryURI(scalarValuesName)) {
		t.Fatalf("definition uri = %q, want %q", libURI, libraryURI(scalarValuesName))
	}
	text := libraryText(t, scalarValuesName)
	if line := loc["range"].(map[string]any)["start"].(map[string]any)["line"].(float64); uint32(line) != lineOf(t, text, "datatype Integer") {
		t.Errorf("definition line = %v, want the datatype Integer line", line)
	}

	send(map[string]any{"jsonrpc": "2.0", "id": 3, "method": MethodStdlibContent, "params": map[string]any{"uri": libURI}})
	res = await(3, nil)
	if got, _ := res["result"].(map[string]any)["text"].(string); got != text {
		t.Errorf("%s returned %d bytes, want the bundled ScalarValues.kerml (%d bytes)", MethodStdlibContent, len(got), len(text))
	}

	send(map[string]any{"jsonrpc": "2.0", "id": 4, "method": MethodStdlibContent, "params": map[string]any{"uri": "file:///tmp/stream.sysml"}})
	res = await(4, nil)
	if res["error"] == nil {
		t.Errorf("%s on a file URI answered %v, want an error", MethodStdlibContent, res["result"])
	}

	send(map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{"textDocument": map[string]any{"uri": libURI, "languageId": "kerml", "version": 1, "text": text}},
	})
	send(map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didChange",
		"params": map[string]any{
			"textDocument":   map[string]any{"uri": libURI, "version": 2},
			"contentChanges": []map[string]any{{"text": "package Bogus;"}},
		},
	})
	// Hover on the declaration: answered from the bundled text, the change not applied.
	declPos := offsetToPosition([]byte(text), strings.Index(text, "datatype Integer")+len("datatype "))
	send(map[string]any{
		"jsonrpc": "2.0", "id": 5, "method": "textDocument/hover",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": libURI},
			"position":     map[string]any{"line": declPos.Line, "character": declPos.Character},
		},
	})
	notes := map[string][]map[string]any{}
	res = await(5, notes)
	hover, _ := res["result"].(map[string]any)
	contents, _ := hover["contents"].(map[string]any)
	if value, _ := contents["value"].(string); !strings.Contains(value, "datatype Integer") {
		t.Errorf("hover on the library document = %v, want datatype Integer", res["result"])
	}
	var refused bool
	for _, note := range notes["window/showMessage"] {
		params, _ := note["params"].(map[string]any)
		if msg, _ := params["message"].(string); strings.Contains(msg, "read-only") {
			refused = true
		}
	}
	if !refused {
		t.Errorf("no window/showMessage refusing the change; notifications: %v", notes)
	}

	send(map[string]any{"jsonrpc": "2.0", "id": 6, "method": "shutdown"})
	await(6, nil)
	_ = client.Close()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the stream closed")
	}
}
