package grpc

import (
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const (
	opLessEqual    = pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS_EQUAL
	opGreaterEqual = pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER_EQUAL
	opIn           = pb.PrimitiveOperator_PRIMITIVE_OPERATOR_IN
	opInstanceOf   = pb.PrimitiveOperator_PRIMITIVE_OPERATOR_INSTANCE_OF
)

// boundedModel declares the multiplicities the inclusive comparisons are
// probed at: a lower bound of 2, an unbounded upper bound, and a bound of 4.
const boundedModel = `
package Bounds {
	private import ScalarValues::*;
	part def Box {
		attribute x : Real [2..*];
		attribute y : Real [4];
	}
}
`

// TestQueryLessEqualIncludesTheBound verifies <= matches an element whose
// bound equals the operand, which < does not, over the standard's reproducer
// `multiplicityLower <= 2` against `attribute x : Real [2..*]`.
func TestQueryLessEqualIncludesTheBound(t *testing.T) {
	_, _, lessEqual := runQueryOnSource(t, boundedModel, &pb.Query{
		Where: primitive(QueryPropMultiplicityLower, opLessEqual, false, "2"),
	})
	if got := queryIDs(lessEqual); !slices.Equal(got, []string{"Bounds::Box::x"}) {
		t.Errorf("multiplicityLower <= 2 = %v, want [Bounds::Box::x]", got)
	}
	_, _, less := runQueryOnSource(t, boundedModel, &pb.Query{
		Where: primitive(QueryPropMultiplicityLower, pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS, false, "2"),
	})
	if got := queryIDs(less); len(got) != 0 {
		t.Errorf("multiplicityLower < 2 = %v, want no elements", got)
	}
}

// TestQueryGreaterEqualIncludesTheBound verifies >= matches an element whose
// bound equals the operand and the unbounded "*", which > 4 does not for the
// bound itself.
func TestQueryGreaterEqualIncludesTheBound(t *testing.T) {
	_, _, greaterEqual := runQueryOnSource(t, boundedModel, &pb.Query{
		Where: primitive(QueryPropMultiplicityUpper, opGreaterEqual, false, "4"),
	})
	if got := queryIDs(greaterEqual); !slices.Equal(got, []string{"Bounds::Box::x", "Bounds::Box::y"}) {
		t.Errorf("multiplicityUpper >= 4 = %v, want [Bounds::Box::x Bounds::Box::y]", got)
	}
	_, _, greater := runQueryOnSource(t, boundedModel, &pb.Query{
		Where: primitive(QueryPropMultiplicityUpper, pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER, false, "4"),
	})
	if got := queryIDs(greater); !slices.Equal(got, []string{"Bounds::Box::x"}) {
		t.Errorf("multiplicityUpper > 4 = %v, want [Bounds::Box::x]", got)
	}
}

// TestQueryInclusiveComparisonFaultsMatchTheStrictOnes verifies <= and >= are
// refused for the same reasons < and > are: an unordered property, a
// non-number, and a value list.
func TestQueryInclusiveComparisonFaultsMatchTheStrictOnes(t *testing.T) {
	cases := map[string]struct {
		constraint *pb.Constraint
		kind       QueryErrorKind
	}{
		"<= on unordered property": {primitive(QueryPropName, opLessEqual, false, "a"), QueryErrUnorderedProperty},
		">= on unordered property": {primitive(QueryPropType, opGreaterEqual, false, "PartUsage"), QueryErrUnorderedProperty},
		"<= against non-number":    {primitive(QueryPropMultiplicityLower, opLessEqual, false, "many"), QueryErrUnparsableValue},
		">= against non-number":    {primitive(QueryPropMultiplicityUpper, opGreaterEqual, false, ""), QueryErrUnparsableValue},
		"<= against value list":    {primitive(QueryPropMultiplicityLower, opLessEqual, false, "1", "2"), QueryErrMalformedConstraint},
		">= against value list":    {primitive(QueryPropMultiplicityUpper, opGreaterEqual, false, "1", "2"), QueryErrMalformedConstraint},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runQuery(t, &pb.Query{Where: tc.constraint})
			assertQueryError(t, err, tc.kind)
		})
	}
}

// TestQueryInMatchesMembershipInTheList verifies `in` selects the elements
// whose property value is one of the listed values, over the standard's
// reproducer `@type in [PartUsage, AttributeUsage]`, and that inverse negates it.
func TestQueryInMatchesMembershipInTheList(t *testing.T) {
	in := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropType, opIn, false, "PartUsage", "AttributeUsage"),
	})
	want := []string{"Demo::Vehicle::mass", "Demo::vehicle", "Demo::vehicle::wheels", "Demo::vehicle::vin", "Demo::spare"}
	if !slices.Equal(in, want) {
		t.Errorf("@type in [PartUsage, AttributeUsage] = %v, want %v", in, want)
	}
	notIn := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropType, opIn, true, "PartUsage", "AttributeUsage"),
	})
	if want := []string{"Demo", "Demo::Vehicle", "Demo::Wheel", "Demo::vehicle::wheels::@0"}; !slices.Equal(notIn, want) {
		t.Errorf("not @type in [PartUsage, AttributeUsage] = %v, want %v", notIn, want)
	}
	none := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropName, opIn, false, "nobody", "noone"),
	})
	if len(none) != 0 {
		t.Errorf("name in [nobody, noone] = %v, want no elements", none)
	}
}

// TestQueryInWithOneValueIsEquality verifies a one-member list under `in` is
// the degenerate membership, the same verdict `=` gives.
func TestQueryInWithOneValueIsEquality(t *testing.T) {
	in := mustRunQuery(t, &pb.Query{Where: primitive(QueryPropName, opIn, false, "spare")})
	equal := mustRunQuery(t, &pb.Query{Where: primitive(QueryPropName, opEqual, false, "spare")})
	if !slices.Equal(in, equal) || !slices.Equal(in, []string{"Demo::spare"}) {
		t.Errorf("name in [spare] = %v, name = spare gives %v, want [Demo::spare] for both", in, equal)
	}
}

// TestQueryInNeedsAList verifies `in` with nothing to be a member of is
// malformed, as `=` with no value is.
func TestQueryInNeedsAList(t *testing.T) {
	_, err := runQuery(t, &pb.Query{Where: primitive(QueryPropType, opIn, false)})
	assertQueryError(t, err, QueryErrMalformedConstraint)
}

// TestQueryInstanceOfSelectsByMetaclassConformance verifies instanceOf answers
// metamodel conformance rather than name equality: a PartUsage is an instance
// of Usage, Feature and Element, while `=` on the same names matches nothing.
func TestQueryInstanceOfSelectsByMetaclassConformance(t *testing.T) {
	usages := []string{"Demo::Vehicle::mass", "Demo::vehicle", "Demo::vehicle::wheels", "Demo::vehicle::vin", "Demo::spare"}
	namespaces := []string{"Demo", "Demo::Vehicle", "Demo::Vehicle::mass", "Demo::Wheel", "Demo::vehicle", "Demo::vehicle::wheels", "Demo::vehicle::vin", "Demo::spare"}
	// The doc comment on wheels is a Documentation: an Element, not a Namespace.
	everything := slices.Insert(slices.Clone(namespaces), 6, "Demo::vehicle::wheels::@0")
	cases := map[string]struct {
		metaclass string
		want      []string
	}{
		"Usage":          {"Usage", usages},
		"Feature":        {"Feature", usages},
		"Element":        {"Element", everything},
		"Namespace":      {"Namespace", namespaces},
		"Documentation":  {"Documentation", []string{"Demo::vehicle::wheels::@0"}},
		"PartUsage":      {"PartUsage", []string{"Demo::vehicle", "Demo::vehicle::wheels", "Demo::spare"}},
		"Definition":     {"Definition", []string{"Demo::Vehicle", "Demo::Wheel"}},
		"Classifier":     {"Classifier", []string{"Demo::Vehicle", "Demo::Wheel"}},
		"Package":        {"Package", []string{"Demo"}},
		"AttributeUsage": {"AttributeUsage", []string{"Demo::Vehicle::mass", "Demo::vehicle::vin"}},
		"Step":           {"Step", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := mustRunQuery(t, &pb.Query{Where: primitive(QueryPropType, opInstanceOf, false, tc.metaclass)})
			if !slices.Equal(got, tc.want) {
				t.Errorf("@type instanceOf %s = %v, want %v", tc.metaclass, got, tc.want)
			}
		})
	}
	if equal := mustRunQuery(t, &pb.Query{Where: primitive(QueryPropType, opEqual, false, "Usage")}); len(equal) != 0 {
		t.Errorf("@type = Usage = %v, want no elements: no element's metaclass is the abstract Usage", equal)
	}
}

// TestQueryInstanceOfAnyOfAListAndInverse verifies a list under instanceOf
// matches an instance of any listed metaclass, and inverse negates the verdict.
func TestQueryInstanceOfAnyOfAListAndInverse(t *testing.T) {
	listed := mustRunQuery(t, &pb.Query{Where: primitive(QueryPropType, opInstanceOf, false, "Definition", "AttributeUsage")})
	if want := []string{"Demo::Vehicle", "Demo::Vehicle::mass", "Demo::Wheel", "Demo::vehicle::vin"}; !slices.Equal(listed, want) {
		t.Errorf("@type instanceOf [Definition, AttributeUsage] = %v, want %v", listed, want)
	}
	notUsage := mustRunQuery(t, &pb.Query{Where: primitive(QueryPropType, opInstanceOf, true, "Usage")})
	if want := []string{"Demo", "Demo::Vehicle", "Demo::Wheel", "Demo::vehicle::wheels::@0"}; !slices.Equal(notUsage, want) {
		t.Errorf("not @type instanceOf Usage = %v, want %v", notUsage, want)
	}
}

// TestQueryInstanceOfCombinesWithOtherConstraints verifies instanceOf nests
// under a composite like any primitive, so "usages named like this" is one query.
func TestQueryInstanceOfCombinesWithOtherConstraints(t *testing.T) {
	got := mustRunQuery(t, &pb.Query{Where: composite(pb.CompositeOperator_COMPOSITE_OPERATOR_AND,
		primitive(QueryPropType, opInstanceOf, false, "Usage"),
		primitive(QueryPropMultiplicityLower, opGreaterEqual, false, "4"),
	)})
	if want := []string{"Demo::vehicle::wheels"}; !slices.Equal(got, want) {
		t.Errorf("instanceOf Usage and multiplicityLower >= 4 = %v, want %v", got, want)
	}
}

// TestQueryInstanceOfFaults verifies instanceOf is refused on any property but
// "@type", with an empty operand, and with a metaclass the library does not
// declare — each a fault in the query rather than an empty answer.
func TestQueryInstanceOfFaults(t *testing.T) {
	cases := map[string]struct {
		constraint *pb.Constraint
		kind       QueryErrorKind
	}{
		"on name":               {primitive(QueryPropName, opInstanceOf, false, "Usage"), QueryErrUnorderedProperty},
		"on type":               {primitive(QueryPropElementType, opInstanceOf, false, "Usage"), QueryErrUnorderedProperty},
		"no value":              {primitive(QueryPropType, opInstanceOf, false), QueryErrMalformedConstraint},
		"empty name":            {primitive(QueryPropType, opInstanceOf, false, ""), QueryErrUnparsableValue},
		"undeclared metaclass":  {primitive(QueryPropType, opInstanceOf, false, "Gizmo"), QueryErrUnparsableValue},
		"undeclared in a list":  {primitive(QueryPropType, opInstanceOf, false, "Usage", "Gizmo"), QueryErrUnparsableValue},
		"undeclared under or":   {composite(pb.CompositeOperator_COMPOSITE_OPERATOR_OR, primitive(QueryPropName, opEqual, false, "vehicle"), primitive(QueryPropType, opInstanceOf, false, "Gizmo")), QueryErrUnparsableValue},
		"model element as name": {primitive(QueryPropType, opInstanceOf, false, "Vehicle"), QueryErrUnparsableValue},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runQuery(t, &pb.Query{Where: tc.constraint})
			assertQueryError(t, err, tc.kind)
		})
	}
}

// TestQueryOperatorTextNamesEveryOperator verifies refusals name the operator
// as the standard writes it, for all seven.
func TestQueryOperatorTextNamesEveryOperator(t *testing.T) {
	want := map[pb.PrimitiveOperator]string{
		opEqual: "=", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER: ">", pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS: "<",
		opLessEqual: "<=", opGreaterEqual: ">=", opIn: "in", opInstanceOf: "instanceOf",
		pb.PrimitiveOperator_PRIMITIVE_OPERATOR_UNSPECIFIED: "(no operator)",
	}
	for op, text := range want {
		if got := operatorText(op); got != text {
			t.Errorf("operatorText(%v) = %q, want %q", op, got, text)
		}
	}
	for value := range pb.PrimitiveOperator_name {
		if _, known := want[pb.PrimitiveOperator(value)]; !known {
			t.Errorf("operator %v has no text", pb.PrimitiveOperator(value))
		}
	}
}

func queryIDs(resp *pb.QueryResponse) []string {
	ids := make([]string, 0, len(resp.Elements))
	for _, element := range resp.Elements {
		ids = append(ids, element.Id)
	}
	return ids
}
