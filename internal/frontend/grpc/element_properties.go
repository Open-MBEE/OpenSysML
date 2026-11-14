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
// name — is read from the form of the library document declaring it.
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
	form, err := cached.ElementForm()
	if err != nil {
		return &pb.GetElementPropertiesResponse{Error: err.Error()}, nil
	}
	element, ok := s.elementLookup(cached, req)(form)
	if !ok {
		// A standard-library element the model's form references, or names,
		// is read from its own library document's form.
		var known bool
		element, form, known, err = export.LibraryElementForm(req.ElementId)
		if err != nil {
			return &pb.GetElementPropertiesResponse{Error: err.Error()}, nil
		}
		if !known {
			return &pb.GetElementPropertiesResponse{Error: fmt.Sprintf("element not found: %s", req.ElementId)}, nil
		}
	}
	resp := &pb.GetElementPropertiesResponse{Element: elementRefOf(form, element.ID)}
	if len(req.Properties) == 0 {
		for _, property := range element.Properties {
			resp.Properties = append(resp.Properties, elementPropertyOf(form, element, property.Name))
		}
		return resp, nil
	}
	for _, name := range req.Properties {
		resp.Properties = append(resp.Properties, elementPropertyOf(form, element, name))
	}
	return resp, nil
}

// ElementForm is the model's API element form, converted once per model. A
// document the parser could not read whole refuses the conversion, as Convert
// refuses it: the form would silently miss what the parser skipped.
func (m *CachedModel) ElementForm() (*export.ElementForm, error) {
	m.elementFormOnce.Do(func() {
		documents := make([]export.ModelDocument, 0, len(m.Documents))
		for _, doc := range m.Documents {
			if doc == nil || doc.Source == nil {
				continue
			}
			if syntax := convert.SyntaxErrorOf(doc.Source.Name(), doc.Source, doc.ParseDiags); syntax != nil {
				m.elementFormErr = syntax
				return
			}
			documents = append(documents, export.ModelDocument{File: doc.Source, Root: doc.Root})
		}
		m.elementForm, m.elementFormErr = export.ModelElementForm(documents)
	})
	return m.elementForm, m.elementFormErr
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

// elementFormElementByID finds the element an id names: the form's "@id", else the id
// GetSymbol reports the element under.
func (m *CachedModel) elementFormElementByID(form *export.ElementForm, id string) (export.APIJSONElement, bool) {
	if element, ok := form.ByID(id); ok {
		return element, true
	}
	if syms := m.LookupElementID(id); len(syms) > 0 {
		return m.elementFormElementOf(form, syms[0])
	}
	return export.APIJSONElement{}, false
}

// elementFormElementNamed finds the element a name states: resolved from the root by
// resolve, else as the form or a member chain names it.
func (m *CachedModel) elementFormElementNamed(form *export.ElementForm, name string, resolve func(string) []*symbols.Symbol) (export.APIJSONElement, bool) {
	var sym *symbols.Symbol
	if syms := resolve(name); len(syms) > 0 {
		sym = syms[0]
	} else if len(symbolfacts.LookupNamed(m.Index, name)) > 0 {
		// Declared, and not visible from the root namespace.
		return export.APIJSONElement{}, false
	} else if element, ok := form.ByQualifiedName(name); ok {
		return element, true
	} else if syms := m.LookupNamedMember(name); len(syms) > 0 {
		sym = syms[0]
	}
	if sym == nil {
		return export.APIJSONElement{}, false
	}
	return m.elementFormElementOf(form, sym)
}

// elementFormElementOf is the form's element for a symbol of the model.
func (m *CachedModel) elementFormElementOf(form *export.ElementForm, sym *symbols.Symbol) (export.APIJSONElement, bool) {
	identity := m.SymbolContext().OwnIdentity(sym)
	if fqn := m.Index.GetFQN(sym); fqn != "" {
		if element, ok := form.ByQualifiedName(fqn); ok {
			return element, true
		}
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
