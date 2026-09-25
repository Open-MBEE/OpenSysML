package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// queryModel is the model the query tests run against: parts and attributes at
// two levels, with a multiplicity to compare and an abstract definition.
const queryModel = `
package Demo {
	abstract part def Vehicle {
		attribute mass;
	}
	part def Wheel;
	part vehicle : Vehicle {
		part <W> wheels : Wheel[4] {
			doc /* Four road wheels,
			     * one per corner. */
		}
		attribute vin;
	}
	part spare : Wheel;
}
`

// runQuery parses queryModel and runs one query over it.
func runQuery(t *testing.T, query *pb.Query) (*pb.QueryResponse, error) {
	t.Helper()
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: queryModel},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	return srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     query,
	})
}

// mustRunQuery runs a query that is expected to succeed and returns the
// qualified names it selected.
func mustRunQuery(t *testing.T, query *pb.Query) []string {
	t.Helper()
	resp, err := runQuery(t, query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	ids := make([]string, 0, len(resp.Elements))
	for _, element := range resp.Elements {
		ids = append(ids, element.Id)
	}
	return ids
}

// primitive builds a PrimitiveConstraint as a Constraint.
func primitive(property string, op pb.PrimitiveOperator, inverse bool, values ...string) *pb.Constraint {
	return &pb.Constraint{Constraint: &pb.Constraint_Primitive{
		Primitive: &pb.PrimitiveConstraint{
			Property: property,
			Operator: op,
			Inverse:  inverse,
			Value:    values,
		},
	}}
}

// composite builds a CompositeConstraint as a Constraint.
func composite(op pb.CompositeOperator, nested ...*pb.Constraint) *pb.Constraint {
	return &pb.Constraint{Constraint: &pb.Constraint_Composite{
		Composite: &pb.CompositeConstraint{Operator: op, Constraint: nested},
	}}
}

const opEqual = pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL

// TestQueryCapabilityIsReported verifies a client can require the Query RPC by
// capability rather than by version.
func TestQueryCapabilityIsReported(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo failed: %v", err)
	}
	if !slices.Contains(info.Capabilities, CapabilityQuery) {
		t.Errorf("capabilities = %v, want it to contain %q", info.Capabilities, CapabilityQuery)
	}
	if !slices.Contains(info.Capabilities, CapabilityOSLCQuery) {
		t.Errorf("capabilities = %v, want it to contain %q", info.Capabilities, CapabilityOSLCQuery)
	}
}

// TestQueryByTypeSelectsThatMetamodelType verifies the cookbook payload shape:
// `@type` = ["PartUsage"] over the whole model.
func TestQueryByTypeSelectsThatMetamodelType(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropType, opEqual, false, "PartUsage"),
	})
	want := []string{"Demo::vehicle", "Demo::vehicle::wheels", "Demo::spare"}
	if !slices.Equal(ids, want) {
		t.Errorf("part usages = %v, want %v", ids, want)
	}
}

// TestQueryWithoutWhereSelectsWholeScope verifies an absent constraint filters
// nothing, and that enumeration reaches nested elements.
func TestQueryWithoutWhereSelectsWholeScope(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{})
	want := []string{
		"Demo",
		"Demo::Vehicle",
		"Demo::Vehicle::mass",
		"Demo::Wheel",
		"Demo::vehicle",
		"Demo::vehicle::wheels",
		"Demo::vehicle::wheels::@0",
		"Demo::vehicle::vin",
		"Demo::spare",
	}
	if !slices.Equal(ids, want) {
		t.Errorf("elements = %v, want %v", ids, want)
	}
}

// TestQueryScopeRestrictsToAnElementAndItsNested verifies a scope considers the
// named element and everything nested inside it, and nothing else.
func TestQueryScopeRestrictsToAnElementAndItsNested(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{Scope: []string{"Demo::vehicle"}})
	want := []string{"Demo::vehicle", "Demo::vehicle::wheels", "Demo::vehicle::wheels::@0", "Demo::vehicle::vin"}
	if !slices.Equal(ids, want) {
		t.Errorf("scoped elements = %v, want %v", ids, want)
	}
}

// TestQueryScopeAndWhereCombine verifies a scope and a constraint apply
// together.
func TestQueryScopeAndWhereCombine(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{
		Scope: []string{"Demo::vehicle"},
		Where: primitive(QueryPropType, opEqual, false, "PartUsage"),
	})
	want := []string{"Demo::vehicle", "Demo::vehicle::wheels"}
	if !slices.Equal(ids, want) {
		t.Errorf("scoped part usages = %v, want %v", ids, want)
	}
}

// TestQueryUnknownScopeFails verifies a scope naming an element the model does
// not have fails the call rather than answering with nothing.
func TestQueryUnknownScopeFails(t *testing.T) {
	_, err := runQuery(t, &pb.Query{Scope: []string{"Demo::Missing"}})
	assertQueryError(t, err, QueryErrUnknownScope)
}

// TestQuerySelectProjectsOnlyThoseProperties verifies `select` projects the
// named properties, while identity and type stay reported.
func TestQuerySelectProjectsOnlyThoseProperties(t *testing.T) {
	resp, err := runQuery(t, &pb.Query{
		Select: []string{QueryPropName, QueryPropOwner},
		Where:  primitive(QueryPropQualifiedName, opEqual, false, "Demo::vehicle::wheels"),
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Elements) != 1 {
		t.Fatalf("elements = %d, want 1", len(resp.Elements))
	}
	element := resp.Elements[0]
	if element.Id != "Demo::vehicle::wheels" || element.Type != "PartUsage" {
		t.Errorf("element = %q (%s), want Demo::vehicle::wheels (PartUsage)", element.Id, element.Type)
	}
	want := map[string]string{QueryPropName: "wheels", QueryPropOwner: "Demo::vehicle"}
	if len(element.Properties) != len(want) {
		t.Fatalf("properties = %v, want only %v", element.Properties, want)
	}
	for name, value := range want {
		if element.Properties[name] != value {
			t.Errorf("properties[%q] = %q, want %q", name, element.Properties[name], value)
		}
	}
}

// TestQuerySelectReportsEveryPropertyByDefault verifies an empty selection
// reports every queryable property the element has, and omits those it lacks.
func TestQuerySelectReportsEveryPropertyByDefault(t *testing.T) {
	resp, err := runQuery(t, &pb.Query{
		Where: primitive(QueryPropQualifiedName, opEqual, false, "Demo::vehicle::wheels"),
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Elements) != 1 {
		t.Fatalf("elements = %d, want 1", len(resp.Elements))
	}
	props := resp.Elements[0].Properties
	for name, want := range map[string]string{
		QueryPropID:                "Demo::vehicle::wheels",
		QueryPropType:              "PartUsage",
		QueryPropName:              "wheels",
		QueryPropDeclaredName:      "wheels",
		QueryPropShortName:         "W",
		QueryPropDeclaredShortName: "W",
		QueryPropDocumentation:     "Four road wheels,\none per corner.",
		QueryPropQualifiedName:     "Demo::vehicle::wheels",
		QueryPropOwner:             "Demo::vehicle",
		QueryPropIsAbstract:        "false",
		QueryPropElementType:       "Demo::Wheel",
		QueryPropMultiplicityLower: "4",
		QueryPropMultiplicityUpper: "4",
	} {
		if props[name] != want {
			t.Errorf("properties[%q] = %q, want %q", name, props[name], want)
		}
	}
}

// TestQueryOmitsPropertiesAnElementDoesNotHave verifies a property with no
// value is absent from the record rather than reported empty.
func TestQueryOmitsPropertiesAnElementDoesNotHave(t *testing.T) {
	resp, err := runQuery(t, &pb.Query{
		Where: primitive(QueryPropQualifiedName, opEqual, false, "Demo"),
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Elements) != 1 {
		t.Fatalf("elements = %d, want 1", len(resp.Elements))
	}
	props := resp.Elements[0].Properties
	for _, absent := range []string{QueryPropOwner, QueryPropIsAbstract, QueryPropMultiplicityLower, QueryPropShortName, QueryPropDocumentation} {
		if value, ok := props[absent]; ok {
			t.Errorf("properties[%q] = %q, want it absent for a top-level package", absent, value)
		}
	}
}

// TestQuerySelectUnknownPropertyFails verifies projecting an unknown property
// fails the call.
func TestQuerySelectUnknownPropertyFails(t *testing.T) {
	_, err := runQuery(t, &pb.Query{Select: []string{"effectiveName"}})
	assertQueryError(t, err, QueryErrUnknownProperty)
}

// TestQueryUnknownPropertyFailsRatherThanMatchingNothing verifies an unknown
// property in a constraint is an error, not an empty result.
func TestQueryUnknownPropertyFailsRatherThanMatchingNothing(t *testing.T) {
	_, err := runQuery(t, &pb.Query{
		Where: primitive("colour", opEqual, false, "red"),
	})
	assertQueryError(t, err, QueryErrUnknownProperty)
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Errorf("status code = %s, want %s", got, connect.CodeInvalidArgument)
	}
}

// TestQueryInverseNegatesTheVerdict verifies `inverse` selects exactly the
// complement of the constraint within the same scope.
func TestQueryInverseNegatesTheVerdict(t *testing.T) {
	plain := mustRunQuery(t, &pb.Query{
		Scope: []string{"Demo"},
		Where: primitive(QueryPropType, opEqual, false, "PartUsage"),
	})
	inverse := mustRunQuery(t, &pb.Query{
		Scope: []string{"Demo"},
		Where: primitive(QueryPropType, opEqual, true, "PartUsage"),
	})
	all := mustRunQuery(t, &pb.Query{Scope: []string{"Demo"}})
	if len(plain)+len(inverse) != len(all) {
		t.Errorf("plain %v plus inverse %v does not partition %v", plain, inverse, all)
	}
	for _, id := range inverse {
		if slices.Contains(plain, id) {
			t.Errorf("%q matched both the constraint and its inverse", id)
		}
	}
}

// TestQueryEqualMatchesAnyOfAListedValue verifies `=` against a list, which is
// how the standard's clients write a `@type` filter.
func TestQueryEqualMatchesAnyOfAListedValue(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropType, opEqual, false, "PartDefinition", "AttributeUsage"),
	})
	want := []string{"Demo::Vehicle", "Demo::Vehicle::mass", "Demo::Wheel", "Demo::vehicle::vin"}
	if !slices.Equal(ids, want) {
		t.Errorf("elements = %v, want %v", ids, want)
	}
}

// TestQueryCompositeNesting verifies and/or nesting, including a composite
// inside a composite.
func TestQueryCompositeNesting(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{
		Where: composite(pb.CompositeOperator_COMPOSITE_OPERATOR_AND,
			primitive(QueryPropType, opEqual, false, "PartUsage"),
			composite(pb.CompositeOperator_COMPOSITE_OPERATOR_OR,
				primitive(QueryPropName, opEqual, false, "wheels"),
				primitive(QueryPropName, opEqual, false, "spare"),
			),
		),
	})
	want := []string{"Demo::vehicle::wheels", "Demo::spare"}
	if !slices.Equal(ids, want) {
		t.Errorf("elements = %v, want %v", ids, want)
	}
}

// TestQueryOrderedComparisonOnMultiplicity verifies > and < on the one ordered
// property pair, and that a bound of a usage that declares none does not match.
func TestQueryOrderedComparisonOnMultiplicity(t *testing.T) {
	greater := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropMultiplicityLower,
			pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER, false, "1"),
	})
	if !slices.Equal(greater, []string{"Demo::vehicle::wheels"}) {
		t.Errorf("multiplicityLower > 1 = %v, want [Demo::vehicle::wheels]", greater)
	}
	less := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropMultiplicityUpper,
			pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS, false, "2"),
	})
	if len(less) != 0 {
		t.Errorf("multiplicityUpper < 2 = %v, want no elements", less)
	}
}

// TestQueryOrderedComparisonOnUnorderedPropertyFails verifies > on a property
// that is not ordered is an error rather than a false verdict.
func TestQueryOrderedComparisonOnUnorderedPropertyFails(t *testing.T) {
	_, err := runQuery(t, &pb.Query{
		Where: primitive(QueryPropName, pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER, false, "a"),
	})
	assertQueryError(t, err, QueryErrUnorderedProperty)
}

// TestQueryOrderedComparisonAgainstNonNumberFails verifies an operand that is
// not a number is an error rather than a false verdict.
func TestQueryOrderedComparisonAgainstNonNumberFails(t *testing.T) {
	_, err := runQuery(t, &pb.Query{
		Where: primitive(QueryPropMultiplicityLower,
			pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS, false, "many"),
	})
	assertQueryError(t, err, QueryErrUnparsableValue)
}

// TestQueryOrderedComparisonNeedsOneOperand verifies > and < reject a value
// list, which they have no comparison for.
func TestQueryOrderedComparisonNeedsOneOperand(t *testing.T) {
	_, err := runQuery(t, &pb.Query{
		Where: primitive(QueryPropMultiplicityLower,
			pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER, false, "1", "2"),
	})
	assertQueryError(t, err, QueryErrMalformedConstraint)
}

// TestQueryMalformedConstraintsFail verifies every shape a query can be broken
// in fails the call, naming what is wrong.
func TestQueryMalformedConstraintsFail(t *testing.T) {
	cases := map[string]*pb.Constraint{
		"no form":            {},
		"unset primitive":    {Constraint: &pb.Constraint_Primitive{}},
		"unset composite":    {Constraint: &pb.Constraint_Composite{}},
		"no operator":        primitive(QueryPropName, pb.PrimitiveOperator_PRIMITIVE_OPERATOR_UNSPECIFIED, false, "vehicle"),
		"no value":           primitive(QueryPropName, opEqual, false),
		"empty composite":    composite(pb.CompositeOperator_COMPOSITE_OPERATOR_AND),
		"composite operator": {Constraint: &pb.Constraint_Composite{Composite: &pb.CompositeConstraint{Constraint: []*pb.Constraint{primitive(QueryPropName, opEqual, false, "vehicle")}}}},
	}
	for name, constraint := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runQuery(t, &pb.Query{Where: constraint})
			assertQueryError(t, err, QueryErrMalformedConstraint)
		})
	}
}

// TestQueryFaultIsReportedWithNoElementsToConsider verifies a query is judged
// before any element is: over a model that declares nothing, an invalid query
// still fails rather than reading as "nothing matched".
func TestQueryFaultIsReportedWithNoElementsToConsider(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: "// nothing but a comment\n"},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	empty, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{},
	})
	if err != nil {
		t.Fatalf("Query over an empty model failed: %v", err)
	}
	if len(empty.Elements) != 0 {
		t.Fatalf("elements = %v, want the model to declare none", empty.Elements)
	}

	cases := map[string]struct {
		where *pb.Constraint
		want  QueryErrorKind
	}{
		"unknown property":   {primitive("colour", opEqual, false, "red"), QueryErrUnknownProperty},
		"no operator":        {primitive(QueryPropName, pb.PrimitiveOperator_PRIMITIVE_OPERATOR_UNSPECIFIED, false, "x"), QueryErrMalformedConstraint},
		"empty composite":    {composite(pb.CompositeOperator_COMPOSITE_OPERATOR_OR), QueryErrMalformedConstraint},
		"unordered property": {primitive(QueryPropName, pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER, false, "1"), QueryErrUnorderedProperty},
		"unparsable operand": {primitive(QueryPropMultiplicityUpper, pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS, false, "many"), QueryErrUnparsableValue},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := srv.Query(context.Background(), &pb.QueryRequest{
				ModelHash: parsed.ModelHash,
				Query:     &pb.Query{Where: tc.where},
			})
			assertQueryError(t, err, tc.want)
		})
	}
}

// TestQueryFaultUnderADecisiveConstraintIsReported verifies a malformed nested
// constraint fails the call even when a sibling already decides the verdict.
func TestQueryFaultUnderADecisiveConstraintIsReported(t *testing.T) {
	_, err := runQuery(t, &pb.Query{
		Where: composite(pb.CompositeOperator_COMPOSITE_OPERATOR_OR,
			primitive(QueryPropType, opEqual, false, "PartUsage"),
			primitive("colour", opEqual, false, "red")),
	})
	assertQueryError(t, err, QueryErrUnknownProperty)
}

// TestQueryUnsetQueryFails verifies a request with no query at all fails.
func TestQueryUnsetQueryFails(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: queryModel},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{ModelHash: parsed.ModelHash})
	assertQueryError(t, err, QueryErrMalformedConstraint)
}

// TestQueryMatchingNothingIsEmptyNotAnError verifies a well-formed query that
// selects nothing answers with no elements.
func TestQueryMatchingNothingIsEmptyNotAnError(t *testing.T) {
	resp, err := runQuery(t, &pb.Query{
		Where: primitive(QueryPropName, opEqual, false, "Nonexistent"),
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Elements) != 0 {
		t.Errorf("elements = %v, want none", resp.Elements)
	}
}

// TestQueryUnknownModelFails verifies a model the cache no longer holds is
// reported as NOT_FOUND rather than as an empty answer.
func TestQueryUnknownModelFails(t *testing.T) {
	srv := mustNewService(t, 10)
	_, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: "nosuchmodel",
		Query:     &pb.Query{},
	})
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Fatalf("status code = %s, want %s (err: %v)", got, connect.CodeNotFound, err)
	}
}

// TestQueryIsAbstractSelectsAbstractDefinitions verifies the boolean property
// compares as the text of the boolean.
func TestQueryIsAbstractSelectsAbstractDefinitions(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropIsAbstract, opEqual, false, "true"),
	})
	if !slices.Equal(ids, []string{"Demo::Vehicle"}) {
		t.Errorf("abstract elements = %v, want [Demo::Vehicle]", ids)
	}
}

// TestQueryTypePropertyReportsResolvedType verifies `type` reports the resolved
// type's qualified name, which is how a client follows a usage to its
// definition.
func TestQueryTypePropertyReportsResolvedType(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{
		Where: primitive(QueryPropElementType, opEqual, false, "Demo::Wheel"),
	})
	want := []string{"Demo::vehicle::wheels", "Demo::spare"}
	if !slices.Equal(ids, want) {
		t.Errorf("elements typed by Demo::Wheel = %v, want %v", ids, want)
	}
}

// TestQueryScopeMayNameALibraryElement verifies a scope is not restricted to
// the parsed document: the loaded standard library is queryable too.
func TestQueryScopeMayNameALibraryElement(t *testing.T) {
	ids := mustRunQuery(t, &pb.Query{
		Scope: []string{"ScalarValues"},
		Where: primitive(QueryPropName, opEqual, false, "Real"),
	})
	if !slices.Contains(ids, "ScalarValues::Real") {
		t.Errorf("elements = %v, want it to contain ScalarValues::Real", ids)
	}
}

// TestQueryUsesPositionalIdentityForUnnamedElements verifies exported positions
// identify unnamed declarations without making body-local names queryable.
func TestQueryUsesPositionalIdentityForUnnamedElements(t *testing.T) {
	const model = `
package Anon {
	doc /* the package's documentation, which is unnamed */
	part def Rig;
	part def Motor;
	part rig : Rig {
		part : Motor;
	}
	connect rig to rig;
}
`
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: model},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	seen := make(map[string]bool, len(resp.Elements))
	positional := 0
	for _, element := range resp.Elements {
		if isPositionalIdentity(element.Id) {
			positional++
		}
		if seen[element.Id] {
			t.Errorf("element id %q was reported twice", element.Id)
		}
		seen[element.Id] = true
	}
	for _, want := range []string{"Anon", "Anon::Rig", "Anon::rig"} {
		if !seen[want] {
			t.Errorf("elements = %v, want it to contain %s", resp.Elements, want)
		}
	}
	if positional != 3 {
		t.Errorf("positional element count = %d, want 3 (doc, anonymous part, connect): %v", positional, resp.Elements)
	}
}

const satisfyQueryModel = `package Demo {
	part def Toaster;
	requirement def EnergyReq { subject t : Toaster; }
	requirement r : EnergyReq { subject t : Toaster; }
	part t : Toaster;
	assert satisfy r by t;
}`

func runQueryOnSource(t *testing.T, model string, query *pb.Query) (*Service, string, *pb.QueryResponse) {
	t.Helper()
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: model},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     query,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return srv, parsed.ModelHash, resp
}

func satisfyUsageQuery(selected ...string) *pb.Query {
	return &pb.Query{
		Where:  primitive(QueryPropType, opEqual, false, "SatisfyRequirementUsage"),
		Select: selected,
	}
}

func TestQuerySatisfyEndsUsePositionalIdentity(t *testing.T) {
	srv, modelHash, resp := runQueryOnSource(t, satisfyQueryModel, satisfyUsageQuery(
		QueryPropSatisfiedRequirement, QueryPropSatisfyingFeature, QueryPropOwner, QueryPropQualifiedName,
	))
	if len(resp.Elements) != 1 {
		t.Fatalf("satisfy elements = %d, want 1: %v", len(resp.Elements), resp.Elements)
	}
	element := resp.Elements[0]
	if element.Id != "Demo::@4" {
		t.Errorf("satisfy id = %q, want Demo::@4", element.Id)
	}
	for property, want := range map[string]string{
		QueryPropSatisfiedRequirement: "Demo::r",
		QueryPropSatisfyingFeature:    "Demo::t",
		QueryPropOwner:                "Demo",
	} {
		if got := element.Properties[property]; got != want {
			t.Errorf("properties[%q] = %q, want %q", property, got, want)
		}
	}
	if _, ok := element.Properties[QueryPropQualifiedName]; ok {
		t.Errorf("qualifiedName = %q, want absent for positional identity", element.Properties[QueryPropQualifiedName])
	}

	convertResp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_ModelHash{ModelHash: modelHash},
		FromFormat: "sysml",
		ToFormat:   "api-json",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if convertResp.Error != "" {
		t.Fatalf("Convert reported %q", convertResp.Error)
	}
	var elements []struct {
		Type string `json:"@type"`
		ID   string `json:"@id"`
	}
	if err := json.Unmarshal([]byte(convertResp.Content), &elements); err != nil {
		t.Fatalf("decode API JSON: %v", err)
	}
	var apiID string
	for _, exported := range elements {
		if exported.Type == "SatisfyRequirementUsage" {
			apiID = exported.ID
			break
		}
	}
	if want := rdf.EncodeElementID(element.Id); apiID != want {
		t.Errorf("API JSON satisfy id = %q, want rdf.EncodeElementID(%q) = %q", apiID, element.Id, want)
	}

	scoped, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: modelHash,
		Query: &pb.Query{
			Scope: []string{element.Id},
			Where: primitive(QueryPropID, opEqual, false, element.Id),
		},
	})
	if err != nil {
		t.Fatalf("Query by positional scope and @id: %v", err)
	}
	if len(scoped.Elements) != 1 || scoped.Elements[0].Id != element.Id {
		t.Errorf("scoped results = %v, want the satisfy %q", scoped.Elements, element.Id)
	}
}

func TestQueryDeclaredSatisfyRequirementUsesOwnIdentity(t *testing.T) {
	model := `package Demo {
		part def Toaster;
		requirement def EnergyReq { subject t : Toaster; }
		part t : Toaster;
		assert satisfy requirement s1 : EnergyReq by t;
	}`
	_, _, resp := runQueryOnSource(t, model, satisfyUsageQuery(
		QueryPropSatisfiedRequirement, QueryPropSatisfyingFeature,
	))
	if len(resp.Elements) != 1 {
		t.Fatalf("satisfy elements = %d, want 1: %v", len(resp.Elements), resp.Elements)
	}
	element := resp.Elements[0]
	if element.Id != "Demo::s1" {
		t.Errorf("satisfy id = %q, want Demo::s1", element.Id)
	}
	for property, want := range map[string]string{
		QueryPropSatisfiedRequirement: "Demo::s1",
		QueryPropSatisfyingFeature:    "Demo::t",
	} {
		if got := element.Properties[property]; got != want {
			t.Errorf("properties[%q] = %q, want %q", property, got, want)
		}
	}
}

func TestQuerySatisfyWithoutByAndFeatureChain(t *testing.T) {
	t.Run("no by", func(t *testing.T) {
		model := `package Demo {
			part def Toaster;
			requirement r;
			part t : Toaster { satisfy r; }
		}`
		_, _, resp := runQueryOnSource(t, model, satisfyUsageQuery(
			QueryPropSatisfiedRequirement, QueryPropSatisfyingFeature,
		))
		if len(resp.Elements) != 1 {
			t.Fatalf("satisfy elements = %d, want 1: %v", len(resp.Elements), resp.Elements)
		}
		if got := resp.Elements[0].Properties[QueryPropSatisfiedRequirement]; got != "Demo::r" {
			t.Errorf("satisfiedRequirement = %q, want Demo::r", got)
		}
		if _, ok := resp.Elements[0].Properties[QueryPropSatisfyingFeature]; ok {
			t.Errorf("satisfyingFeature = %q, want absent", resp.Elements[0].Properties[QueryPropSatisfyingFeature])
		}
	})
	t.Run("feature chain", func(t *testing.T) {
		model := `package Demo {
			part def Toaster;
			requirement r;
			part def Vehicle { part heater : Toaster; }
			part v : Vehicle;
			assert satisfy r by v.heater;
		}`
		_, _, resp := runQueryOnSource(t, model, satisfyUsageQuery(QueryPropSatisfyingFeature))
		if len(resp.Elements) != 1 {
			t.Fatalf("satisfy elements = %d, want 1: %v", len(resp.Elements), resp.Elements)
		}
		if got := resp.Elements[0].Properties[QueryPropSatisfyingFeature]; got != "Demo::Vehicle::heater" {
			t.Errorf("satisfyingFeature = %q, want Demo::Vehicle::heater", got)
		}
	})
}

func TestQueryVerifyHasNoSatisfyEndProperties(t *testing.T) {
	model := `package Demo {
		requirement r;
		verification def Check {
			objective { verify r; }
		}
	}`
	_, _, resp := runQueryOnSource(t, model, satisfyUsageQuery(
		QueryPropSatisfiedRequirement, QueryPropSatisfyingFeature,
	))
	if len(resp.Elements) != 1 {
		t.Fatalf("verify elements = %d, want 1: %v", len(resp.Elements), resp.Elements)
	}
	for _, property := range []string{QueryPropSatisfiedRequirement, QueryPropSatisfyingFeature} {
		if _, ok := resp.Elements[0].Properties[property]; ok {
			t.Errorf("verify properties[%q] = %q, want absent", property, resp.Elements[0].Properties[property])
		}
	}
}

func TestQueryOmitsPositionalIdentityWhenExporterRefusesCollision(t *testing.T) {
	model := `package Collision {
		metadata def M;
		#M part def Car {
			part def '@1';
		}
		part a;
		part b;
		connect a to b;
	}`
	_, _, resp := runQueryOnSource(t, model, &pb.Query{})
	seen := make(map[string]bool, len(resp.Elements))
	for _, element := range resp.Elements {
		if seen[element.Id] {
			t.Errorf("element id %q was reported more than once", element.Id)
		}
		seen[element.Id] = true
	}
	for _, want := range []string{"Collision", "Collision::M", "Collision::Car", "Collision::Car::@1", "Collision::a", "Collision::b"} {
		if !seen[want] {
			t.Errorf("elements = %v, want named element %q", resp.Elements, want)
		}
	}
	for _, element := range resp.Elements {
		if isPositionalIdentity(element.Id) && element.Id != "Collision::Car::@1" {
			t.Errorf("element id = %q, want no positional ids after the exporter refused the collision", element.Id)
		}
	}
}

// TestQueryOmitsBodyLocalDeclarations verifies an element declared inside an
// action body is not reported: the scope that declares it is owned by no symbol,
// so its qualified name is a bare local one that names no element back.
func TestQueryOmitsBodyLocalDeclarations(t *testing.T) {
	const model = `
package Demo {
	action def Drive {
		if true { action step; } else { action step; }
	}
	action step;
}
`
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: model},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	var ids []string
	for _, element := range resp.Elements {
		ids = append(ids, element.Id)
	}
	want := []string{"Demo", "Demo::Drive", "Demo::step"}
	if !slices.Equal(ids, want) {
		t.Errorf("element ids = %v, want %v", ids, want)
	}
}

// TestMetamodelTypeNameCoversEveryKind verifies the mapping is total over the
// kinds a parsed declaration can have.
func TestMetamodelTypeNameCoversEveryKind(t *testing.T) {
	for kind := symbols.SymbolPackage; kind <= symbols.SymbolConnectorEnd; kind++ {
		if corequery.MetamodelTypeName(kind) == "" {
			t.Errorf("symbol kind %q has no metamodel type name", kind)
		}
	}
	if corequery.MetamodelTypeName(symbols.SymbolUnknown) != "" {
		t.Error("an unclassified declaration must have no metamodel type name")
	}
}

// assertQueryError verifies a query failed with the expected kind of typed
// error, reported as INVALID_ARGUMENT.
func assertQueryError(t *testing.T, err error, want QueryErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("query succeeded, want a %v error", want)
	}
	var qerr *QueryError
	if !errors.As(err, &qerr) {
		t.Fatalf("error %v is not a *QueryError", err)
	}
	if qerr.Kind != want {
		t.Errorf("error kind = %v, want %v (message: %s)", qerr.Kind, want, qerr.Message)
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Errorf("status code = %s, want %s", got, connect.CodeInvalidArgument)
	}
}
