package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

const (
	// importedLibrary declares Base on line 4, past the last line of any cell.
	importedLibrary = "// The library a model imports.\n\npackage Lib {\n    part def Other;\n    part def Base;\n}\n"
	importingModel  = "private import Lib::*;\npart def D :> Base;\n"
	baseInLibrary   = `"range":{"end":{"character":18,"line":4},"start":{"character":4,"line":4}}`
	baseNameInLib   = `"range":{"end":{"character":17,"line":4},"start":{"character":13,"line":4}}`
	baseInModel     = `"range":{"end":{"character":18,"line":1},"start":{"character":14,"line":1}}`
)

// importingSession is a session whose model imports lib.sysml beside it: text
// is the document a request is asked on, the model as a file or as a cell.
type importingSession struct {
	*wireSession
	text, library string
}

// importingSessions opens the same importing model twice, each beside its own
// copy of the library: as a .sysml file and as the second cell of a notebook.
func importingSessions(t *testing.T) map[string]*importingSession {
	t.Helper()
	library := func(dir string) string {
		path := filepath.Join(dir, "lib.sysml")
		if err := os.WriteFile(path, []byte(importedLibrary), 0o600); err != nil {
			t.Fatal(err)
		}
		return string(uri.File(path))
	}
	fileDir, notebookDir := t.TempDir(), t.TempDir()
	file := newWireSession(t)
	file.uri = string(uri.File(filepath.Join(fileDir, "model.sysml")))
	fileLibrary := library(fileDir)
	file.open(importingModel)
	notebookLibrary := library(notebookDir)
	nb := openCellsIn(t, notebookDir, []cellSpec{{"sysml", "part def Local;\n"}, {"sysml", importingModel}})
	for i, cell := range nb.cells {
		if diags := nb.awaitDiagnostics(cell); len(diags) != 0 {
			t.Fatalf("cell %d: %s", i, asJSON(diags))
		}
	}
	return map[string]*importingSession{
		"file":     {file, file.uri, fileLibrary},
		"notebook": {nb.wireSession, nb.cells[1], notebookLibrary},
	}
}

// ask is a request at a position of document, extra being its further params.
func (s *importingSession) ask(method, document string, line, character int, extra string) string {
	s.t.Helper()
	return asJSON(s.call(method, fmt.Sprintf(
		`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}%s}`, document, line, character, extra)))
}

// A definition in an imported file is answered at its place in that file, from
// a notebook cell as from a file: the file's range is not a range of the notebook.
func TestDefinitionInAnImportedFileFromACellAndAFile(t *testing.T) {
	for name, s := range importingSessions(t) {
		t.Run(name, func(t *testing.T) {
			got := s.ask("textDocument/definition", s.text, 1, 15, "")
			if want := fmt.Sprintf(`[{%s,"uri":%q}]`, baseInLibrary, s.library); got != want {
				t.Errorf("definition of Base = %s, want %s", got, want)
			}
		})
	}
}

// References found from the importing model name the imported file's
// declaration where it is, and the model's own reference in the model.
func TestReferencesIntoAnImportedFileFromACellAndAFile(t *testing.T) {
	for name, s := range importingSessions(t) {
		t.Run(name, func(t *testing.T) {
			got := s.ask("textDocument/references", s.text, 1, 15, `,"context":{"includeDeclaration":true}`)
			for _, want := range []string{
				fmt.Sprintf(`{%s,"uri":%q}`, baseInLibrary, s.library),
				fmt.Sprintf(`{%s,"uri":%q}`, baseInModel, s.text),
			} {
				if !strings.Contains(got, want) {
					t.Errorf("references to Base = %s, want %s among them", got, want)
				}
			}
		})
	}
}

// References found from the imported file name the model's reference in the
// document the editor holds: the cell, at the cell's line.
func TestReferencesFromAnImportedFileIntoACellAndAFile(t *testing.T) {
	for name, s := range importingSessions(t) {
		t.Run(name, func(t *testing.T) {
			got := s.ask("textDocument/references", s.library, 4, 14, `,"context":{"includeDeclaration":true}`)
			for _, want := range []string{
				fmt.Sprintf(`{%s,"uri":%q}`, baseInLibrary, s.library),
				fmt.Sprintf(`{%s,"uri":%q}`, baseInModel, s.text),
			} {
				if !strings.Contains(got, want) {
					t.Errorf("references to Base = %s, want %s among them", got, want)
				}
			}
		})
	}
}

// A rename asked in the importing model edits the imported file's declaration
// where it is and the model's reference in the model.
func TestRenameAcrossAnImportedFileFromACellAndAFile(t *testing.T) {
	for name, s := range importingSessions(t) {
		t.Run(name, func(t *testing.T) {
			prepared := s.ask("textDocument/prepareRename", s.text, 1, 15, "")
			if !strings.Contains(prepared, strings.TrimPrefix(baseInModel, `"range":`)) {
				t.Errorf("prepareRename on Base = %s, want its range in the model", prepared)
			}
			got := s.ask("textDocument/rename", s.text, 1, 15, `,"newName":"Root"`)
			for _, want := range []string{
				fmt.Sprintf(`%q:[{"newText":"Root",%s}]`, s.library, baseNameInLib),
				fmt.Sprintf(`%q:[{"newText":"Root",%s}]`, s.text, baseInModel),
			} {
				if !strings.Contains(got, want) {
					t.Errorf("rename of Base = %s, want %s in it", got, want)
				}
			}
		})
	}
}

// Hover on the imported name is on the name in the document asked.
func TestHoverOnAnImportedNameFromACellAndAFile(t *testing.T) {
	for name, s := range importingSessions(t) {
		t.Run(name, func(t *testing.T) {
			got := s.ask("textDocument/hover", s.text, 1, 15, "")
			if !strings.Contains(got, "part def Base") || !strings.Contains(got, baseInModel) {
				t.Errorf("hover on Base = %s, want part def Base over %s", got, baseInModel)
			}
		})
	}
}

// An answer naming another file keeps that file's ranges, whatever its shape;
// the ranges beside them of the asked cell's text still move to the cell.
func TestCellResultKeepsTheRangesOfAnotherFile(t *testing.T) {
	sessions := importingSessions(t)
	s := sessions["notebook"]
	nb := s.s.notebooks.owning(uri.URI(s.text))
	if nb == nil {
		t.Fatal("the cell's notebook is not open")
	}
	inFile := `{"start":{"line":4,"character":4},"end":{"line":4,"character":18}}`
	inDocument := `{"start":{"line":3,"character":14},"end":{"line":3,"character":18}}`
	inCell := `{"end":{"character":18,"line":1},"start":{"character":14,"line":1}}`
	kept := `{"end":{"character":18,"line":4},"start":{"character":4,"line":4}}`
	for name, tc := range map[string]struct {
		result, want string
		// anywhere is whether a request on a file is answered the same.
		anywhere bool
	}{
		"location link": {
			fmt.Sprintf(`{"originSelectionRange":%s,"targetUri":%q,"targetRange":%s,"targetSelectionRange":%s}`, inDocument, s.library, inFile, inFile),
			fmt.Sprintf(`{"originSelectionRange":%s,"targetRange":%s,"targetSelectionRange":%s,"targetUri":%q}`, inCell, kept, kept, s.library), false,
		},
		"hierarchy item": {
			fmt.Sprintf(`{"name":"Base","uri":%q,"range":%s,"selectionRange":%s}`, s.library, inFile, inFile),
			fmt.Sprintf(`{"name":"Base","range":%s,"selectionRange":%s,"uri":%q}`, kept, kept, s.library), false,
		},
		"related location": {
			fmt.Sprintf(`{"range":%s,"relatedInformation":[{"location":{"uri":%q,"range":%s},"message":"m"}]}`, inDocument, s.library, inFile),
			fmt.Sprintf(`{"range":%s,"relatedInformation":[{"location":{"range":%s,"uri":%q},"message":"m"}]}`, inCell, kept, s.library), false,
		},
		"link into the notebook": {
			fmt.Sprintf(`{"targetUri":%q,"targetRange":%s,"targetSelectionRange":%s}`, nameToURI(nb.name), inDocument, inDocument),
			fmt.Sprintf(`{"targetRange":%s,"targetSelectionRange":%s,"targetUri":%q}`, inCell, inCell, s.text), true,
		},
		"item in the notebook": {
			fmt.Sprintf(`{"uri":%q,"range":%s,"selectionRange":%s}`, nameToURI(nb.name), inDocument, inDocument),
			fmt.Sprintf(`{"range":%s,"selectionRange":%s,"uri":%q}`, inCell, inCell, s.text), true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			for asked, implicit := range map[string]*notebookDocument{"a cell": nb, "a file": nil} {
				if implicit == nil && !tc.anywhere {
					continue
				}
				var value any
				if err := json.Unmarshal([]byte(tc.result), &value); err != nil {
					t.Fatal(err)
				}
				if got := asJSON(s.s.mapValue(value, implicit)); got != tc.want {
					t.Errorf("asked on %s: mapped %s\n got %s\nwant %s", asked, tc.result, got, tc.want)
				}
			}
		})
	}
}
