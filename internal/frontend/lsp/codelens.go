package lsp

import (
	"context"
	"fmt"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// runElementCommand is the client command a Run or Evaluate lens invokes, with
// one runElementArgs argument. The VS Code extension implements it by running
// the matching `sysml` check over the workspace.
const runElementCommand = "opensysml.runElement"

// showReferencesCommand is the editor command a reference-count lens invokes,
// with the document URI, the position and the locations as its arguments.
const showReferencesCommand = "editor.action.showReferences"

// runElementArgs names the element a Run or Evaluate lens runs and what it is:
// action, state, calc, constraint or requirement.
type runElementArgs struct {
	URI     uri.URI `json:"uri"`
	Element string  `json:"element"`
	Kind    string  `json:"kind"`
}

// codeLensData is what a reference-count lens carries to its resolve request:
// the definition it counts the references of, told apart from another
// declaration of the same name in the document by where its name is written.
type codeLensData struct {
	URI     uri.URI `json:"uri"`
	Element string  `json:"element"`
	Name    int     `json:"name"`
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
	docURI := params.TextDocument.URI
	out := []protocol.CodeLens{}
	var encodeErr error
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
				data, err := lspAny(codeLensData{URI: docURI, Element: fqn, Name: sym.NameSpan.Offset})
				if err != nil {
					encodeErr = err
					return
				}
				out = append(out, protocol.CodeLens{Range: rng, Data: data})
			}
			// `sysml` runs an element by name, so a name declared twice runs nothing.
			if kind, ok := executableKind(sym); ok && len(s.ws.LookupQualified(fqn)) == 1 {
				command, err := runCommand(docURI, fqn, sym.Name, kind)
				if err != nil {
					encodeErr = err
					return
				}
				out = append(out, protocol.CodeLens{Range: rng, Command: command})
			}
		}
		for _, child := range scope.Children() {
			visit(child)
		}
	}
	visit(doc.Scope)
	if encodeErr != nil {
		return nil, encodeErr
	}
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
	sym := s.definitionNamed(name, data.Element, data.Name)
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
	arguments, err := lspAnys(data.URI, lens.Range.Start, locations)
	if err != nil {
		return nil, err
	}
	resolved := *lens
	resolved.Command = protocol.Command{
		Title:     title,
		Command:   showReferencesCommand,
		Arguments: arguments,
	}
	return &resolved, nil
}

// lensData reads the data a CodeLens request attached, which arrives as the
// JSON the lens was sent with.
func lensData(lens *protocol.CodeLens) (codeLensData, bool) {
	var data codeLensData
	if lens == nil || len(lens.Data) == 0 {
		return data, false
	}
	if err := protocol.Unmarshal(lens.Data, &data); err != nil || data.Element == "" {
		return data, false
	}
	return data, true
}

// lspAny encodes a value as the opaque JSON a command argument or lens data is.
func lspAny(v any) (protocol.LSPAny, error) {
	raw, err := protocol.Marshal(v)
	if err != nil {
		return nil, err
	}
	return protocol.LSPAny(raw), nil
}

// lspAnys encodes each value as lspAny does.
func lspAnys(values ...any) ([]protocol.LSPAny, error) {
	out := make([]protocol.LSPAny, len(values))
	for i, v := range values {
		raw, err := lspAny(v)
		if err != nil {
			return nil, err
		}
		out[i] = raw
	}
	return out, nil
}

// definitionNamed is the definition the named document declares under fqn with
// its name written at offset nameAt, nil once an edit has moved it.
func (s *Server) definitionNamed(doc, fqn string, nameAt int) *symbols.Symbol {
	for _, sym := range s.ws.LookupQualified(fqn) {
		if sym.DocName == doc && sym.DeclaresDefinition() && sym.NameSpan.Offset == nameAt {
			return sym
		}
	}
	return nil
}

// runCommand is the lens command running or evaluating the named element.
func runCommand(docURI uri.URI, fqn, name, kind string) (protocol.Command, error) {
	verb := "Evaluate"
	if kind == "action" || kind == "state" {
		verb = "Run"
	}
	arguments, err := lspAnys(runElementArgs{URI: docURI, Element: fqn, Kind: kind})
	if err != nil {
		return protocol.Command{}, err
	}
	return protocol.Command{
		Title:     verb + " " + name,
		Command:   runElementCommand,
		Arguments: arguments,
	}, nil
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
