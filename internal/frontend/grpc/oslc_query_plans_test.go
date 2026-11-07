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
	resp = oslcQuery(t, srv, hash, `oslc.where=rdf:type=sysml:PartUsage&oslc.select=*`)
	if props := resp.Elements[0].Properties; props["qualifiedName"] == "" || props["@type"] != "PartUsage" {
		t.Fatalf("wildcard properties = %v", props)
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
	_, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, OslcQuery: `oslc.where=rdf:type=sysml:Package&oslc.select=sysml:isComposite`})
	assertQueryError(t, err, QueryErrUnknownProperty)
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("error %v is not a connect error", err)
	}
	for _, detail := range connErr.Details() {
		if msg, err := detail.Value(); err == nil {
			if _, ok := msg.(*pb.QueryOutcome); ok {
				return
			}
		}
	}
	t.Fatalf("no QueryOutcome detail on %v", err)
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
