package opensysml

import (
	"context"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Query selects elements of a model, as the SysML v2 API & Services Query
// resource does: the elements considered, the properties reported for each, and
// the filter every one must satisfy.
type Query struct {
	// Scope names the elements considered by qualified name, each together with
	// everything nested inside it. Empty considers the whole model.
	Scope []string
	// Select names the properties to report. Empty reports all of them.
	Select []string
	// Where is the filter every considered element must satisfy, nil for none.
	Where *Condition
	// OrderBy names the properties the selected elements are sorted by, in
	// order of precedence, each ascending; elements equal under every one keep
	// declaration order. Empty keeps declaration order throughout.
	OrderBy []string
}

// Condition is a query filter: one comparison, or several combined. Build one
// with Equals, In, InstanceOf, Greater, GreaterOrEqual, Less, LessOrEqual, All
// or Any, and negate it with Not.
type Condition struct {
	inverse   bool
	property  string
	operator  pb.PrimitiveOperator
	values    []string
	composite pb.CompositeOperator
	operands  []*Condition
}

// Equals matches an element whose property is any of the values given. The
// property names one of the query properties documented in the API reference
// ("@type", "name", "qualifiedName", …); an unknown one fails the call.
func Equals(property string, values ...string) *Condition {
	return &Condition{property: property, operator: pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, values: values}
}

// Greater matches an element whose property is greater than the value.
func Greater(property, value string) *Condition {
	return &Condition{
		property: property,
		operator: pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER,
		values:   []string{value},
	}
}

// Less matches an element whose property is less than the value.
func Less(property, value string) *Condition {
	return &Condition{
		property: property,
		operator: pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS,
		values:   []string{value},
	}
}

// GreaterOrEqual matches an element whose property is at least the value.
func GreaterOrEqual(property, value string) *Condition {
	return &Condition{
		property: property,
		operator: pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER_EQUAL,
		values:   []string{value},
	}
}

// LessOrEqual matches an element whose property is at most the value.
func LessOrEqual(property, value string) *Condition {
	return &Condition{
		property: property,
		operator: pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS_EQUAL,
		values:   []string{value},
	}
}

// In matches an element whose property is a member of the values given, the
// standard's `in` operator. An empty list fails the call.
func In(property string, values ...string) *Condition {
	return &Condition{property: property, operator: pb.PrimitiveOperator_PRIMITIVE_OPERATOR_IN, values: values}
}

// InstanceOf matches an element whose metaclass conforms to any of the named
// metaclasses ("Usage", "Feature", "Element", …), the standard's `instanceOf`
// operator on "@type". A name the library declares no metaclass for fails the call.
func InstanceOf(metaclasses ...string) *Condition {
	return &Condition{property: "@type", operator: pb.PrimitiveOperator_PRIMITIVE_OPERATOR_INSTANCE_OF, values: metaclasses}
}

// All matches an element every condition matches. Fewer than two conditions
// fail the call: the standard combines at least two.
func All(conditions ...*Condition) *Condition {
	return &Condition{composite: pb.CompositeOperator_COMPOSITE_OPERATOR_AND, operands: conditions}
}

// Any matches an element at least one condition matches. Fewer than two
// conditions fail the call.
func Any(conditions ...*Condition) *Condition {
	return &Condition{composite: pb.CompositeOperator_COMPOSITE_OPERATOR_OR, operands: conditions}
}

// Not negates a condition. A comparison negates its verdict; a composite one,
// which the wire has no negation for, negates each operand and swaps All for
// Any, which says the same thing.
func (c *Condition) Not() *Condition {
	if c == nil {
		return nil
	}
	negated := *c
	if negated.composite == pb.CompositeOperator_COMPOSITE_OPERATOR_UNSPECIFIED {
		negated.inverse = !negated.inverse
		return &negated
	}
	if negated.composite == pb.CompositeOperator_COMPOSITE_OPERATOR_AND {
		negated.composite = pb.CompositeOperator_COMPOSITE_OPERATOR_OR
	} else {
		negated.composite = pb.CompositeOperator_COMPOSITE_OPERATOR_AND
	}
	negated.operands = make([]*Condition, 0, len(c.operands))
	for _, operand := range c.operands {
		negated.operands = append(negated.operands, operand.Not())
	}
	return &negated
}

// QueryElement is one element a query selected.
type QueryElement struct {
	// ID is the element's qualified name.
	ID string
	// Type is its metamodel type name ("PartUsage", "PartDefinition", …).
	Type string
	// Properties carries what Select asked for, omitting a property the element
	// does not have.
	Properties map[string]string
}

func (c *client) Query(ctx context.Context, model *Model, query Query) ([]QueryElement, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	return c.selectElements(ctx, &pb.QueryRequest{
		ModelHash: hash,
		Query: &pb.Query{
			Scope:   append([]string(nil), query.Scope...),
			Select:  append([]string(nil), query.Select...),
			Where:   conditionToProto(query.Where),
			OrderBy: append([]string(nil), query.OrderBy...),
		},
	})
}

func (c *client) QueryOSLC(ctx context.Context, model *Model, oslc string) ([]QueryElement, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	return c.selectElements(ctx, &pb.QueryRequest{ModelHash: hash, OslcQuery: oslc})
}

func (c *client) selectElements(ctx context.Context, req *pb.QueryRequest) ([]QueryElement, error) {
	resp, err := c.caller.query(ctx, req)
	if err != nil {
		return nil, err
	}
	elements := make([]QueryElement, 0, len(resp.Elements))
	for _, element := range resp.Elements {
		converted := QueryElement{ID: element.Id, Type: element.Type}
		if len(element.Properties) > 0 {
			converted.Properties = make(map[string]string, len(element.Properties))
			for name, value := range element.Properties {
				converted.Properties[name] = value
			}
		}
		elements = append(elements, converted)
	}
	return elements, nil
}

func conditionToProto(condition *Condition) *pb.Constraint {
	if condition == nil {
		return nil
	}
	if condition.composite != pb.CompositeOperator_COMPOSITE_OPERATOR_UNSPECIFIED {
		composite := &pb.CompositeConstraint{Operator: condition.composite}
		for _, operand := range condition.operands {
			composite.Constraint = append(composite.Constraint, conditionToProto(operand))
		}
		return &pb.Constraint{Constraint: &pb.Constraint_Composite{Composite: composite}}
	}
	return &pb.Constraint{Constraint: &pb.Constraint_Primitive{Primitive: &pb.PrimitiveConstraint{
		Inverse:  condition.inverse,
		Property: condition.property,
		Operator: condition.operator,
		Value:    append([]string(nil), condition.values...),
	}}}
}
