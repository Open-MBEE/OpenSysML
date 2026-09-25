package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// libraryDocsModel projects the documentation of the element bound as root.
const libraryDocsModel = `package LibraryDocs {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	calc def Doc :> Query {
		in root : Element;
		Project(source = root, properties = ("name", "documentation"))
	}
}
`

// A cached model reads the documentation of standard-library elements from the
// library files its index was built from, not only from the request's documents.
func TestRunDocumentQueryReadsLibraryDocumentation(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: libraryDocsModel},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: parsed.ModelHash,
		QueryId:   "LibraryDocs::Doc",
		Bindings:  []*pb.DocumentQueryBinding{binding("root", element("Parts::Part"))},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	if len(resp.Rows) != 1 || len(resp.Rows[0].Cells) != 2 {
		t.Fatalf("rows = %v, want one row of two cells", resp.Rows)
	}
	if got := stringCell(t, resp.Rows[0].Cells[0]); got != "Part" {
		t.Errorf("name = %q, want Part", got)
	}
	doc := stringCell(t, resp.Rows[0].Cells[1])
	if !strings.HasPrefix(doc, "Part is the most general class of objects") {
		t.Errorf("documentation = %q, want the library's doc body", doc)
	}
}
