package lsp

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/notebook"
)

// cellLanguages are the cell language ids the server syncs: a notebook's cells
// in these languages are its model; every other cell is prose or another
// kernel's and takes no part.
var cellLanguages = []string{"sysml", "kerml"}

// notebookSyncOptions is the capability the server advertises: every notebook
// holding a SysML or KerML cell is synced, and its save forwarded.
func notebookSyncOptions() *protocol.NotebookDocumentSyncOptions {
	cells := make([]protocol.NotebookCellLanguage, len(cellLanguages))
	for i, lang := range cellLanguages {
		cells[i] = protocol.NotebookCellLanguage{Language: lang}
	}
	return &protocol.NotebookDocumentSyncOptions{
		NotebookSelector: []protocol.NotebookSelector{&protocol.NotebookDocumentFilterWithCells{Cells: cells}},
		Save:             new(true),
	}
}

// notebookDocument is an open notebook as the server analyzes it: one SysML
// document whose text is the model cells' declarations in notebook order, each
// cell's lines where the cell stands, so a position in the document names a
// cell and a position in it. A value is replaced, never changed, so a sweep
// publishing from another goroutine maps against the state it read.
type notebookDocument struct {
	uri     uri.URI
	name    string
	version int32
	cells   []*notebookCell
	source  []byte
}

// notebookCell is one cell of a notebook. Content is nil for a cell the client
// does not sync: prose, or code in another language.
type notebookCell struct {
	document uri.URI
	kind     protocol.NotebookCellKind
	language string
	content  []byte
	version  int32
	// startLine is the document line the cell's first line stands on, -1 for a
	// cell that is not in the document; lines counts the cell's lines.
	startLine int
	lines     int
}

// inSource reports whether the cell's text is part of the notebook's document.
func (c *notebookCell) inSource() bool {
	return c.content != nil && c.kind == protocol.NotebookCellKindCode && slices.Contains(cellLanguages, c.language)
}

// newNotebookDocument reads an opened notebook: its cells in order, the synced
// ones carrying their text.
func newNotebookDocument(nb protocol.NotebookDocument, texts []protocol.TextDocumentItem) *notebookDocument {
	doc := &notebookDocument{uri: nb.URI, name: uriToName(nb.URI), version: nb.Version}
	for _, cell := range nb.Cells {
		doc.cells = append(doc.cells, &notebookCell{document: cell.Document, kind: cell.Kind, startLine: -1})
	}
	doc.openCells(texts)
	doc.compose()
	return doc
}

// openCells gives the cells their synced text documents.
func (d *notebookDocument) openCells(texts []protocol.TextDocumentItem) {
	for _, text := range texts {
		if cell := d.cell(text.URI); cell != nil {
			cell.language, cell.content, cell.version = string(text.LanguageID), []byte(text.Text), text.Version
		}
	}
}

// cell is the notebook's cell whose text document is u, nil when none is.
func (d *notebookDocument) cell(u uri.URI) *notebookCell {
	for _, cell := range d.cells {
		if cell.document == u {
			return cell
		}
	}
	return nil
}

// compose lays the model cells out as one document: each cell's declarations
// as the kernel loads them, `%` command and expression lines blank, one cell
// after another with a line break between.
func (d *notebookDocument) compose() {
	var buf bytes.Buffer
	line := 0
	for _, cell := range d.cells {
		cell.startLine, cell.lines = -1, 0
		if !cell.inSource() {
			continue
		}
		text, _ := notebook.Declarations(string(cell.content))
		cell.startLine, cell.lines = line, strings.Count(text, "\n")+1
		buf.WriteString(text)
		buf.WriteByte('\n')
		line += cell.lines
	}
	d.source = buf.Bytes()
}

// changed is the notebook after a change event: its cell list spliced, its
// cells' properties and texts updated, and its document composed again.
func (d *notebookDocument) changed(version int32, change protocol.NotebookDocumentChangeEvent) *notebookDocument {
	next := &notebookDocument{uri: d.uri, name: d.name, version: version}
	for _, cell := range d.cells {
		copied := *cell
		next.cells = append(next.cells, &copied)
	}
	if change.Cells == nil {
		next.compose()
		return next
	}
	if structure := change.Cells.Structure; structure != nil {
		next.splice(structure.Array)
		next.openCells(structure.DidOpen)
		for _, closed := range structure.DidClose {
			if cell := next.cell(closed.URI); cell != nil {
				cell.content, cell.language = nil, ""
			}
		}
	}
	for _, data := range change.Cells.Data {
		if cell := next.cell(data.Document); cell != nil {
			cell.kind = data.Kind
		}
	}
	for _, text := range change.Cells.TextContent {
		cell := next.cell(text.Document.URI)
		if cell == nil || cell.content == nil {
			continue
		}
		content := bytes.Clone(cell.content)
		for _, ch := range text.Changes {
			content = applyContentChange(content, ch)
		}
		cell.content, cell.version = content, text.Document.Version
	}
	next.compose()
	return next
}

// splice applies an array change to the cell list: a cell the change keeps by
// its document keeps its text, so a move is not a reopen.
func (d *notebookDocument) splice(array protocol.NotebookCellArrayChange) {
	start := min(int(array.Start), len(d.cells))
	end := min(start+int(array.DeleteCount), len(d.cells))
	inserted := make([]*notebookCell, len(array.Cells))
	for i, cell := range array.Cells {
		inserted[i] = &notebookCell{document: cell.Document, kind: cell.Kind, startLine: -1}
		if kept := d.cell(cell.Document); kept != nil {
			inserted[i].language, inserted[i].content, inserted[i].version = kept.language, kept.content, kept.version
		}
	}
	d.cells = slices.Concat(d.cells[:start], inserted, d.cells[end:])
}

// syncedCells are the documents of the cells whose text the server holds, in
// notebook order: the cells diagnostics are published to.
func (d *notebookDocument) syncedCells() []uri.URI {
	var out []uri.URI
	for _, cell := range d.cells {
		if cell.inSource() {
			out = append(out, cell.document)
		}
	}
	return out
}

// cellAt is the cell a document line stands in: the last to begin at or
// before it, nil before the first.
func (d *notebookDocument) cellAt(line int) *notebookCell {
	var found *notebookCell
	for _, cell := range d.cells {
		if cell.startLine >= 0 && cell.startLine <= line {
			found = cell
		}
	}
	return found
}

// toCell maps a document position to the cell holding it and the position in
// the cell; a position on the break after a cell's last line is the cell's end.
func (d *notebookDocument) toCell(pos protocol.Position) (uri.URI, protocol.Position, bool) {
	cell := d.cellAt(int(pos.Line))
	if cell == nil {
		return "", protocol.Position{}, false
	}
	return cell.document, cell.clamp(pos), true
}

// clamp is pos, a document position in or just after the cell, as a position
// in the cell's text.
func (c *notebookCell) clamp(pos protocol.Position) protocol.Position {
	line := int(pos.Line) - c.startLine
	if line >= c.lines {
		return positionsFor(c.content).position(len(c.content))
	}
	return protocol.Position{Line: uint32Clamp(line), Character: pos.Character}
}

// toCellRange maps a document range to the cell its start stands in; an end
// past the cell is the cell's end, since a range names text of one cell.
func (d *notebookDocument) toCellRange(r protocol.Range) (uri.URI, protocol.Range, bool) {
	cell := d.cellAt(int(r.Start.Line))
	if cell == nil {
		return "", protocol.Range{}, false
	}
	return cell.document, protocol.Range{Start: cell.clamp(r.Start), End: cell.clamp(r.End)}, true
}

// fromCell maps a position in a cell to the document position, reporting
// whether the cell is in the document.
func (d *notebookDocument) fromCell(u uri.URI, pos protocol.Position) (protocol.Position, bool) {
	cell := d.cell(u)
	if cell == nil || !cell.inSource() {
		return protocol.Position{}, false
	}
	return protocol.Position{Line: uint32Clamp(cell.startLine + int(pos.Line)), Character: pos.Character}, true
}

// notebooks is the server's open notebooks, by document name and by cell.
type notebooks struct {
	mu     sync.RWMutex
	byName map[string]*notebookDocument
	byCell map[uri.URI]*notebookDocument
}

// set records nb as the state of its notebook, replacing the one before.
func (n *notebooks) set(nb *notebookDocument) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.byName == nil {
		n.byName, n.byCell = map[string]*notebookDocument{}, map[uri.URI]*notebookDocument{}
	}
	if old := n.byName[nb.name]; old != nil {
		for _, cell := range old.cells {
			delete(n.byCell, cell.document)
		}
	}
	n.byName[nb.name] = nb
	for _, cell := range nb.cells {
		n.byCell[cell.document] = nb
	}
}

// remove forgets the notebook named, returning what was held.
func (n *notebooks) remove(name string) *notebookDocument {
	n.mu.Lock()
	defer n.mu.Unlock()
	nb := n.byName[name]
	if nb != nil {
		for _, cell := range nb.cells {
			delete(n.byCell, cell.document)
		}
		delete(n.byName, name)
	}
	return nb
}

// named is the open notebook of the document name, nil when name is a file.
func (n *notebooks) named(name string) *notebookDocument {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.byName[name]
}

// owning is the open notebook whose cell u is, nil when u is no cell.
func (n *notebooks) owning(u uri.URI) *notebookDocument {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.byCell[u]
}

// any reports whether a notebook is open, so a result may name one.
func (n *notebooks) any() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.byName) > 0
}

// DidOpenNotebookDocument opens a notebook as one document of its model cells,
// indexed with the directory it lives in so imports of files beside it resolve
// as the kernel's do, and publishes each cell's diagnostics.
func (s *Server) DidOpenNotebookDocument(ctx context.Context, params *protocol.DidOpenNotebookDocumentParams) error {
	nb := newNotebookDocument(params.NotebookDocument, params.CellTextDocuments)
	s.notebooks.set(nb)
	before := s.ws.Generation()
	s.debugEdit(ctx, "", func() {
		s.ws.OpenWithKind(nb.name, nb.source, int(nb.version), source.KindSysML)
		s.indexOpenedDirectory(nb.name)
	})
	s.publishDiagnostics(ctx, nb.name)
	if s.ws.Generation() != before {
		s.queueOpenDiagnostics(ctx, nb.name)
	}
	return nil
}

// DidChangeNotebookDocument re-analyzes the notebook after cells were edited,
// added, removed or moved, publishing every cell's diagnostics again and
// withdrawing those of a cell that left the document.
func (s *Server) DidChangeNotebookDocument(ctx context.Context, params *protocol.DidChangeNotebookDocumentParams) error {
	old := s.notebooks.named(uriToName(params.NotebookDocument.URI))
	if old == nil {
		return nil
	}
	nb := old.changed(params.NotebookDocument.Version, params.Change)
	s.notebooks.set(nb)
	s.debugEdit(ctx, "", func() { s.ws.Update(nb.name, nb.source, int(nb.version)) })
	kept := nb.syncedCells()
	for _, cell := range old.syncedCells() {
		if !slices.Contains(kept, cell) {
			s.clearCellDiagnostics(ctx, cell)
		}
	}
	s.publishDiagnostics(ctx, nb.name)
	s.queueOpenDiagnostics(ctx, nb.name)
	return nil
}

// DidSaveNotebookDocument refreshes every open document, as a file's save does:
// the notebook on disk is what the other documents' imports read.
func (s *Server) DidSaveNotebookDocument(ctx context.Context, params *protocol.DidSaveNotebookDocumentParams) error {
	name := uriToName(params.NotebookDocument.URI)
	if s.notebooks.named(name) == nil {
		return nil
	}
	s.publishDiagnostics(ctx, name)
	s.refreshOpenDiagnostics(ctx, name)
	return nil
}

// DidCloseNotebookDocument drops the notebook's document and symbols and
// withdraws every cell's diagnostics.
func (s *Server) DidCloseNotebookDocument(ctx context.Context, params *protocol.DidCloseNotebookDocumentParams) error {
	nb := s.notebooks.remove(uriToName(params.NotebookDocument.URI))
	if nb == nil {
		return nil
	}
	s.debugEdit(ctx, nb.name, func() {
		s.ws.Close(nb.name)
		s.releaseOpenedDirectories(nb.name)
	})
	for _, cell := range nb.syncedCells() {
		s.clearCellDiagnostics(ctx, cell)
	}
	s.queueOpenDiagnostics(ctx, nb.name)
	return nil
}

// clearCellDiagnostics withdraws a cell's markers.
func (s *Server) clearCellDiagnostics(ctx context.Context, cell uri.URI) {
	if s.client == nil {
		return
	}
	s.pubMu.Lock()
	defer s.pubMu.Unlock()
	_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{URI: cell, Diagnostics: []protocol.Diagnostic{}})
}

// sendNotebookDiagnosticsLocked publishes the notebook's diagnostics to the
// cells they stand in, every synced cell receiving its set so a cell whose
// problems were fixed is cleared. Caller holds pubMu.
func (s *Server) sendNotebookDiagnosticsLocked(ctx context.Context, nb *notebookDocument) {
	sets := map[uri.URI][]protocol.Diagnostic{}
	cells := nb.syncedCells()
	for _, cell := range cells {
		sets[cell] = []protocol.Diagnostic{}
	}
	if content, diags, ok := s.ws.AnalyzedContent(nb.name); ok {
		for _, f := range s.ws.FindingsOn(nb.name) {
			diags = append(diags, f.Diagnostic)
		}
		pos := positionsFor(content)
		for _, d := range diags {
			ld := protocolDiagnostic(pos, d)
			if cell, rng, ok := nb.toCellRange(ld.Range); ok {
				ld.Range = rng
				sets[cell] = append(sets[cell], ld)
			}
		}
	}
	for _, cell := range cells {
		_ = s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{URI: cell, Diagnostics: sets[cell]})
	}
	s.queueRenderChanged(ctx, nb.name)
}
