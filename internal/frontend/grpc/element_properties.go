package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// GetElementProperties reads the metamodel properties of one element of a
// parsed model by their specification names, from the model's API element
// form: the same values Convert writes as api-json, one element at a time. A
// standard-library element — one a value referenced, or a library qualified
// name — is read from the form of the library document declaring it. The model
// is read without its excluded documents, which every response names; only an
// element of an excluded document answers that document's error.
func (s *Service) GetElementProperties(ctx context.Context, req *pb.GetElementPropertiesRequest) (*pb.GetElementPropertiesResponse, error) {
	if err := s.requireCapability(CapabilityElementProperties); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	if strings.TrimSpace(req.ElementId) == "" {
		return nil, statusError(connect.CodeInvalidArgument, "element_id is required")
	}
	for _, name := range req.Properties {
		if strings.TrimSpace(name) == "" {
			return nil, statusError(connect.CodeInvalidArgument, "a property name is empty")
		}
	}
	exclusions := cached.documentExclusions()
	resp := s.elementProperties(cached, req, exclusions)
	resp.ExcludedDocuments = exclusions
	return resp, nil
}

// elementProperties answers req over the documents the model is read with.
func (s *Service) elementProperties(cached *CachedModel, req *pb.GetElementPropertiesRequest, exclusions []*pb.ExcludedDocument) *pb.GetElementPropertiesResponse {
	form, err := cached.ElementForm()
	if err != nil {
		return &pb.GetElementPropertiesResponse{Error: err.Error()}
	}
	element, ok := s.elementLookup(cached, req)(form)
	if !ok {
		if excluded := cached.excludedDocumentOf(form, req.ElementId, exclusions); excluded != nil {
			return &pb.GetElementPropertiesResponse{Error: excluded.Error}
		}
		// A standard-library element the model's form references, or names,
		// is read from its own library document's form.
		var known bool
		element, form, known, err = s.libraryElementForm(req)
		if err != nil {
			return &pb.GetElementPropertiesResponse{Error: err.Error()}
		}
		if !known {
			return &pb.GetElementPropertiesResponse{Error: fmt.Sprintf("element not found: %s", req.ElementId)}
		}
	}
	resp := &pb.GetElementPropertiesResponse{Element: elementRefOf(form, element.ID)}
	if len(req.Properties) == 0 {
		for _, property := range element.Properties {
			resp.Properties = append(resp.Properties, elementPropertyOf(form, element, property.Name))
		}
		return resp
	}
	for _, name := range req.Properties {
		resp.Properties = append(resp.Properties, elementPropertyOf(form, element, name))
	}
	return resp
}

// ElementForm is the API element form of the documents the model is read with, converted once per
// model: an excluded document does not refuse the others' elements, as Convert refuses the model whole.
func (m *CachedModel) ElementForm() (*export.ElementForm, error) {
	m.readElementForm()
	return m.elementForm, m.elementFormErr
}

// documentExclusions is the documents the model is read without, in document order: one the parser
// could not read whole, one the form cannot write, one whose ids an earlier document already declares.
func (m *CachedModel) documentExclusions() []*pb.ExcludedDocument {
	m.readElementForm()
	return m.elementFormExclusions
}

// readElementForm converts the model's readable documents once, keeping the ones left out.
func (m *CachedModel) readElementForm() {
	m.elementFormOnce.Do(func() {
		unreadable := map[string]error{}
		documents := make([]export.ModelDocument, 0, len(m.Documents))
		for _, doc := range m.Documents {
			if doc == nil || doc.Source == nil {
				continue
			}
			if syntax := convert.SyntaxErrorOf(doc.Source.Name(), doc.Source, doc.ParseDiags); syntax != nil {
				unreadable[doc.Source.Name()] = syntax
			}
			documents = append(documents, export.ModelDocument{File: doc.Source, Root: doc.Root})
		}
		var excluded []export.ExcludedDocument
		m.elementForm, excluded, m.elementFormErr = export.ReadableModelElementForm(documents, unreadable)
		for _, exclusion := range excluded {
			m.elementFormExclusions = append(m.elementFormExclusions, &pb.ExcludedDocument{Document: exclusion.Document, Error: exclusion.Err.Error()})
		}
	})
}

// excludedDocumentOf is the excluded document declaring the element id names or
// identifies; nil when none does, or a document the model is read with declares it too.
func (m *CachedModel) excludedDocumentOf(form *export.ElementForm, id string, exclusions []*pb.ExcludedDocument) *pb.ExcludedDocument {
	if len(exclusions) == 0 {
		return nil
	}
	excluded := make(map[string]*pb.ExcludedDocument, len(exclusions))
	for _, exclusion := range exclusions {
		excluded[exclusion.Document] = exclusion
	}
	if document, ok := form.ExcludedDocumentOf(id); ok {
		return excluded[document]
	}
	var found *pb.ExcludedDocument
	for _, syms := range [][]*symbols.Symbol{m.LookupElementID(id), symbolfacts.LookupNamed(m.Index, id), m.LookupNamedMember(id)} {
		for _, sym := range syms {
			exclusion, isExcluded := excluded[sym.DocName]
			if !isExcluded {
				return nil
			}
			if found == nil {
				found = exclusion
			}
		}
	}
	return found
}

// elementLookup is the lookup a request's kind states; without a kind, element_id is read
// as the form's "@id" first, then as a name.
func (s *Service) elementLookup(cached *CachedModel, req *pb.GetElementPropertiesRequest) func(*export.ElementForm) (export.APIJSONElement, bool) {
	if s.capabilities.has(CapabilitySymbolLookupKind) {
		switch req.Lookup {
		case pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME:
			return func(form *export.ElementForm) (export.APIJSONElement, bool) {
				return cached.elementFormElementNamed(form, req.ElementId, cached.ResolveQualifiedMemberName)
			}
		case pb.LookupKind_LOOKUP_KIND_ELEMENT_ID:
			return func(form *export.ElementForm) (export.APIJSONElement, bool) {
				return cached.elementFormElementByID(form, req.ElementId)
			}
		}
	}
	return func(form *export.ElementForm) (export.APIJSONElement, bool) {
		if element, ok := form.ByID(req.ElementId); ok {
			return element, true
		}
		return cached.elementFormElementNamed(form, req.ElementId, cached.ResolveMemberName)
	}
}

// libraryElementForm reads a standard-library element as req's lookup kind states it: by
// qualified name alone, else by the form's id or the name GetSymbol reports it under.
func (s *Service) libraryElementForm(req *pb.GetElementPropertiesRequest) (export.APIJSONElement, *export.ElementForm, bool, error) {
	if s.capabilities.has(CapabilitySymbolLookupKind) && req.Lookup == pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME {
		return export.LibraryElementFormNamed(req.ElementId)
	}
	return export.LibraryElementForm(req.ElementId)
}

// elementFormElementByID finds the element an id names: the form's "@id", else the id
// GetSymbol reports the element under.
func (m *CachedModel) elementFormElementByID(form *export.ElementForm, id string) (export.APIJSONElement, bool) {
	if element, ok := form.ByID(id); ok {
		return element, true
	}
	return m.elementFormElementOfAny(form, m.LookupElementID(id))
}

// elementFormElementNamed finds the element a name states: resolved from the root by
// resolve, else as the form or a member chain names it.
func (m *CachedModel) elementFormElementNamed(form *export.ElementForm, name string, resolve func(string) []*symbols.Symbol) (export.APIJSONElement, bool) {
	syms := resolve(name)
	if len(syms) == 0 {
		if len(symbolfacts.LookupNamed(m.Index, name)) > 0 {
			// Declared, and not visible from the root namespace.
			return export.APIJSONElement{}, false
		}
		if element, ok := form.ByQualifiedName(name); ok {
			return element, true
		}
		syms = m.LookupNamedMember(name)
	}
	return m.elementFormElementOfAny(form, syms)
}

// elementFormElementOfAny is the form's element for the first of syms it holds: a name two
// documents declare is read from the one the model is read with.
func (m *CachedModel) elementFormElementOfAny(form *export.ElementForm, syms []*symbols.Symbol) (export.APIJSONElement, bool) {
	for _, sym := range syms {
		if element, ok := m.elementFormElementOf(form, sym); ok {
			return element, true
		}
	}
	return export.APIJSONElement{}, false
}

// elementFormElementOf is the form's element for a symbol of the model.
func (m *CachedModel) elementFormElementOf(form *export.ElementForm, sym *symbols.Symbol) (export.APIJSONElement, bool) {
	identity := m.SymbolContext().OwnIdentity(sym)
	if fqn := m.Index.GetFQN(sym); fqn != "" {
		if element, ok := form.ByQualifiedName(fqn); ok {
			return element, true
		}
	}
	// The form's own ids first: they are those of the documents it reads, where
	// the whole model's may be refused for a document it leaves out.
	if elementID, ok := form.IDOfDeclaration(sym.DocName, sym.Decl, identity); ok {
		return form.ByID(elementID)
	}
	elementID, err := m.elementIDReader().Read(sym.DocName, sym.Decl, identity)
	if err != nil || elementID == "" {
		return export.APIJSONElement{}, false
	}
	return form.ByID(elementID)
}

// elementRefOf identifies an element by the form's knowledge of its id: an
// element of the model, or a standard-library element the model references.
func elementRefOf(form *export.ElementForm, id string) *pb.ElementRef {
	ref := &pb.ElementRef{Id: id}
	ref.QualifiedName, ref.Metaclass, _ = form.NameOf(id)
	return ref
}

// elementPropertyOf answers one property of an element: its values when the
// form carries it; none when the metaclass declares it and the form has no
// value for it; the reason when the metaclass does not declare it, the
// writer left it out of this element, or the metamodel evaluator does not
// derive it at all.
func elementPropertyOf(form *export.ElementForm, element export.APIJSONElement, name string) *pb.ElementProperty {
	out := &pb.ElementProperty{Name: name}
	definition, declared := ontology.PropertyOf(element.Metaclass, name)
	if declared {
		out.DefiningClass = definition.DefiningClass
		out.Many = definition.Many
		out.Derived = definition.Derived
	}
	for _, property := range element.Properties {
		if property.Name != name {
			continue
		}
		out.Values = elementPropertyValues(form, property.Value)
		return out
	}
	if !declared {
		out.Unsupported = fmt.Sprintf("%s declares no property %s", element.Metaclass, name)
		return out
	}
	if reason, omitted := form.Omission(element.ID, name); omitted {
		out.Unsupported = fmt.Sprintf("%s is not derived for this element: %s", definition.QualifiedName(), reason)
		return out
	}
	if reason, omitted := omittedPropertyReasons()[definition.QualifiedName()]; omitted {
		out.Unsupported = fmt.Sprintf("%s is not derived by OpenSysML: %s", definition.QualifiedName(), reason)
	}
	return out
}

// elementPropertyValues spells a form value as property values: a collection
// member per value, one value for a single-valued property, none for null.
func elementPropertyValues(form *export.ElementForm, value any) []*pb.ElementPropertyValue {
	switch v := value.(type) {
	case nil:
		return nil
	case []any:
		out := make([]*pb.ElementPropertyValue, 0, len(v))
		for _, member := range v {
			out = append(out, elementPropertyValues(form, member)...)
		}
		return out
	case export.APIJSONReference:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_Element{Element: elementRefOf(form, v.ID)}}}
	case string:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_StringValue{StringValue: v}}}
	case bool:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_BoolValue{BoolValue: v}}}
	case int64:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_IntValue{IntValue: v}}}
	case float64:
		return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_RealValue{RealValue: v}}}
	}
	return []*pb.ElementPropertyValue{{Value: &pb.ElementPropertyValue_Number{Number: fmt.Sprint(value)}}}
}

// omittedPropertyReasons keys the metamodel evaluator's omissions by the
// property's qualified name, "Connector::connectorEnd".
var omittedPropertyReasons = sync.OnceValue(func() map[string]string {
	reasons := map[string]string{}
	for _, omission := range metamodel.Omitted() {
		reasons[omission.DefiningClass+"::"+omission.Property] = omission.Reason
	}
	return reasons
})
