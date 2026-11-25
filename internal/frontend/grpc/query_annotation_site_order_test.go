package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const annotationSiteDeclarations = `	private import DocumentQueries::*;
	private import ScalarValues::*;
	metadata def Outer {
		attribute nested {
			attribute value : String;
		}
	}
	metadata def Tag { attribute label : String; }
`

const firstBinding = "Demo::Tag::label=first"
const secondBinding = "Demo::Outer::nested.value=second"

func annotationQuery() *pb.Query {
	return &pb.Query{
		Scope:  []string{"Demo::a"},
		Select: []string{QueryPropAnnotations},
	}
}

func firstAnnotationBinding(t *testing.T, resp *pb.QueryResponse) string {
	t.Helper()
	rows := rowsByID(t, resp)
	row := rows["Demo::a"]
	if row == nil {
		t.Fatalf("Demo::a absent from %v", resp.Elements)
	}
	return row.Properties[QueryPropAnnotations]
}

func annotationFirstValueInModel(t *testing.T, model string) string {
	t.Helper()
	_, _, resp := runQueryOnSource(t, model, annotationQuery())
	return firstAnnotationBinding(t, resp)
}

func sourceDoc(name, content string) *pb.SourceDocument {
	return &pb.SourceDocument{
		Name:   name,
		Source: &pb.SourceDocument_Content{Content: content},
	}
}

func annotationFirstValueInDocuments(t *testing.T, documents ...*pb.SourceDocument) string {
	t.Helper()
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: documents})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     annotationQuery(),
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return firstAnnotationBinding(t, resp)
}

// An about annotation written before an inline one is the row's first binding,
// and the inline one written first stays first when the order is reversed.
func TestQueryRPCOrdersAnnotationsBySiteInOneDocument(t *testing.T) {
	aboutFirst := "package Demo {\n" + annotationSiteDeclarations +
		`	@Tag about a { label = "first"; }
	part a { @Outer { nested { value = "second"; } } }
}`
	if got := annotationFirstValueInModel(t, aboutFirst); got != firstBinding {
		t.Fatalf("about then inline = %q, want %q", got, firstBinding)
	}
	inlineFirst := "package Demo {\n" + annotationSiteDeclarations +
		`	part a { @Outer { nested { value = "second"; } } }
	@Tag about a { label = "first"; }
}`
	if got := annotationFirstValueInModel(t, inlineFirst); got != secondBinding {
		t.Fatalf("inline then about = %q, want %q", got, secondBinding)
	}
}

// Across documents, the document order decides which site's binding comes
// first. Both sites are padded to the same source offset, so only the document
// can break the tie.
func TestQueryRPCOrdersAnnotationsByDocumentAndSpan(t *testing.T) {
	inlinePrefix := "package Demo {\n" + annotationSiteDeclarations + "\npart a { "
	aboutPrefix := "package Annotations {\n\tprivate import Demo::*;\n"
	inlineDoc := inlinePrefix +
		`@Outer { nested { value = "second"; } } }
}
`
	aboutDoc := aboutPrefix + strings.Repeat(" ", len(inlinePrefix)-len(aboutPrefix)) +
		`@Tag about Demo::a { label = "first"; }
}
`
	if got := annotationFirstValueInDocuments(t,
		sourceDoc("a-about.sysml", aboutDoc), sourceDoc("b-inline.sysml", inlineDoc)); got != firstBinding {
		t.Fatalf("about document then inline document = %q, want %q", got, firstBinding)
	}
	if got := annotationFirstValueInDocuments(t,
		sourceDoc("a-inline.sysml", inlineDoc), sourceDoc("b-about.sysml", aboutDoc)); got != secondBinding {
		t.Fatalf("inline document then about document = %q, want %q", got, secondBinding)
	}
}
