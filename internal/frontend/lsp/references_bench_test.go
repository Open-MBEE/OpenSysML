package lsp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// benchWorkspaceDocs is the size of the synthetic workspace the benchmarks
// open: a core document plus that many chained ones, each importing the core
// and its predecessor and naming both.
const benchWorkspaceDocs = 120

func benchDocName(i int) string {
	return uri.File(fmt.Sprintf("/tmp/refbench/p%03d.sysml", i)).Filename()
}

func benchDocSource(i int) string {
	if i == 0 {
		return "package Core {\n\tpart def Hub;\n\tpart def Spoke;\n\tpart def D000 :> Hub;\n}\n"
	}
	prev := i - 1
	return fmt.Sprintf(
		"package P%03d {\n\timport Core::*;\n\timport %s::*;\n"+
			"\tpart def D%03d :> D%03d;\n\tpart h : Hub;\n\tpart s : Core::Spoke;\n"+
			"\tpart d : %s::D%03d;\n\tpart def W%03d { part hub : Hub; part w : D%03d; }\n}\n",
		i, benchPackage(prev), i, prev, benchPackage(prev), prev, i, i)
}

func benchPackage(i int) string {
	if i == 0 {
		return "Core"
	}
	return fmt.Sprintf("P%03d", i)
}

// openBenchWorkspace opens the synthetic workspace and returns the server plus
// the position of the widely-used declaration `Hub`.
func openBenchWorkspace(b *testing.B) (*model.Workspace, *Server, protocol.TextDocumentPositionParams) {
	b.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	for i := 0; i <= benchWorkspaceDocs; i++ {
		ws.Open(benchDocName(i), []byte(benchDocSource(i)), 1)
	}
	core := benchDocSource(0)
	pos := protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(benchDocName(0))},
		Position:     offsetToPosition([]byte(core), strings.Index(core, "Hub")),
	}
	return ws, s, pos
}

func benchReferences(b *testing.B, s *Server, pos protocol.TextDocumentPositionParams) int {
	b.Helper()
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{TextDocumentPositionParams: pos})
	if err != nil {
		b.Fatal(err)
	}
	if len(locs) < benchWorkspaceDocs {
		b.Fatalf("references = %d, want at least %d", len(locs), benchWorkspaceDocs)
	}
	return len(locs)
}

// BenchmarkReferencesWarm answers repeated references queries on an unchanged
// workspace.
func BenchmarkReferencesWarm(b *testing.B) {
	_, s, pos := openBenchWorkspace(b)
	benchReferences(b, s, pos)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchReferences(b, s, pos)
	}
}

// BenchmarkReferencesCold answers a references query right after an edit to
// one document of the workspace.
func BenchmarkReferencesCold(b *testing.B) {
	ws, s, pos := openBenchWorkspace(b)
	edited := benchDocName(benchWorkspaceDocs / 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		ws.Update(edited, []byte(benchDocSource(benchWorkspaceDocs/2)+fmt.Sprintf("// %d\n", i)), i+2)
		b.StartTimer()
		benchReferences(b, s, pos)
	}
}

// BenchmarkRenameWarm answers repeated rename queries on an unchanged workspace.
func BenchmarkRenameWarm(b *testing.B) {
	_, s, pos := openBenchWorkspace(b)
	benchReferences(b, s, pos)
	params := &protocol.RenameParams{TextDocumentPositionParams: pos, NewName: "Axle"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		edit, err := s.Rename(context.Background(), params)
		if err != nil {
			b.Fatal(err)
		}
		if len(edit.Changes) < benchWorkspaceDocs {
			b.Fatalf("rename touched %d documents, want at least %d", len(edit.Changes), benchWorkspaceDocs)
		}
	}
}

// BenchmarkWorkspaceUpdate is the didChange path: applying one document's new
// text to the workspace, which must not build the index.
func BenchmarkWorkspaceUpdate(b *testing.B) {
	ws, s, pos := openBenchWorkspace(b)
	benchReferences(b, s, pos)
	edited := benchDocName(benchWorkspaceDocs / 2)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ws.Update(edited, []byte(benchDocSource(benchWorkspaceDocs/2)+fmt.Sprintf("// %d\n", i)), i+2)
	}
}
