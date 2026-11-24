package grpc

import (
	"context"
	"os"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// boundSubjectFixture pins both binding forms and a case that binds nothing.
const boundSubjectFixture = "../../../tests/testdata/bound-subject/verification_subjects.sysml"

func boundSubjectModel(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(boundSubjectFixture)
	if err != nil {
		t.Fatalf("read %s: %v", boundSubjectFixture, err)
	}
	return string(data)
}

// TestQueryBoundSubjectNamesTheBoundDeclaration pins the bound FQN on each
// case row, and absence on the unbound case — never the subject parameter.
func TestQueryBoundSubjectNamesTheBoundDeclaration(t *testing.T) {
	_, _, resp := runQueryOnSource(t, boundSubjectModel(t), &pb.Query{
		Select: []string{QueryPropID, QueryPropBoundSubject},
	})
	got := map[string]string{}
	for _, element := range resp.Elements {
		if value, ok := element.Properties[QueryPropBoundSubject]; ok {
			got[element.Id] = value
		}
	}
	want := map[string]string{
		"Verification::maintainedSizing": "Structure::boilerAsMaintained",
		"Verification::spareSizing":      "Structure::spareBoiler",
	}
	if len(got) != len(want) {
		t.Errorf("boundSubject rows = %v, want %v", got, want)
	}
	for id, value := range want {
		if got[id] != value {
			t.Errorf("boundSubject of %s = %q, want %q", id, got[id], value)
		}
	}
	if _, present := got["Verification::unboundSizing"]; present {
		t.Error("a case binding no subject answered a boundSubject")
	}
	if _, present := got["Verification::maintainedSizing::s"]; present {
		t.Error("a subject parameter was answered as a bound subject")
	}
}

// TestQueryBoundSubjectSelectsCasesByTheirBinding pins an equality on the
// bound FQN selecting only the case bound to that declaration.
func TestQueryBoundSubjectSelectsCasesByTheirBinding(t *testing.T) {
	_, _, resp := runQueryOnSource(t, boundSubjectModel(t), &pb.Query{
		Where:  primitive(QueryPropBoundSubject, opEqual, false, "Structure::spareBoiler"),
		Select: []string{QueryPropID},
	})
	if len(resp.Elements) != 1 || resp.Elements[0].Id != "Verification::spareSizing" {
		var ids []string
		for _, element := range resp.Elements {
			ids = append(ids, element.Id)
		}
		t.Errorf("elements = %v, want Verification::spareSizing", ids)
	}
}

// TestQueryOSLCBoundSubject verifies the OpenSysML-namespace spelling works in
// OSLC query text, as the other derived properties do.
func TestQueryOSLCBoundSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: boundSubjectModel(t)},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		OslcQuery: `oslc.where=sysx:boundSubject="Structure::boilerAsMaintained"&oslc.select=sysml:name`,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(resp.Elements) != 1 || resp.Elements[0].Properties["name"] != "maintainedSizing" {
		t.Errorf("OSLC elements = %+v, want only maintainedSizing", resp.Elements)
	}
}

// TestQueryStillRefusesAPropertyNextToBoundSubject verifies the addition leaves
// every other undeclared name an unknown property.
func TestQueryStillRefusesAPropertyNextToBoundSubject(t *testing.T) {
	for _, name := range []string{"boundsubject", "bound_subject", "subjectFQN"} {
		_, err := runQuery(t, &pb.Query{Select: []string{name}})
		assertQueryError(t, err, QueryErrUnknownProperty)
	}
}
