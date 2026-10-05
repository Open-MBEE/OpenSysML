package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// runElementCommand is the client command a Run or Evaluate lens invokes, with
// one runElementArgs argument. The VS Code extension implements it by running
// the matching `sysml` check on the document.
const runElementCommand = "opensysml.runElement"

// showReferencesCommand is the editor command a reference-count lens invokes,
// with the document URI, the position and the locations as its arguments.
const showReferencesCommand = "editor.action.showReferences"

// runElementArgs names the element a Run or Evaluate lens runs and what it is:
// action, state, calc, constraint or requirement.
type runElementArgs struct {
	URI     protocol.DocumentURI `json:"uri"`
	Element string               `json:"element"`
	Kind    string               `json:"kind"`
}

// codeLensData is what a reference-count lens carries to its resolve request:
// the definition it counts the references of.
type codeLensData struct {
	URI     protocol.DocumentURI `json:"uri"`
	Element string               `json:"element"`
}

// CodeLens answers the lenses of a document: a reference count on each
// definition, resolved on demand, and a Run or Evaluate command on each
// executable action, state, calc, constraint and requirement declaration that
// is not a step of another behavior.
func (s *Server) CodeLens(ctx context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	name := uriToName(params.TextDocument.URI)
	if s.ws.IsLibraryDocument(name) {
		return nil, nil
	}
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return nil, nil
	}
	pos := positionsOf(doc)
	uri := params.TextDocument.URI
	out := []protocol.CodeLens{}
	seen := map[*symbols.Symbol]bool{}
	var visit func(scope *symbols.Scope)
	visit = func(scope *symbols.Scope) {
		for _, sym := range scope.Members() {
			if seen[sym] || sym.Name == "" || sym.DocName != name {
				continue
			}
			seen[sym] = true
			fqn := s.ws.FQNOf(sym)
			if fqn == "" {
				continue
			}
			rng := pos.rangeOf(sym.NameSpan)
			if sym.DeclaresDefinition() {
				out = append(out, protocol.CodeLens{Range: rng, Data: codeLensData{URI: uri, Element: fqn}})
			}
			if kind, ok := executableKind(sym); ok {
				out = append(out, protocol.CodeLens{Range: rng, Command: runCommand(uri, fqn, kind)})
			}
		}
		for _, child := range scope.Children() {
			visit(child)
		}
	}
	visit(doc.Scope)
	return out, nil
}

// CodeLensResolve fills in a reference-count lens: the references the
// workspace holds to its definition, shown by the editor's references peek.
func (s *Server) CodeLensResolve(ctx context.Context, lens *protocol.CodeLens) (*protocol.CodeLens, error) {
	data, ok := lensData(lens)
	if !ok {
		return lens, nil
	}
	name := uriToName(data.URI)
	sym := s.definitionNamed(name, data.Element)
	if sym == nil {
		return lens, nil
	}
	refs, err := s.ws.ReferencesTo(sym)
	if err != nil {
		return nil, err
	}
	locations := make([]protocol.Location, 0, len(refs))
	posOf := map[string]positions{}
	for _, ref := range refs {
		pos, ok := posOf[ref.Doc]
		if !ok {
			pos = positionsFor(ref.Content)
			posOf[ref.Doc] = pos
		}
		locations = append(locations, protocol.Location{URI: s.documentURI(ref.Doc), Range: pos.rangeOf(ref.Span)})
	}
	title := fmt.Sprintf("%d references", len(locations))
	if len(locations) == 1 {
		title = "1 reference"
	}
	resolved := *lens
	resolved.Command = &protocol.Command{
		Title:     title,
		Command:   showReferencesCommand,
		Arguments: []interface{}{data.URI, lens.Range.Start, locations},
	}
	return &resolved, nil
}

// lensData reads the data a CodeLens request attached, which arrives decoded
// into a generic map.
func lensData(lens *protocol.CodeLens) (codeLensData, bool) {
	var data codeLensData
	if lens == nil || lens.Data == nil {
		return data, false
	}
	raw, err := json.Marshal(lens.Data)
	if err != nil {
		return data, false
	}
	if err := json.Unmarshal(raw, &data); err != nil || data.Element == "" {
		return data, false
	}
	return data, true
}

// definitionNamed is the definition the named document declares under fqn.
func (s *Server) definitionNamed(doc, fqn string) *symbols.Symbol {
	for _, sym := range s.ws.LookupQualified(fqn) {
		if sym.DocName == doc && sym.DeclaresDefinition() {
			return sym
		}
	}
	return nil
}

// runCommand is the lens command running or evaluating the named element.
func runCommand(uri protocol.DocumentURI, fqn, kind string) *protocol.Command {
	title := "Evaluate"
	if kind == "action" || kind == "state" {
		title = "Run"
	}
	return &protocol.Command{
		Title:     title,
		Command:   runElementCommand,
		Arguments: []interface{}{runElementArgs{URI: uri, Element: fqn, Kind: kind}},
	}
}

// executableKind names what a Run or Evaluate lens would run sym as, false for
// a declaration no `sysml` check takes by name: another kind, a step or
// substate of a behavior, or a transition.
func executableKind(sym *symbols.Symbol) (string, bool) {
	kind, ok := behaviorKind(sym)
	if !ok {
		return "", false
	}
	if u, isUsage := sym.Decl.(*ast.Usage); isUsage && (u.IsBodyParameter || u.IsActionNode || u.IsTerminate) {
		return "", false
	}
	if owner := sym.Owner(); owner != nil {
		if _, nested := behaviorKind(owner); nested {
			return "", false
		}
	}
	return kind, true
}

// behaviorKind is the check kind a declaration's syntactic kind runs under.
func behaviorKind(sym *symbols.Symbol) (string, bool) {
	if kind, ok := sym.UsageKind(); ok {
		switch kind {
		case ast.UsageAction:
			return "action", true
		case ast.UsageState:
			return "state", true
		case ast.UsageCalc:
			return "calc", true
		case ast.UsageConstraint:
			return "constraint", true
		case ast.UsageRequirement:
			return "requirement", true
		}
		return "", false
	}
	if kind, ok := sym.DefinitionKind(); ok {
		switch kind {
		case ast.DefAction:
			return "action", true
		case ast.DefState:
			return "state", true
		case ast.DefCalc:
			return "calc", true
		case ast.DefConstraint:
			return "constraint", true
		case ast.DefRequirement:
			return "requirement", true
		}
	}
	return "", false
}
