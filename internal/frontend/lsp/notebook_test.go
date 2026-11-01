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
	return openCells(t, []cellSpec{{"sysml", cellPackage}, {"sysml", cellUsage}, {"markdown", ""}, {"sysml", cellMixed}})
}

// cellSpec is a cell to open: its language and text; a Markdown cell is not synced.
type cellSpec struct{ language, text string }

func openCells(t *testing.T, specs []cellSpec) *notebookSession {
	t.Helper()
	w := newWireSession(t)
	nb := &notebookSession{wireSession: w, uri: string(uri.File(filepath.Join(t.TempDir(), "model.ipynb")))}
	var cells, texts []string
	for i, spec := range specs {
		nb.cells = append(nb.cells, fmt.Sprintf("vscode-notebook-cell:/model.ipynb#cell%d", i))
		if spec.language == "markdown" {
			cells = append(cells, fmt.Sprintf(`{"kind": 1, "document": %q}`, nb.cells[i]))
			continue
		}
		cells = append(cells, fmt.Sprintf(`{"kind": 2, "document": %q}`, nb.cells[i]))
		texts = append(texts, fmt.Sprintf(`{"uri": %q, "languageId": %q, "version": 1, "text": %s}`, nb.cells[i], spec.language, jsonString(spec.text)))
	}
	w.call("initialize", `{"capabilities":{}}`)
	w.notify("initialized", `{}`)
	w.notify("notebookDocument/didOpen", fmt.Sprintf(`{
		"notebookDocument": {"uri": %q, "notebookType": "jupyter-notebook", "version": 1, "cells": [%s]},
		"cellTextDocuments": [%s]}`, nb.uri, strings.Join(cells, ", "), strings.Join(texts, ", ")))
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

// A declaration on a later line of a later cell is found at that line of that
// cell: the location is mapped once, with the document its URI names.
func TestNotebookLocationsInLaterCellsMapOnce(t *testing.T) {
	nb := openNotebook(t)
	for _, cell := range []int{0, 1, 3} {
		nb.awaitDiagnostics(nb.cells[cell])
	}
	late := "vscode-notebook-cell:/model.ipynb#late"
	nb.change(2, fmt.Sprintf(`{"structure": {
		"array": {"start": 4, "deleteCount": 0, "cells": [{"kind": 2, "document": %q}]},
		"didOpen": [{"uri": %q, "languageId": "sysml", "version": 1, "text": %s}]}}`,
		late, late, jsonString("\n\npart def Missing;\n")))
	nb.awaitDiagnostics(late)
	definition := asJSON(nb.callOn("textDocument/definition", 3, 2, 10))
	if !strings.Contains(definition, fmt.Sprintf(`"uri":%q`, late)) || !strings.Contains(definition, `"start":{"character":0,"line":2}`) {
		t.Errorf("definition of Missing, declared on line 2 of the last cell: %s", definition)
	}
	references := asJSON(nb.call("textDocument/references", fmt.Sprintf(
		`{"textDocument":{"uri":%q},"position":{"line":2,"character":10},"context":{"includeDeclaration":true}}`, late)))
	if !strings.Contains(references, fmt.Sprintf(`"uri":%q`, nb.cells[3])) || !strings.Contains(references, `"start":{"character":9,"line":2}`) {
		t.Errorf("references to Missing from its declaration: %s", references)
	}
}

// A package opened in one cell and closed in the next: each cell's outline
// holds what is named in it, the member lifted out of the package it is in.
func TestNotebookDocumentSymbolsFollowTheCellTheyAreNamedIn(t *testing.T) {
	nb := openCells(t, []cellSpec{{"sysml", "package P {\n"}, {"sysml", "  part def A;\n}\n"}})
	nb.awaitDiagnostics(nb.cells[0])
	nb.awaitDiagnostics(nb.cells[1])
	first := asJSON(nb.call("textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, nb.cells[0])))
	if !strings.Contains(first, `"name":"P"`) || strings.Contains(first, `"name":"A"`) || !strings.Contains(first, `"end":{"character":0,"line":1}`) {
		t.Errorf("symbols of the cell opening the package: %s", first)
	}
	second := asJSON(nb.call("textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, nb.cells[1])))
	if !strings.Contains(second, `"name":"A"`) || strings.Contains(second, `"name":"P"`) {
		t.Errorf("symbols of the cell declaring the member: %s", second)
	}
}

// A notebook's language is its first model cell's; a cell in the other
// language is told it is not part of the model rather than misparsed.
func TestNotebookLanguageFollowsItsFirstModelCell(t *testing.T) {
	nb := openCells(t, []cellSpec{{"kerml", "classifier C;\n"}, {"kerml", "feature f : C;\n"}, {"sysml", "part def D;\n"}})
	if diags := nb.awaitDiagnostics(nb.cells[0]); len(diags) != 0 {
		t.Errorf("KerML cell parsed as KerML: %s", asJSON(diags))
	}
	if diags := nb.awaitDiagnostics(nb.cells[1]); len(diags) != 0 {
		t.Errorf("KerML cell referencing the first: %s", asJSON(diags))
	}
	diags := nb.awaitDiagnostics(nb.cells[2])
	if len(diags) != 1 || !strings.Contains(asJSON(diags), "a sysml cell in a kerml notebook") {
		t.Errorf("SysML cell in a KerML notebook: %s", asJSON(diags))
	}
	if nb.callOn("textDocument/hover", 2, 0, 10) != nil {
		t.Error("a cell outside the model answers hover")
	}
}

// Removing the SysML cell above an identical KerML cell changes the notebook's
// language though not its text: the text is parsed again, as KerML.
func TestNotebookLanguageChangeReparsesUnchangedText(t *testing.T) {
	nb := openCells(t, []cellSpec{{"sysml", "part def A;\n"}, {"kerml", "part def A;\n"}})
	if diags := nb.awaitDiagnostics(nb.cells[0]); len(diags) != 0 {
		t.Errorf("SysML cell: %s", asJSON(diags))
	}
	if diags := nb.awaitDiagnostics(nb.cells[1]); len(diags) != 1 || !strings.Contains(asJSON(diags), "kerml cell in a sysml notebook") {
		t.Errorf("KerML cell in a SysML notebook: %s", asJSON(diags))
	}
	nb.change(2, fmt.Sprintf(`{"structure": {"array": {"start": 0, "deleteCount": 1, "cells": []}, "didClose": [{"uri": %q}]}}`, nb.cells[0]))
	nb.awaitDiagnostics(nb.cells[0])
	diags := nb.awaitDiagnostics(nb.cells[1])
	if len(diags) == 0 || strings.Contains(asJSON(diags), "notebook") {
		t.Errorf("part def in a KerML notebook: want KerML syntax diagnostics, got %s", asJSON(diags))
	}
}
