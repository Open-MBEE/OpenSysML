package grpc_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

func TestOSLCQueryMatchesREPLQuery(t *testing.T) {
	srv := grpc.MustNewServiceForTest(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: grpc.QueryModelForTest},
	})
	if err != nil {
		t.Fatal(err)
	}
	const text = `oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name,sysml:owner`
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		OslcQuery: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := repl.NewSession()
	if result := session.Submit(grpc.QueryModelForTest); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	lines, err := session.Query(text)
	if err != nil {
		t.Fatal(err)
	}
	// The service reports a property under its query property name; the REPL
	// reports it under the OSLC name the query asked for.
	asked := []struct{ property, spelling string }{
		{grpc.QueryPropName, "sysml:name"},
		{grpc.QueryPropOwner, "sysml:owner"},
	}
	want := make([]string, 0, len(resp.Elements))
	for _, element := range resp.Elements {
		line := fmt.Sprintf("%s  %s", element.Id, element.Type)
		for _, property := range asked {
			if value, ok := element.Properties[property.property]; ok {
				line += fmt.Sprintf("  %s=%s", property.spelling, value)
			}
		}
		want = append(want, line)
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("REPL lines = %v, gRPC lines = %v", lines, want)
	}
}
