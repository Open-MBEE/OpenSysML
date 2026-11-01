package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// cellHandler answers requests on notebook cells from the notebook's document:
// a cell's position is moved to the document before the server looks, and
// every position in an answer that names the document, this request's or any
// other's, is moved back to the cell it stands in.
func (s *Server) cellHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		if !s.notebooks.any() || !req.IsCall() {
			return inner(ctx, req)
		}
		var params struct {
			TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
		}
		var result any
		var err error
		var nb *notebookDocument
		if json.Unmarshal(req.Params(), &params) == nil {
			nb = s.notebooks.owning(params.TextDocument.URI)
		}
		if nb != nil {
			result, err = s.cellRequest(ctx, req, nb, params.TextDocument.URI)
		} else {
			result, err = inner(ctx, req)
		}
		if err != nil || result == nil {
			return result, err
		}
		return s.cellResult(req.Method(), result, nb)
	}
}

// cellRequest answers a request on the cell at u. A cell of a language the
// server does not read answers nothing.
func (s *Server) cellRequest(ctx context.Context, req *jsonrpc2.Request, nb *notebookDocument, u uri.URI) (any, error) {
	cell := nb.cell(u)
	if cell == nil || !cell.inSource() {
		return nil, nil
	}
	docURI := nameToURI(nb.name)
	at := func(pos protocol.Position) protocol.Position {
		mapped, _ := nb.fromCell(u, pos)
		return mapped
	}
	span := func(r protocol.Range) protocol.Range {
		return protocol.Range{Start: at(r.Start), End: at(r.End)}
	}
	inCell := func(r protocol.Range) bool {
		return nb.cellAt(int(r.Start.Line)) == cell
	}
	switch req.Method() {
	case protocol.MethodTextDocumentHover:
		p, err := decodeParams[protocol.HoverParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Position = docURI, at(p.Position)
		return s.Hover(ctx, p)
	case protocol.MethodTextDocumentDefinition:
		p, err := decodeParams[protocol.DefinitionParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Position = docURI, at(p.Position)
		return s.Definition(ctx, p)
	case protocol.MethodTextDocumentReferences:
		p, err := decodeParams[protocol.ReferenceParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Position = docURI, at(p.Position)
		return s.References(ctx, p)
	case protocol.MethodTextDocumentCompletion:
		p, err := decodeParams[protocol.CompletionParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Position = docURI, at(p.Position)
		return s.Completion(ctx, p)
	case protocol.MethodTextDocumentSignatureHelp:
		p, err := decodeParams[protocol.SignatureHelpParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Position = docURI, at(p.Position)
		return s.SignatureHelp(ctx, p)
	case protocol.MethodTextDocumentDocumentSymbol:
		p, err := decodeParams[protocol.DocumentSymbolParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI = docURI
		result, err := s.DocumentSymbol(ctx, p)
		if syms, ok := result.(protocol.DocumentSymbolSlice); ok {
			return cellSymbols(syms, inCell), err
		}
		return result, err
	case protocol.MethodTextDocumentCodeLens:
		p, err := decodeParams[protocol.CodeLensParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI = docURI
		lenses, err := s.CodeLens(ctx, p)
		var own []protocol.CodeLens
		for _, lens := range lenses {
			if inCell(lens.Range) {
				own = append(own, lens)
			}
		}
		return own, err
	case protocol.MethodTextDocumentInlayHint:
		p, err := decodeParams[protocol.InlayHintParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Range = docURI, span(p.Range)
		return s.InlayHint(ctx, p)
	case protocol.MethodTextDocumentCodeAction:
		p, err := decodeParams[protocol.CodeActionParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Range = docURI, span(p.Range)
		for i := range p.Context.Diagnostics {
			p.Context.Diagnostics[i].Range = span(p.Context.Diagnostics[i].Range)
		}
		return s.CodeAction(ctx, p)
	case protocol.MethodTextDocumentFormatting:
		p, err := decodeParams[protocol.DocumentFormattingParams](req)
		if err != nil {
			return nil, err
		}
		return s.formattingEdits(docURI, p.Options, lineWindow{cell.startLine, cell.startLine + cell.lines})
	case protocol.MethodTextDocumentRangeFormatting:
		p, err := decodeParams[protocol.DocumentRangeFormattingParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Range = docURI, span(p.Range)
		return s.RangeFormatting(ctx, p)
	case protocol.MethodTextDocumentPrepareRename:
		p, err := decodeParams[protocol.PrepareRenameParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Position = docURI, at(p.Position)
		return s.PrepareRename(ctx, p)
	case protocol.MethodTextDocumentRename:
		p, err := decodeParams[protocol.RenameParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Position = docURI, at(p.Position)
		return s.Rename(ctx, p)
	case protocol.MethodTextDocumentSemanticTokensFull:
		p, err := decodeParams[protocol.SemanticTokensParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI = docURI
		toks, err := s.SemanticTokensFull(ctx, p)
		return cellTokens(toks, cell), err
	case protocol.MethodTextDocumentSemanticTokensRange:
		p, err := decodeParams[protocol.SemanticTokensRangeParams](req)
		if err != nil {
			return nil, err
		}
		p.TextDocument.URI, p.Range = docURI, span(p.Range)
		toks, err := s.SemanticTokensRange(ctx, p)
		return cellTokens(toks, cell), err
	}
	return nil, nil
}

// cellSymbols is the part of a symbol tree declared in one cell: a symbol
// named in the cell with the children named there, and the children named
// there of a symbol named in another cell, lifted to its place.
func cellSymbols(syms []protocol.DocumentSymbol, inCell func(protocol.Range) bool) protocol.DocumentSymbolSlice {
	own := protocol.DocumentSymbolSlice{}
	for _, sym := range syms {
		children := cellSymbols(sym.Children, inCell)
		if inCell(sym.SelectionRange) {
			sym.Children = children
			own = append(own, sym)
		} else {
			own = append(own, children...)
		}
	}
	return own
}

// cellTokens keeps the tokens of the document on the cell's lines, encoded
// relative to the cell's first line.
func cellTokens(toks *protocol.SemanticTokens, cell *notebookCell) *protocol.SemanticTokens {
	if toks == nil {
		return nil
	}
	out := &protocol.SemanticTokens{ResultID: toks.ResultID, Data: []uint32{}}
	line, char := uint32(0), uint32(0)
	prevLine, prevChar := uint32(0), uint32(0)
	for i := 0; i+5 <= len(toks.Data); i += 5 {
		if toks.Data[i] != 0 {
			char = 0
		}
		line, char = line+toks.Data[i], char+toks.Data[i+1]
		local := int(line) - cell.startLine
		if local < 0 || local >= cell.lines {
			continue
		}
		deltaLine, deltaChar := uint32Clamp(local)-prevLine, char
		if deltaLine == 0 {
			deltaChar = char - prevChar
		}
		out.Data = append(out.Data, deltaLine, deltaChar, toks.Data[i+2], toks.Data[i+3], toks.Data[i+4])
		prevLine, prevChar = uint32Clamp(local), char
	}
	return out
}

// cellResult moves every position of a result that stands in an open
// notebook's document to the cell it stands in, working on the result's JSON
// so one rule covers every answer shape; a bare range is in implicit, the
// notebook whose cell was asked.
func (s *Server) cellResult(method string, result any, implicit *notebookDocument) (any, error) {
	raw, err := protocol.Marshal(result)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrInternal, err)
	}
	if method == protocol.MethodCodeLensResolve {
		s.cellLensCommand(value)
	}
	return s.mapValue(value, implicit), nil
}

// notebookOf is the open notebook whose document a URI names, nil otherwise.
func (s *Server) notebookOf(value any) *notebookDocument {
	str, ok := value.(string)
	if !ok || !strings.HasPrefix(str, "file:") {
		return nil
	}
	return s.notebooks.named(uriToName(uri.URI(str)))
}

// positional names the keys whose values are positions or ranges of the
// document a result stands in when nothing beside them says otherwise.
var positional = []string{"range", "selectionRange", "originSelectionRange", "insert", "replace", "position"}

// mapValue rewrites value in place and returns it: a location names the cell
// its range is in; a bare range or position belongs to implicit, the document
// whose cell was asked, and is moved to that cell's coordinates.
func (s *Server) mapValue(value any, implicit *notebookDocument) any {
	switch v := value.(type) {
	case []any:
		for i := range v {
			v[i] = s.mapValue(v[i], implicit)
		}
	case map[string]any:
		if isRange(v) {
			if implicit != nil {
				if _, rng, ok := implicit.toCellRange(decodeRange(v)); ok {
					return encodeValue(rng)
				}
			}
			return v
		}
		// done holds the keys mapped with the document their uri names, which
		// the loop below must not read again as the request document's.
		done := map[string]bool{"uri": true, "targetUri": true, "changes": true, "documentChanges": true, "data": true}
		if nb := s.notebookOf(v["uri"]); nb != nil {
			done["range"] = true
			if rng, ok := v["range"].(map[string]any); ok && isRange(rng) {
				if cell, mapped, ok := nb.toCellRange(decodeRange(rng)); ok {
					v["uri"], v["range"] = string(cell), encodeValue(mapped)
				}
			}
		}
		if nb := s.notebookOf(v["targetUri"]); nb != nil {
			done["targetRange"], done["targetSelectionRange"] = true, true
			s.mapTarget(v, nb)
		}
		if changes, ok := v["changes"].(map[string]any); ok {
			v["changes"] = s.mapChanges(changes)
		}
		if docs, ok := v["documentChanges"].([]any); ok {
			v["documentChanges"] = s.mapDocumentChanges(docs)
		}
		for key, field := range v {
			switch {
			case done[key]:
			case implicit != nil && isPositional(key, field):
				v[key] = s.mapPositional(field, implicit)
			default:
				v[key] = s.mapValue(field, implicit)
			}
		}
	}
	return value
}

// isPositional reports whether key holds a range or position of the document.
func isPositional(key string, field any) bool {
	m, ok := field.(map[string]any)
	if !ok {
		return false
	}
	for _, name := range positional {
		if name == key {
			return isRange(m) || isPosition(m)
		}
	}
	return false
}

// mapPositional moves a range or position of nb's document to its cell.
func (s *Server) mapPositional(field any, nb *notebookDocument) any {
	m := field.(map[string]any)
	if isRange(m) {
		if _, rng, ok := nb.toCellRange(decodeRange(m)); ok {
			return encodeValue(rng)
		}
		return m
	}
	if _, pos, ok := nb.toCell(decodePosition(m)); ok {
		return encodeValue(pos)
	}
	return m
}

// mapTarget moves a LocationLink's target into the cell its range is in.
func (s *Server) mapTarget(link map[string]any, nb *notebookDocument) {
	target, ok := link["targetRange"].(map[string]any)
	if !ok || !isRange(target) {
		return
	}
	cell, rng, ok := nb.toCellRange(decodeRange(target))
	if !ok {
		return
	}
	link["targetUri"], link["targetRange"] = string(cell), encodeValue(rng)
	if sel, ok := link["targetSelectionRange"].(map[string]any); ok && isRange(sel) {
		if _, rng, ok := nb.toCellRange(decodeRange(sel)); ok {
			link["targetSelectionRange"] = encodeValue(rng)
		}
	}
}

// mapChanges regroups a WorkspaceEdit's changes to a notebook's document by
// the cells the edits stand in.
func (s *Server) mapChanges(changes map[string]any) map[string]any {
	out := map[string]any{}
	for docURI, edits := range changes {
		nb := s.notebookOf(docURI)
		list, ok := edits.([]any)
		if nb == nil || !ok {
			out[docURI] = s.mapValue(edits, nil)
			continue
		}
		for _, edit := range list {
			cell, mapped, ok := s.cellEdit(nb, edit)
			if !ok {
				continue
			}
			existing, _ := out[string(cell)].([]any)
			out[string(cell)] = append(existing, mapped)
		}
	}
	return out
}

// mapDocumentChanges splits a TextDocumentEdit of a notebook's document into
// one per cell edited, each at the cell's version.
func (s *Server) mapDocumentChanges(docs []any) []any {
	var out []any
	for _, doc := range docs {
		change, ok := doc.(map[string]any)
		textDocument, _ := change["textDocument"].(map[string]any)
		edits, _ := change["edits"].([]any)
		nb := s.notebookOf(textDocument["uri"])
		if !ok || nb == nil || edits == nil {
			out = append(out, s.mapValue(doc, nil))
			continue
		}
		var cells []uri.URI
		perCell := map[uri.URI][]any{}
		for _, edit := range edits {
			cell, mapped, ok := s.cellEdit(nb, edit)
			if !ok {
				continue
			}
			if _, seen := perCell[cell]; !seen {
				cells = append(cells, cell)
			}
			perCell[cell] = append(perCell[cell], mapped)
		}
		for _, cell := range cells {
			out = append(out, map[string]any{
				"textDocument": map[string]any{"uri": string(cell), "version": nb.cell(cell).version},
				"edits":        perCell[cell],
			})
		}
	}
	return out
}

// cellEdit moves a TextEdit on nb's document to the cell its range is in.
func (s *Server) cellEdit(nb *notebookDocument, edit any) (uri.URI, any, bool) {
	m, ok := edit.(map[string]any)
	if !ok {
		return "", nil, false
	}
	rng, ok := m["range"].(map[string]any)
	if !ok || !isRange(rng) {
		return "", nil, false
	}
	cell, mapped, ok := nb.toCellRange(decodeRange(rng))
	if !ok {
		return "", nil, false
	}
	m["range"] = encodeValue(mapped)
	return cell, m, true
}

// cellLensCommand points a resolved lens's references command at the cell the
// lens is in: the command names the document the lens's data recorded, and the
// position it carries is already the cell's.
func (s *Server) cellLensCommand(value any) {
	lens, _ := value.(map[string]any)
	command, _ := lens["command"].(map[string]any)
	args, _ := command["arguments"].([]any)
	if command["command"] != showReferencesCommand || len(args) != 3 {
		return
	}
	nb := s.notebookOf(args[0])
	data, _ := lens["data"].(map[string]any)
	offset, _ := data["name"].(float64)
	if nb == nil {
		return
	}
	if cell, _, ok := nb.toCell(positionsFor(nb.source).position(int(offset))); ok {
		args[0] = string(cell)
	}
}

func isPosition(m map[string]any) bool {
	_, line := m["line"].(float64)
	_, char := m["character"].(float64)
	return line && char && len(m) == 2
}

func isRange(m map[string]any) bool {
	start, _ := m["start"].(map[string]any)
	end, _ := m["end"].(map[string]any)
	return len(m) == 2 && isPosition(start) && isPosition(end)
}

func decodePosition(m map[string]any) protocol.Position {
	line, _ := m["line"].(float64)
	char, _ := m["character"].(float64)
	return protocol.Position{Line: uint32Clamp(int(line)), Character: uint32Clamp(int(char))}
}

func decodeRange(m map[string]any) protocol.Range {
	start, _ := m["start"].(map[string]any)
	end, _ := m["end"].(map[string]any)
	return protocol.Range{Start: decodePosition(start), End: decodePosition(end)}
}

// encodeValue is v as the generic JSON value mapValue works on.
func encodeValue(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}
