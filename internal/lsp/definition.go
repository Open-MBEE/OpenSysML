package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Definition returns the declaration location of the reference under the cursor,
// in a workspace document or a bundled library one.
func (s *Server) Definition(ctx context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	content := doc.Content
	offset := positionToOffset(content, params.Position)

	refs := collectRefs(doc.AST, doc.Scope)
	ref := refAtOffset(refs, offset)
	if ref == nil {
		// A metadata body declaration's name is not a reference, but it
		// implicitly redefines a metadata-definition feature; jump to that.
		if sym := usageNameAt(doc.Scope, offset); sym != nil {
			if target, _, ok := s.ws.MetadataBodyRedefines(sym); ok {
				return []protocol.Location{s.symbolLocation(name, target)}, nil
			}
		}
		// A document query binding's name is not a reference either; it names a
		// parameter of the query typing the enclosing calc usage. Jump to it.
		if sym := symbolAtOffset(doc.Scope, offset); sym != nil {
			if param, ok := s.ws.QueryBindingParameter(sym); ok {
				return []protocol.Location{s.symbolLocation(name, param)}, nil
			}
		}
		return nil, nil
	}
	// A qualifier goes to the namespace it names; the called name, when the
	// arguments leave it tied between overloads, goes to every one of them.
	if !onCalledName(*ref, offset) {
		if sym, _, ok := s.referencedSegment(name, *ref, offset); ok && sym != nil {
			return []protocol.Location{s.symbolLocation(name, sym)}, nil
		}
	} else if overloads := s.ws.AmbiguousInvocationInDoc(name, *ref); len(overloads) > 0 {
		locs := make([]protocol.Location, len(overloads))
		for i, sym := range overloads {
			locs[i] = s.symbolLocation(name, sym)
		}
		return locs, nil
	}
	sym, ok := s.ws.ResolveReferenceInDoc(name, *ref)
	if !ok || sym == nil {
		return nil, nil
	}
	return []protocol.Location{s.symbolLocation(name, sym)}, nil
}

// symbolLocation builds a Location for a resolved symbol using the symbol's own
// declaring document (DocName), so cross-file definitions point at the correct
// file/bytes; a bundled library declaration is located in its sysml-stdlib
// document, at the position its text gives it. Falls back to the requesting
// document name if the symbol was not stamped (should not happen for resolved
// symbols).
func (s *Server) symbolLocation(reqName string, sym *symbols.Symbol) protocol.Location {
	declName := sym.DocName
	if declName == "" {
		declName = reqName // fallback: symbol not stamped (shouldn't happen for resolved symbols)
	}
	var content []byte
	if doc := s.document(declName); doc != nil {
		content = doc.Content
	}
	return protocol.Location{URI: s.documentURI(declName), Range: spanToRange(content, sym.DeclSpan)}
}
