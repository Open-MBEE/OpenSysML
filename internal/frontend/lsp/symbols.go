package lsp

import (
	"context"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// DocumentSymbol returns the hierarchical symbol tree for a document, a bundled
// library one included.
func (s *Server) DocumentSymbol(ctx context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	syms := walkDocumentSymbols(doc.Content, doc.Scope)
	out := make([]interface{}, 0, len(syms))
	for _, ds := range syms {
		out = append(out, ds)
	}
	return out, nil
}

// walkDocumentSymbols converts a scope's members (and their child scopes) into
// a DocumentSymbol tree. A metadata annotation body's declarations live in an
// anonymous child scope no member owns, so those scopes are walked too.
func walkDocumentSymbols(content []byte, scope *symbols.Scope) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, sym := range scope.Members() {
		if sym.Name == "" {
			continue
		}
		ds := protocol.DocumentSymbol{
			Name:           sym.Name,
			Kind:           lspSymbolKind(sym.Kind),
			Range:          spanToRange(content, sym.DeclSpan),
			SelectionRange: spanToRange(content, sym.DeclSpan),
		}
		if sym.Scope != nil {
			ds.Children = walkDocumentSymbols(content, sym.Scope)
		}
		out = append(out, ds)
	}
	for _, child := range scope.Children() {
		if isAnnotationBodyScope(child) {
			out = append(out, walkDocumentSymbols(content, child)...)
		}
	}
	return out
}

// lspSymbolKind maps a core SymbolKind to the LSP SymbolKind.
func lspSymbolKind(k symbols.SymbolKind) protocol.SymbolKind {
	switch k {
	case symbols.SymbolPackage:
		return protocol.SymbolKindPackage
	case symbols.SymbolNamespace:
		return protocol.SymbolKindNamespace
	case symbols.SymbolAlias:
		return protocol.SymbolKindVariable
	case symbols.SymbolDependency, symbols.SymbolRelationship:
		return protocol.SymbolKindModule
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation:
		return protocol.SymbolKindString
	default:
		return protocol.SymbolKindObject
	}
}

// Symbols implements workspace/symbol: a flat, query-filtered list of all
// symbols across every open document.
func (s *Server) Symbols(ctx context.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	query := strings.ToLower(params.Query)

	var out []protocol.SymbolInformation
	for _, name := range s.ws.DocumentNames() {
		doc := s.ws.Document(name)
		if doc == nil || doc.Scope == nil {
			continue
		}
		content := doc.Content
		uriStr := s.documentURI(name)
		collectWorkspaceSymbols(doc.Scope, "", query, content, uriStr, &out)
	}
	return out, nil
}

// collectWorkspaceSymbols recursively appends matching symbols to out.
func collectWorkspaceSymbols(scope *symbols.Scope, container, query string, content []byte, docURI uri.URI, out *[]protocol.SymbolInformation) {
	for _, sym := range scope.Members() {
		if sym.Name == "" {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(sym.Name), query) {
			*out = append(*out, protocol.SymbolInformation{
				Name: sym.Name,
				Kind: lspSymbolKind(sym.Kind),
				Location: protocol.Location{
					URI:   docURI,
					Range: spanToRange(content, sym.DeclSpan),
				},
				ContainerName: container,
			})
		}
		if sym.Scope != nil {
			collectWorkspaceSymbols(sym.Scope, sym.Name, query, content, docURI, out)
		}
	}
	for _, child := range scope.Children() {
		if isAnnotationBodyScope(child) {
			collectWorkspaceSymbols(child, container, query, content, docURI, out)
		}
	}
}
