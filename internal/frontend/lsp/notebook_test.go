package lsp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

// notebookSession is a wire session with a notebook open: a package cell, a
// usage cell referencing it, a Markdown cell the client does not sync, and a
// cell mixing a kernel command and an expression with a declaration.
type notebookSession struct {
	*wireSession
	uri   string
	cells []string
}

const (
	cellPackage = "package P { part def A; }\n"
	cellUsage   = "part a : P::A;\n"
	cellMixed   = "%help\n1 + 2\npart b : Missing;\n"
)

func openNotebook(t *testing.T) *notebookSession {
	t.Helper()
	w := newWireSession(t)
	nb := &notebookSession{wireSession: w, uri: string(uri.File(filepath.Join(t.TempDir(), "model.ipynb")))}
	for i := range 4 {
		nb.cells = append(nb.cells, fmt.Sprintf("vscode-notebook-cell:/model.ipynb#cell%d", i))
	}
	w.call("initialize", `{"capabilities":{}}`)
	w.notify("initialized", `{}`)
	w.notify("notebookDocument/didOpen", fmt.Sprintf(`{
		"notebookDocument": {"uri": %q, "notebookType": "jupyter-notebook", "version": 1, "cells": [
			{"kind": 2, "document": %q}, {"kind": 2, "document": %q}, {"kind": 1, "document": %q}, {"kind": 2, "document": %q}]},
		"cellTextDocuments": [
			{"uri": %q, "languageId": "sysml", "version": 1, "text": %s},
			{"uri": %q, "languageId": "sysml", "version": 1, "text": %s},
			{"uri": %q, "languageId": "sysml", "version": 1, "text": %s}]}`,
		nb.uri, nb.cells[0], nb.cells[1], nb.cells[2], nb.cells[3],
		nb.cells[0], jsonString(cellPackage), nb.cells[1], jsonString(cellUsage), nb.cells[3], jsonString(cellMixed)))
	return nb
}

// change sends a notebookDocument/didChange with the given cells change.
func (nb *notebookSession) change(version int, cells string) {
	nb.t.Helper()
	nb.notify("notebookDocument/didChange", fmt.Sprintf(
		`{"notebookDocument": {"uri": %q, "version": %d}, "change": {"cells": %s}}`, nb.uri, version, cells))
}

// callOn is a textDocument request on a cell at a position.
func (nb *notebookSession) callOn(method string, cell int, line, character int) any {
	nb.t.Helper()
	return nb.call(method, fmt.Sprintf(
		`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, nb.cells[cell], line, character))
}

func diagnosticLines(diags []any) []int {
	var out []int
	for _, d := range diags {
		fields, _ := d.(map[string]any)
		rng, _ := fields["range"].(map[string]any)
		start, _ := rng["start"].(map[string]any)
		line, _ := start["line"].(float64)
		out = append(out, int(line))
	}
	return out
}

func asJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestInitializeAdvertisesNotebookSync(t *testing.T) {
	w := newWireSession(t)
	result := w.call("initialize", `{"capabilities":{}}`)
	got := asJSON(result)
	for _, want := range []string{`"notebookDocumentSync"`, `"cells":[{"language":"sysml"},{"language":"kerml"}]`, `"save":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("initialize result lacks %s: %s", want, got)
		}
	}
}

// A notebook's cells see each other in order, kernel command lines and
// expressions are not parsed, and each diagnostic lands in its cell's lines.
func TestNotebookOpenPublishesEachCellsDiagnostics(t *testing.T) {
	nb := openNotebook(t)
	if diags := nb.awaitDiagnostics(nb.cells[0]); len(diags) != 0 {
		t.Errorf("package cell: %s", asJSON(diags))
	}
	if diags := nb.awaitDiagnostics(nb.cells[1]); len(diags) != 0 {
		t.Errorf("usage cell referencing the package cell: %s", asJSON(diags))
	}
	diags := nb.awaitDiagnostics(nb.cells[3])
	if lines := diagnosticLines(diags); len(lines) != 1 || lines[0] != 2 || !strings.Contains(asJSON(diags), "Missing") {
		t.Errorf("mixed cell: want one diagnostic on line 2 about Missing, got %s", asJSON(diags))
	}
	if nb.s.ws.Document(uriToName(uri.URI(nb.uri))) == nil {
		t.Error("the notebook is not a workspace document")
	}
}

func TestNotebookReorderMovesDiagnosticsToTheCellThatLostItsContext(t *testing.T) {
	nb := openNotebook(t)
	for _, cell := range []int{0, 1, 3} {
		nb.awaitDiagnostics(nb.cells[cell])
	}
	nb.change(2, fmt.Sprintf(`{"structure": {"array": {"start": 0, "deleteCount": 2, "cells": [{"kind": 2, "document": %q}, {"kind": 2, "document": %q}]}}}`,
		nb.cells[1], nb.cells[0]))
	if diags := nb.awaitDiagnostics(nb.cells[1]); len(diags) != 1 || diagnosticLines(diags)[0] != 0 || !strings.Contains(asJSON(diags), "later cell") {
		t.Errorf("usage cell moved above the package: want one later-cell diagnostic on line 0, got %s", asJSON(diags))
	}
	if diags := nb.awaitDiagnostics(nb.cells[0]); len(diags) != 0 {
		t.Errorf("package cell after the move: %s", asJSON(diags))
	}
	nb.change(3, fmt.Sprintf(`{"structure": {"array": {"start": 0, "deleteCount": 2, "cells": [{"kind": 2, "document": %q}, {"kind": 2, "document": %q}]}}}`,
		nb.cells[0], nb.cells[1]))
	nb.awaitDiagnostics(nb.cells[0])
	if diags := nb.awaitDiagnostics(nb.cells[1]); len(diags) != 0 {
		t.Errorf("usage cell moved back below the package: %s", asJSON(diags))
	}
}

func TestNotebookCellEditRepublishesTheNotebook(t *testing.T) {
	nb := openNotebook(t)
	for _, cell := range []int{0, 1, 3} {
		nb.awaitDiagnostics(nb.cells[cell])
	}
	nb.change(2, fmt.Sprintf(`{"textContent": [{"document": {"uri": %q, "version": 2}, "changes": [
		{"range": {"start": {"line": 0, "character": 21}, "end": {"line": 0, "character": 22}}, "text": "B"}]}]}`, nb.cells[0]))
	nb.awaitDiagnostics(nb.cells[0])
	diags := nb.awaitDiagnostics(nb.cells[1])
	if len(diags) != 1 || !strings.Contains(asJSON(diags), "P::A") {
		t.Errorf("usage cell after A was renamed B in the package cell: %s", asJSON(diags))
	}
	nb.change(3, fmt.Sprintf(`{"textContent": [{"document": {"uri": %q, "version": 2}, "changes": [{"text": %s}]}]}`,
		nb.cells[1], jsonString("part a : P::B;\n")))
	nb.awaitDiagnostics(nb.cells[0])
	if diags := nb.awaitDiagnostics(nb.cells[1]); len(diags) != 0 {
		t.Errorf("usage cell replaced whole to follow the rename: %s", asJSON(diags))
	}
}

func TestNotebookInsertAndDeleteCells(t *testing.T) {
	nb := openNotebook(t)
	for _, cell := range []int{0, 1, 3} {
		nb.awaitDiagnostics(nb.cells[cell])
	}
	inserted := "vscode-notebook-cell:/model.ipynb#cell9"
	nb.change(2, fmt.Sprintf(`{"structure": {
		"array": {"start": 1, "deleteCount": 0, "cells": [{"kind": 2, "document": %q}]},
		"didOpen": [{"uri": %q, "languageId": "sysml", "version": 1, "text": %s}]}}`,
		inserted, inserted, jsonString("part def Missing;\n")))
	nb.awaitDiagnostics(nb.cells[0])
	if diags := nb.awaitDiagnostics(inserted); len(diags) != 0 {
		t.Errorf("inserted cell: %s", asJSON(diags))
	}
	nb.awaitDiagnostics(nb.cells[1])
	if diags := nb.awaitDiagnostics(nb.cells[3]); len(diags) != 0 {
		t.Errorf("mixed cell once Missing is declared above it: %s", asJSON(diags))
	}
	nb.change(3, fmt.Sprintf(`{"structure": {
		"array": {"start": 1, "deleteCount": 1, "cells": []},
		"didClose": [{"uri": %q}]}}`, inserted))
	if diags := nb.awaitDiagnostics(inserted); len(diags) != 0 {
		t.Errorf("deleted cell keeps diagnostics: %s", asJSON(diags))
	}
	nb.awaitDiagnostics(nb.cells[0])
	nb.awaitDiagnostics(nb.cells[1])
	if diags := nb.awaitDiagnostics(nb.cells[3]); len(diags) != 1 {
		t.Errorf("mixed cell once its declaration was deleted: %s", asJSON(diags))
	}
}

func TestNotebookCloseClearsEveryCellAndDropsTheDocument(t *testing.T) {
	nb := openNotebook(t)
	for _, cell := range []int{0, 1, 3} {
		nb.awaitDiagnostics(nb.cells[cell])
	}
	nb.notify("notebookDocument/didClose", fmt.Sprintf(`{"notebookDocument": {"uri": %q}, "cellTextDocuments": [{"uri": %q}, {"uri": %q}, {"uri": %q}]}`,
		nb.uri, nb.cells[0], nb.cells[1], nb.cells[3]))
	for _, cell := range []int{0, 1, 3} {
		if diags := nb.awaitDiagnostics(nb.cells[cell]); len(diags) != 0 {
			t.Errorf("cell %d after close: %s", cell, asJSON(diags))
		}
	}
	if symbols := nb.call("workspace/symbol", `{"query": "P"}`); asJSON(symbols) != "[]" && symbols != nil {
		t.Errorf("symbols survive the close: %s", asJSON(symbols))
	}
	if nb.s.ws.Document(uriToName(uri.URI(nb.uri))) != nil {
		t.Error("the notebook document survives the close")
	}
	if nb.callOn("textDocument/hover", 1, 0, 12) != nil {
		t.Error("a closed cell still answers hover")
	}
}

func TestNotebookCellRequestsAnswerInCellCoordinates(t *testing.T) {
	nb := openNotebook(t)
	for _, cell := range []int{0, 1, 3} {
		nb.awaitDiagnostics(nb.cells[cell])
	}
	hover := asJSON(nb.callOn("textDocument/hover", 1, 0, 12))
	if !strings.Contains(hover, "part def A") || !strings.Contains(hover, `"start":{"character":12,"line":0}`) {
		t.Errorf("hover on P::A in the usage cell: %s", hover)
	}
	definition := asJSON(nb.callOn("textDocument/definition", 1, 0, 12))
	if !strings.Contains(definition, fmt.Sprintf(`"uri":%q`, nb.cells[0])) || !strings.Contains(definition, `"end":{"character":23,"line":0}`) {
		t.Errorf("definition of P::A from the usage cell: %s", definition)
	}
	references := asJSON(nb.call("textDocument/references", fmt.Sprintf(
		`{"textDocument":{"uri":%q},"position":{"line":0,"character":22},"context":{"includeDeclaration":false}}`, nb.cells[0])))
	if !strings.Contains(references, fmt.Sprintf(`"uri":%q`, nb.cells[1])) || !strings.Contains(references, `"start":{"character":12,"line":0}`) {
		t.Errorf("references to A from the package cell: %s", references)
	}
	symbols := asJSON(nb.call("textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, nb.cells[1])))
	if !strings.Contains(symbols, `"name":"a"`) || strings.Contains(symbols, `"name":"P"`) {
		t.Errorf("document symbols of the usage cell: %s", symbols)
	}
	tokens, _ := nb.call("textDocument/semanticTokens/full", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, nb.cells[1])).(map[string]any)
	data, _ := tokens["data"].([]any)
	if len(data) < 5 || data[0].(float64) != 0 || data[1].(float64) != 0 {
		t.Errorf("semantic tokens of the usage cell should start at its line 0: %v", data)
	}
	completion := asJSON(nb.call("textDocument/completion", fmt.Sprintf(
		`{"textDocument":{"uri":%q},"position":{"line":0,"character":12}}`, nb.cells[1])))
	if !strings.Contains(completion, `"label":"A"`) {
		t.Errorf("completion after P:: in the usage cell: %s", completion)
	}
	if hover := nb.callOn("textDocument/hover", 3, 0, 2); hover != nil {
		t.Errorf("hover on a %%help line: %s", asJSON(hover))
	}
}
