package opensysml

import (
	"context"
	"fmt"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// ElementProperties is what ElementProperties read of one element: the
// element itself and one entry per property, in the order asked for, or in
// the element form's order when none was named.
type ElementProperties struct {
	Element    ElementRef
	Properties []ElementProperty
}

// Property is the entry for the named property, nil when the read did not
// ask for it.
func (e *ElementProperties) Property(name string) *ElementProperty {
	if e == nil {
		return nil
	}
	for i := range e.Properties {
		if e.Properties[i].Name == name {
			return &e.Properties[i]
		}
	}
	return nil
}

// ElementRef identifies an element the model's element form knows: an element
// of the model, or a standard-library element it references.
type ElementRef struct {
	// ID is the "@id" of the API element form, which ElementProperties accepts
	// back as the element to read.
	ID string
	// QualifiedName is empty for an unnamed element; an unnamed standard-library
	// member carries its library path (owner/Metaclass[i]).
	QualifiedName string
	// Metaclass is "ConnectionUsage", "PartDefinition", ...; empty when the id
	// is known to nothing.
	Metaclass string
}

// ElementProperty is one metamodel property of an element.
type ElementProperty struct {
	// Name is the specification name ("connectorEnd").
	Name string
	// DefiningClass is the metaclass declaring the property ("Connector");
	// empty when the element's metaclass declares no such property.
	DefiningClass string
	// Many is whether the property holds a collection.
	Many bool
	// Derived is whether the specification derives the property.
	Derived bool
	// Values are the values in the collection's order; none for an empty or
	// null property.
	Values []PropertyValue
	// Unsupported says why there are no values when the property cannot be
	// answered: the metaclass declares no such property, or OpenSysML does not
	// derive it yet. Empty for a property that is answered.
	Unsupported string
}

// Elements are the element references among the values, in order.
func (p *ElementProperty) Elements() []ElementRef {
	if p == nil {
		return nil
	}
	var out []ElementRef
	for _, value := range p.Values {
		if value.Element != nil {
			out = append(out, *value.Element)
		}
	}
	return out
}

// PropertyValue is one value of a metamodel property: exactly one of its
// fields is set.
type PropertyValue struct {
	Element *ElementRef
	String  *string
	Bool    *bool
	Int     *int64
	Real    *float64
	// Number is a number beyond int64 and float64, in decimal.
	Number *string
}

// Text spells the value for display: an element's qualified name or id, a
// literal's text.
func (v PropertyValue) Text() string {
	switch {
	case v.Element != nil:
		if v.Element.QualifiedName != "" {
			return v.Element.QualifiedName
		}
		return v.Element.ID
	case v.String != nil:
		return *v.String
	case v.Bool != nil:
		return fmt.Sprint(*v.Bool)
	case v.Int != nil:
		return fmt.Sprint(*v.Int)
	case v.Real != nil:
		return fmt.Sprint(*v.Real)
	case v.Number != nil:
		return *v.Number
	}
	return ""
}

func (c *client) ElementProperties(ctx context.Context, model *Model, element string, properties ...string) (*ElementProperties, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.getElementProperties(ctx, &pb.GetElementPropertiesRequest{
		ModelHash:  hash,
		ElementId:  element,
		Properties: append([]string(nil), properties...),
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "ElementProperties", Message: resp.Error}
	}
	out := &ElementProperties{Element: elementRefFromProto(resp.Element)}
	for _, property := range resp.Properties {
		converted := ElementProperty{
			Name:          property.Name,
			DefiningClass: property.DefiningClass,
			Many:          property.Many,
			Derived:       property.Derived,
			Unsupported:   property.Unsupported,
		}
		for _, value := range property.Values {
			converted.Values = append(converted.Values, propertyValueFromProto(value))
		}
		out.Properties = append(out.Properties, converted)
	}
	return out, nil
}

func elementRefFromProto(ref *pb.ElementRef) ElementRef {
	if ref == nil {
		return ElementRef{}
	}
	return ElementRef{ID: ref.Id, QualifiedName: ref.QualifiedName, Metaclass: ref.Metaclass}
}

func propertyValueFromProto(value *pb.ElementPropertyValue) PropertyValue {
	switch v := value.GetValue().(type) {
	case *pb.ElementPropertyValue_Element:
		ref := elementRefFromProto(v.Element)
		return PropertyValue{Element: &ref}
	case *pb.ElementPropertyValue_StringValue:
		return PropertyValue{String: &v.StringValue}
	case *pb.ElementPropertyValue_BoolValue:
		return PropertyValue{Bool: &v.BoolValue}
	case *pb.ElementPropertyValue_IntValue:
		return PropertyValue{Int: &v.IntValue}
	case *pb.ElementPropertyValue_RealValue:
		return PropertyValue{Real: &v.RealValue}
	case *pb.ElementPropertyValue_Number:
		return PropertyValue{Number: &v.Number}
	}
	return PropertyValue{}
}
