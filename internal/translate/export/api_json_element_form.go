package export

import (
	"encoding/json"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ElementForm is the API element form of a model — the objects WriteAPIJSON
// writes — indexed for reading one element's properties at a time.
type ElementForm struct {
	elements        []APIJSONElement
	byID            map[string]int
	byQualifiedName map[string]int
	libraryNames    map[string]APIJSONLibraryReference
	// omissions records, per element id and property, why the writer left a
	// value out of the element object.
	omissions map[string]map[string]metamodel.Reason
}

// APIJSONElement is one object of the element form: its "@id", its "@type"
// and its properties as the form spells them, in the form's order. The
// writer's own annotations ("sysx:" keys) are left out.
type APIJSONElement struct {
	ID        string
	Metaclass string
	// QualifiedName is the element's qualifiedName property; empty for an
	// unnamed element.
	QualifiedName string
	Properties    []APIJSONProperty
}

// APIJSONProperty is one property of an element object. Value is what the
// JSON would hold: string, bool, int64, float64, nil, an APIJSONReference to
// an element, a json.Number beyond int64 and float64, or a []any of those in
// the collection's order.
type APIJSONProperty struct {
	Name  string
	Value any
}

// ModelElementForm converts the model's documents to its element form once
// and indexes the result. An unresolvable object property fails the
// conversion, as WriteAPIJSON does.
func ModelElementForm(documents []ModelDocument) (*ElementForm, error) {
	return modelElementFormWith(documents, IDQualifiedName)
}

func modelElementFormWith(documents []ModelDocument, idForm IDForm) (*ElementForm, error) {
	graph, err := ModelToRDFWith(documents, idForm)
	if err != nil {
		return nil, err
	}
	return elementFormOfGraph(graph)
}

func elementFormOfGraph(graph *rdf.Graph) (*ElementForm, error) {
	form := &ElementForm{
		byID:            map[string]int{},
		byQualifiedName: map[string]int{},
		libraryNames:    map[string]APIJSONLibraryReference{},
		omissions:       map[string]map[string]metamodel.Reason{},
	}
	options := APIJSONOptions{
		LibraryReferences: func(refs []APIJSONLibraryReference) {
			for _, ref := range refs {
				form.libraryNames[ref.ID] = ref
			}
		},
		Omitted: func(omission APIJSONOmission) {
			properties := form.omissions[omission.Subject]
			if properties == nil {
				properties = map[string]metamodel.Reason{}
				form.omissions[omission.Subject] = properties
			}
			if _, recorded := properties[omission.Property]; !recorded {
				properties[omission.Property] = omission.Reason
			}
		},
	}
	objects, err := apiJSONElementsWithOptions(graph, options)
	if err != nil {
		return nil, err
	}
	form.elements = make([]APIJSONElement, 0, len(objects))
	for _, object := range objects {
		element := APIJSONElement{Properties: make([]APIJSONProperty, 0, len(object))}
		for _, member := range object {
			switch {
			case member.key == "@id":
				element.ID, _ = member.value.(string)
			case member.key == "@type":
				element.Metaclass, _ = member.value.(string)
			case strings.HasPrefix(member.key, "sysx:"):
				continue
			default:
				if member.key == "qualifiedName" {
					element.QualifiedName, _ = member.value.(string)
				}
				value, err := elementFormValue(member.value)
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %w", element.ID, member.key, err)
				}
				element.Properties = append(element.Properties, APIJSONProperty{Name: member.key, Value: value})
			}
		}
		form.byID[element.ID] = len(form.elements)
		if element.QualifiedName != "" {
			if _, dup := form.byQualifiedName[element.QualifiedName]; !dup {
				form.byQualifiedName[element.QualifiedName] = len(form.elements)
			}
		}
		form.elements = append(form.elements, element)
	}
	return form, nil
}

// elementFormValue normalizes a member value to the shapes APIJSONProperty
// documents; a name reference no id resolved is refused, as the encoder refuses it.
func elementFormValue(value any) (any, error) {
	switch v := value.(type) {
	case nil, string, bool, int64, float64, APIJSONReference:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case uint64:
		if v > 1<<63-1 {
			return json.Number(strconv.FormatUint(v, 10)), nil
		}
		return int64(v), nil
	case float32:
		return float64(v), nil
	case json.Number:
		// An integer lexical form beyond int64 keeps its text; only a real form
		// becomes a float64, and one beyond float64 keeps its text too.
		if !strings.ContainsAny(string(v), ".eE") {
			if i, err := v.Int64(); err == nil {
				return i, nil
			}
			return v, nil
		}
		if f, err := v.Float64(); err == nil {
			return f, nil
		}
		return v, nil
	case apiJSONNameReference:
		return nil, fmt.Errorf("unresolved object-property name")
	case []any:
		out := make([]any, 0, len(v))
		for _, member := range v {
			normalized, err := elementFormValue(member)
			if err != nil {
				return nil, err
			}
			out = append(out, normalized)
		}
		return out, nil
	case []APIJSONReference:
		out := make([]any, 0, len(v))
		for _, member := range v {
			out = append(out, member)
		}
		return out, nil
	case []string:
		out := make([]any, 0, len(v))
		for _, member := range v {
			out = append(out, member)
		}
		return out, nil
	}
	return nil, fmt.Errorf("value of type %T has no element-form spelling", value)
}

// Elements are every element of the form in the form's order.
func (f *ElementForm) Elements() []APIJSONElement {
	if f == nil {
		return nil
	}
	return slices.Clone(f.elements)
}

// ByID is the element with the given "@id".
func (f *ElementForm) ByID(id string) (APIJSONElement, bool) {
	if f == nil {
		return APIJSONElement{}, false
	}
	i, ok := f.byID[id]
	if !ok {
		return APIJSONElement{}, false
	}
	return f.elements[i], true
}

// ByQualifiedName is the element whose qualifiedName is the given name; the
// first in the form's order when two identity scopes declare one name.
func (f *ElementForm) ByQualifiedName(name string) (APIJSONElement, bool) {
	if f == nil {
		return APIJSONElement{}, false
	}
	i, ok := f.byQualifiedName[name]
	if !ok {
		return APIJSONElement{}, false
	}
	return f.elements[i], true
}

// Omission is why the element id's object carries no value for property: the
// metamodel evaluator could not derive it for this element, or a reference it
// holds could not be resolved. A property the object carries, or one that is
// legitimately empty, has no omission.
func (f *ElementForm) Omission(id, property string) (metamodel.Reason, bool) {
	if f == nil {
		return metamodel.Reason{}, false
	}
	reason, ok := f.omissions[id][property]
	return reason, ok
}

// NameOf is the qualified name and metaclass of the element an id refers to:
// an element of the form, or a standard-library element the form references.
// An unnamed library member answers its library path in place of a name.
func (f *ElementForm) NameOf(id string) (qualifiedName, metaclass string, ok bool) {
	if f == nil {
		return "", "", false
	}
	if i, found := f.byID[id]; found {
		return f.elements[i].QualifiedName, f.elements[i].Metaclass, true
	}
	if ref, found := f.libraryNames[id]; found {
		name := ref.QualifiedName
		if name == "" {
			name = ref.LibraryPath
		}
		return name, ref.Type, true
	}
	return "", "", false
}
