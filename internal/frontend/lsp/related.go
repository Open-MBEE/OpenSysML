package lsp

import (
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// MethodRelatedElements answers what relates to an element and how, grouped by
// relationship kind, so a client can offer them for a rendering's `include`.
const MethodRelatedElements = "opensysml/relatedElements"

// RelatedElementsCapability is the capability a server advertises when it
// serves opensysml/relatedElements; its value lists the relationship kinds the
// result groups by, in listing order.
const RelatedElementsCapability = "openSysmlRelatedElements"

// RelatedElementsParams asks what relates to one element.
type RelatedElementsParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	// Element is the qualified name of the element, as a rendering's node carries it in `fqn`.
	Element string `json:"element"`
}

// RelatedElementsResult groups the elements related to one element by
// relationship kind; a kind relating nothing is left out.
type RelatedElementsResult struct {
	Element RelatedElement `json:"element"`
	Groups  []RelatedGroup `json:"groups"`
}

// RelatedGroup is the elements one kind of relationship relates the element to.
type RelatedGroup struct {
	Kind     string           `json:"kind"`
	Label    string           `json:"label"`
	Elements []RelatedElement `json:"elements"`
}

// RelatedElement names an element a rendering can include.
type RelatedElement struct {
	// FQN is the qualified name a render request includes the element by.
	FQN string `json:"fqn"`
	// Name is the element's own name; Kind the keyword the notation declares it with.
	Name string `json:"name"`
	Kind string `json:"kind"`
	// DeclaredHere is true when the requested document declares the element.
	DeclaredHere bool          `json:"declaredHere,omitempty"`
	Origin       *RenderOrigin `json:"origin,omitempty"`
}

// relatedKindNames lists the relationship kinds the result groups by.
func relatedKindNames() []string {
	kinds := view.RelatedKinds()
	out := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		out = append(out, string(kind))
	}
	return out
}

// RelatedElements answers opensysml/relatedElements. The elements are read
// under the workspace lock and described after it is released: describing one
// locates it in its document, which reads the workspace again.
func (s *Server) RelatedElements(params *RelatedElementsParams) (*RelatedElementsResult, error) {
	name := uriToName(params.TextDocument.URI)
	if params.Element == "" {
		return nil, fmt.Errorf("%s: element names no element", MethodRelatedElements)
	}
	var (
		element *symbols.Symbol
		groups  []view.RelatedGroup
		docs    = map[string]*model.Document{}
	)
	err := s.ws.Read(func(r *model.Reading) error {
		sym, related, err := r.RelatedElements(name, params.Element)
		if err != nil {
			return err
		}
		if sym == nil {
			return fmt.Errorf("%s: no element named %s", name, params.Element)
		}
		element, groups = sym, related
		docs[sym.DocName] = r.Document(sym.DocName)
		for _, group := range related {
			for _, elem := range group.Elements {
				docs[elem.DocName] = r.Document(elem.DocName)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := &RelatedElementsResult{Element: s.relatedElement(docs, name, element), Groups: []RelatedGroup{}}
	for _, group := range groups {
		listed := RelatedGroup{Kind: string(group.Kind), Label: group.Label, Elements: []RelatedElement{}}
		for _, elem := range group.Elements {
			if _, named := nodeOwners(elem); !named {
				continue
			}
			listed.Elements = append(listed.Elements, s.relatedElement(docs, name, elem))
		}
		if len(listed.Elements) > 0 {
			out.Groups = append(out.Groups, listed)
		}
	}
	return out, nil
}

// relatedElement describes sym for a client: its qualified name as an edit or
// an include spells it, whether the requested document declares it, and where
// in its document, read from the documents as the elements were.
func (s *Server) relatedElement(docs map[string]*model.Document, requested string, sym *symbols.Symbol) RelatedElement {
	out := RelatedElement{FQN: notationName(sym), Name: sym.Name, Kind: sym.Notation(), DeclaredHere: sym.DocName == requested}
	if doc := docs[sym.DocName]; doc != nil {
		out.Origin = s.originOf(doc, sym.Origin())
	}
	return out
}
