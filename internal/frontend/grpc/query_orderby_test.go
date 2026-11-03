package grpc

import (
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// orderedModel declares the values the sort comparison is probed at: upper
// bounds whose numeric and lexical orders differ ("9" < "10" < "*"), an
// abstract and a concrete definition declared in the reverse of their sort
// order, one short name, and members whose names collide across types.
const orderedModel = `
package Sorted {
	private import ScalarValues::*;
	abstract part def Beta;
	part def Alpha;
	part ten : Alpha [10];
	part <s> nine : Alpha [9];
	part many : Alpha [1..*];
	attribute alpha : Real;
	part alpha : Alpha;
}
`

func orderedQuery(where *pb.Constraint, orderBy ...string) *pb.Query {
	return &pb.Query{Where: where, OrderBy: orderBy, Select: []string{QueryPropName}}
}

// TestQueryOrderByNameSortsAscendingByteWise verifies orderBy sorts the matched
// elements by the named property's text, byte-wise, so capitals precede
// lower-case names.
func TestQueryOrderByNameSortsAscendingByteWise(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedModel, orderedQuery(
		primitive(QueryPropType, opInstanceOf, false, "Usage", "Definition"), QueryPropName))
	want := []string{"Sorted::Alpha", "Sorted::Beta", "Sorted::alpha", "Sorted::alpha", "Sorted::many", "Sorted::nine", "Sorted::ten"}
	if got := queryIDs(resp); !slices.Equal(got, want) {
		t.Errorf("orderBy name = %v, want %v", got, want)
	}
}

// TestQueryOrderByListGovernsPrecedence verifies the first property sorts,
// the next breaks its ties, and declaration order breaks the last ones.
func TestQueryOrderByListGovernsPrecedence(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedModel, orderedQuery(nil, QueryPropType, QueryPropName))
	want := []string{"Sorted::alpha", "Sorted", "Sorted::Alpha", "Sorted::Beta", "Sorted::alpha", "Sorted::many", "Sorted::nine", "Sorted::ten"}
	if got := queryIDs(resp); !slices.Equal(got, want) {
		t.Errorf("orderBy @type, name = %v, want %v", got, want)
	}
	if resp.Elements[0].Type != "AttributeUsage" || resp.Elements[4].Type != "PartUsage" {
		t.Errorf("the two alpha members sort by @type first: %s then %s", resp.Elements[0].Type, resp.Elements[4].Type)
	}
	_, _, byType := runQueryOnSource(t, orderedModel, orderedQuery(primitive(QueryPropType, opEqual, false, "PartUsage"), QueryPropType))
	if got, want := queryIDs(byType), []string{"Sorted::ten", "Sorted::nine", "Sorted::many", "Sorted::alpha"}; !slices.Equal(got, want) {
		t.Errorf("orderBy @type over equal types = %v, want declaration order %v", got, want)
	}
}

// TestQueryOrderByNumberComparesNumerically verifies an ordered property sorts
// as numbers, with the unbounded "*" greatest, rather than as text.
func TestQueryOrderByNumberComparesNumerically(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedModel, orderedQuery(
		primitive(QueryPropType, opEqual, false, "PartUsage"), QueryPropMultiplicityUpper))
	want := []string{"Sorted::alpha", "Sorted::nine", "Sorted::ten", "Sorted::many"}
	if got := queryIDs(resp); !slices.Equal(got, want) {
		t.Errorf("orderBy multiplicityUpper = %v, want %v (1 < 9 < 10 < *)", got, want)
	}
}

// TestQueryOrderByBooleanPutsFalseFirst verifies a boolean property sorts
// false before true.
func TestQueryOrderByBooleanPutsFalseFirst(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedModel, orderedQuery(
		primitive(QueryPropType, opEqual, false, "PartDefinition"), QueryPropIsAbstract))
	if got, want := queryIDs(resp), []string{"Sorted::Alpha", "Sorted::Beta"}; !slices.Equal(got, want) {
		t.Errorf("orderBy isAbstract = %v, want %v (false before true)", got, want)
	}
}

// TestQueryOrderByAbsentValuesSortFirst verifies elements without the property
// precede those with it, keeping declaration order among themselves, so a
// property not every element has still gives one total order.
func TestQueryOrderByAbsentValuesSortFirst(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedModel, orderedQuery(
		primitive(QueryPropType, opEqual, false, "PartUsage"), QueryPropShortName))
	if got, want := queryIDs(resp), []string{"Sorted::ten", "Sorted::many", "Sorted::alpha", "Sorted::nine"}; !slices.Equal(got, want) {
		t.Errorf("orderBy shortName = %v, want %v (no short name first)", got, want)
	}
	_, _, mixed := runQueryOnSource(t, orderedModel, orderedQuery(nil, QueryPropMultiplicityUpper))
	want := []string{"Sorted", "Sorted::Beta", "Sorted::Alpha", "Sorted::alpha", "Sorted::alpha", "Sorted::nine", "Sorted::ten", "Sorted::many"}
	if got := queryIDs(mixed); !slices.Equal(got, want) {
		t.Errorf("orderBy multiplicityUpper over all elements = %v, want %v", got, want)
	}
}

// TestQueryOrderByDoesNotWidenTheProjection verifies sorting by a property
// select leaves out reports only what select asked for.
func TestQueryOrderByDoesNotWidenTheProjection(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedModel, orderedQuery(
		primitive(QueryPropType, opEqual, false, "PartUsage"), QueryPropMultiplicityUpper))
	for _, element := range resp.Elements {
		if _, reported := element.Properties[QueryPropMultiplicityUpper]; reported {
			t.Errorf("%s reports multiplicityUpper, which select did not ask for", element.Id)
		}
		if _, reported := element.Properties[QueryPropName]; !reported {
			t.Errorf("%s does not report name, which select asked for", element.Id)
		}
	}
}

// TestQueryOrderByUnknownPropertyFails verifies an unknown sort property is
// refused as select and where refuse one, even when nothing would be sorted.
func TestQueryOrderByUnknownPropertyFails(t *testing.T) {
	_, err := runQuery(t, &pb.Query{OrderBy: []string{"colour"}})
	assertQueryError(t, err, QueryErrUnknownProperty)
	_, err = runQuery(t, &pb.Query{
		Where:   primitive(QueryPropName, opEqual, false, "nobody"),
		OrderBy: []string{QueryPropName, "colour"},
	})
	assertQueryError(t, err, QueryErrUnknownProperty)
}

// TestQueryWithoutOrderByKeepsDeclarationOrder verifies an empty orderBy is
// the declaration order the service always reported.
func TestQueryWithoutOrderByKeepsDeclarationOrder(t *testing.T) {
	_, _, resp := runQueryOnSource(t, orderedModel, orderedQuery(primitive(QueryPropType, opInstanceOf, false, "Usage")))
	want := []string{"Sorted::ten", "Sorted::nine", "Sorted::many", "Sorted::alpha", "Sorted::alpha"}
	if got := queryIDs(resp); !slices.Equal(got, want) {
		t.Errorf("no orderBy = %v, want %v", got, want)
	}
}
