package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func oslcQuery(t *testing.T, srv *Service, hash, text string) *pb.QueryResponse {
	t.Helper()
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, OslcQuery: text})
	if err != nil {
		t.Fatalf("Query %q: %v", text, err)
	}
	return resp
}

// TestOSLCQuerySelectsHeldObjects: rdf:type=elmt:<id> answers the objects the
// model's session holds of that element, each under its path, reading their
// features as object rows do.
func TestOSLCQuerySelectsHeldObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)
	wheel := "rdf:type=elmt:" + rdf.EncodeElementID("Garage::Wheel")
	if resp := oslcQuery(t, srv, hash, wheel); len(resp.Elements) != 0 {
		t.Fatalf("objects before Instantiate = %v, want none", resp.Elements)
	}
	holdObject(t, srv, hash, "Garage::car")
	holdObject(t, srv, hash, "Garage::spare")
	resp := oslcQuery(t, srv, hash, "oslc.where="+wheel+"&oslc.select=sysml:name,sysx:pressure&oslc.orderBy=-sysx:pressure")
	var got []string
	for _, element := range resp.Elements {
		got = append(got, element.Path+" "+element.Id+" "+element.Type+" "+element.Properties["pressure"])
	}
	// An object answers under its path; id and type are the usage it is an instance of.
	want := []string{
		"Garage::car.wheels[1] Garage::Car::wheels PartUsage 30",
		"Garage::car.wheels[2] Garage::Car::wheels PartUsage 30",
		"Garage::spare Garage::spare PartUsage 20",
	}
	if strings.Join(got, ";") != strings.Join(want, ";") {
		t.Fatalf("objects = %v, want %v", got, want)
	}
	resp = oslcQuery(t, srv, hash, "oslc.where="+wheel+` and sysx:pressure<25`)
	if len(resp.Elements) != 1 || resp.Elements[0].Path != "Garage::spare" {
		t.Fatalf("pressure<25 = %v, want the spare", resp.Elements)
	}
	// An object's wildcard is the features its model declares, not those its
	// types inherit from the library; a member path reads into the objects a
	// feature holds, on select, where and orderBy alike.
	car := "rdf:type=elmt:" + rdf.EncodeElementID("Garage::Car")
	resp = oslcQuery(t, srv, hash, "oslc.where="+wheel+"&oslc.select=*")
	if props := resp.Elements[0].Properties; len(props) != 1 || props["pressure"] == "" {
		t.Fatalf("a wheel's wildcard = %v, want pressure alone", props)
	}
	resp = oslcQuery(t, srv, hash, "oslc.where="+car+` and sysx:engine{sysx:power>50}&oslc.select=*,sysx:wheels{*},sysx:engine{sysx:power}&oslc.orderBy=-sysx:engine{sysx:power}`)
	if len(resp.Elements) != 1 {
		t.Fatalf("cars with a strong engine = %v, want the car", resp.Elements)
	}
	if props := resp.Elements[0].Properties; props["engine"] != "Garage::car.engine" || props["wheels.pressure"] != "30" || props["engine.power"] != "100" || len(props) != 4 {
		t.Fatalf("a car's properties = %v", props)
	}
	// Conjoined instance terms keep the objects every term selects.
	spare := "rdf:type=elmt:" + rdf.EncodeElementID("Garage::spare")
	if resp := oslcQuery(t, srv, hash, wheel+" and "+spare); len(resp.Elements) != 1 || resp.Elements[0].Path != "Garage::spare" {
		t.Fatalf("Wheel and spare = %v, want the spare", resp.Elements)
	}
	engine := "rdf:type=elmt:" + rdf.EncodeElementID("Garage::Engine")
	if resp := oslcQuery(t, srv, hash, wheel+" and "+engine); len(resp.Elements) != 0 {
		t.Fatalf("Wheel and Engine = %v, want none", resp.Elements)
	}
	if resp := oslcQuery(t, srv, hash, `rdf:type=sysml:PartUsage`); resp.Elements[0].Path != "" {
		t.Fatalf("a metaclass query answers elements, got path %q", resp.Elements[0].Path)
	}
	_, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, OslcQuery: "rdf:type=elmt:" + rdf.EncodeElementID("Garage::Nothing")})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("objects of an unknown element: %v, want INVALID_ARGUMENT", err)
	}
}

// TestOSLCQueryScopedTermsNestedSelectionAndSearch: the constructs the text
// refused before are translated — a scoped term and a nested selection as a
// property path, the wildcard as every property, searchTerms as a text match.
func TestOSLCQueryScopedTermsNestedSelectionAndSearch(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)
	resp := oslcQuery(t, srv, hash, `oslc.where=rdf:type=sysml:PartUsage and sysml:owner{sysml:name="Car"}&oslc.select=sysml:name,sysml:owner{sysml:qualifiedName}`)
	var got []string
	for _, element := range resp.Elements {
		got = append(got, element.Id+" "+element.Properties["owner.qualifiedName"])
	}
	if want := "Garage::Car::engine Garage::Car;Garage::Car::wheels Garage::Car"; strings.Join(got, ";") != want {
		t.Fatalf("scoped = %v, want %s", got, want)
	}
	// The wildcard is every property the rows' metaclass has that the
	// evaluator serves — isComposite of a PartUsage among them — and the
	// evaluator's own; a feature inherited from the library (dispatchScope)
	// is none, and a nested wildcard reads the related element's.
	resp = oslcQuery(t, srv, hash, `oslc.where=rdf:type=sysml:PartUsage&oslc.select=*,sysml:owner{*}`)
	props := resp.Elements[0].Properties
	if props["qualifiedName"] == "" || props["@type"] != "PartUsage" || props["isComposite"] != "true" || props["owner.@type"] != "PartDefinition" {
		t.Fatalf("wildcard properties = %v", props)
	}
	for _, unwanted := range []string{"*", "dispatchScope", "owner.*"} {
		if _, ok := props[unwanted]; ok {
			t.Fatalf("wildcard properties carry %q: %v", unwanted, props)
		}
	}
	resp = oslcQuery(t, srv, hash, `oslc.where=rdf:type=sysml:PartUsage&oslc.searchTerms="WHEEL"`)
	got = got[:0]
	for _, element := range resp.Elements {
		got = append(got, element.Id)
	}
	if want := "Garage::Car::wheels;Garage::CarReport::wheels"; strings.Join(got, ";") != want {
		t.Fatalf("search = %v, want %s", got, want)
	}
	_, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, OslcQuery: `sysml:name="a" or sysml:name="b"`})
	assertQueryError(t, err, QueryErrUnsupported)
}

// TestQueryFailureCarriesTheQueryOutcome: a plan the evaluator refuses while
// reading the model attaches the QVTS record as a QueryOutcome detail.
func TestQueryFailureCarriesTheQueryOutcome(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)
	// The evaluator's refusal: a property no row carries.
	_, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, OslcQuery: `oslc.where=rdf:type=sysml:Package&oslc.select=sysml:isComposite`})
	assertQueryError(t, err, QueryErrUnknownProperty)
	if outcome := queryOutcomeOf(t, err); outcome.Kind != "pathError" {
		t.Fatalf("outcome = %v, want a path error", outcome)
	}
	// The translator's: a property the metamodel does not define, with the
	// nearest spelling on the frontier — in OSLC text and the structured form.
	_, err = srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, OslcQuery: `sysml:nmae="car"`})
	assertQueryError(t, err, QueryErrUnknownProperty)
	if outcome := queryOutcomeOf(t, err); len(outcome.Failures) != 1 || outcome.Failures[0].GetFrontier().GetNearest() != "sysml:name" {
		t.Fatalf("outcome = %v, want sysml:name nearest", outcome)
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: &pb.Query{Select: []string{"nmae"}}})
	assertQueryError(t, err, QueryErrUnknownProperty)
	if outcome := queryOutcomeOf(t, err); len(outcome.Failures) != 1 || outcome.Failures[0].GetFrontier().GetNearest() != "name" {
		t.Fatalf("outcome = %v, want name nearest", outcome)
	}
}

// queryOutcomeOf is the QueryOutcome detail a failed Query carries.
func queryOutcomeOf(t *testing.T, err error) *pb.QueryOutcome {
	t.Helper()
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("error %v is not a connect error", err)
	}
	for _, detail := range connErr.Details() {
		if msg, err := detail.Value(); err == nil {
			if outcome, ok := msg.(*pb.QueryOutcome); ok {
				return outcome
			}
		}
	}
	t.Fatalf("no QueryOutcome detail on %v", err)
	return nil
}

// TestAPIQuerySelectsHeldObjects: the structured Query reaches the objects
// through the same translation, `@type in [<urn:sysmlv2:element:id>]`.
func TestAPIQuerySelectsHeldObjects(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, objectFixture)
	holdObject(t, srv, hash, "Garage::car")
	iri := "<" + rdf.Element + rdf.EncodeElementID("Garage::Wheel") + ">"
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: &pb.Query{
		Where:  primitive(QueryPropType, opIn, false, iri),
		Select: []string{"pressure"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Elements) != 2 || resp.Elements[1].Path != "Garage::car.wheels[2]" || resp.Elements[1].Properties["pressure"] != "30" {
		t.Fatalf("objects = %v", resp.Elements)
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: &pb.Query{
		Where: primitive(QueryPropType, opEqual, true, iri),
	}})
	assertQueryError(t, err, QueryErrUnsupported)
	_, err = srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: &pb.Query{
		Where: primitive(QueryPropType, opIn, false, iri, "PartUsage"),
	}})
	assertQueryError(t, err, QueryErrUnsupported)
}

// TestQueryReportsAnEmptyValueAsPresent: a property whose value is the empty
// string is reported as "", which a client tells from an absent property.
func TestQueryReportsAnEmptyValueAsPresent(t *testing.T) {
	_, _, resp := runQueryOnSource(t, "package Demo { comment C /**/ part p; }", &pb.Query{
		Where:  primitive(QueryPropType, opEqual, false, "Comment"),
		Select: []string{QueryPropBody, QueryPropDocumentation},
	})
	if len(resp.Elements) != 1 {
		t.Fatalf("comments = %v, want one", resp.Elements)
	}
	props := resp.Elements[0].Properties
	body, present := props[QueryPropBody]
	if !present || body != "" {
		t.Fatalf("body = %q (present %v), want the empty string present", body, present)
	}
	if _, present := props[QueryPropDocumentation]; present {
		t.Fatalf("documentation of a comment is present: %v", props)
	}
}
