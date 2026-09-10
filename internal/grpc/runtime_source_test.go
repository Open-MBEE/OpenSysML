package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Every worker's semantic model reads documentation through the cached model's own
// source lookup, so a request's runtime answers the same as the next request's.
func TestRuntimeWorkersReadTheCachedSourceLookup(t *testing.T) {
	const model = `
package Demo {
	requirement def <'R1'> Safe {
		doc /* The crew shall return safely. */
	}
}
`
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: model},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	cached, ok := srv.cache.Get(parsed.ModelHash)
	if !ok {
		t.Fatal("parsed model not cached")
	}
	syms := cached.Index.LookupQualified("Demo::Safe")
	if len(syms) != 1 {
		t.Fatalf("Demo::Safe resolved to %d symbols, want 1", len(syms))
	}

	var last *runtime.Context
	for i := 0; i < 3; i++ {
		ctx := srv.newRuntime(cached)
		if last != nil && (ctx.Model() == last.Model() || ctx.Resolver() == last.Resolver()) {
			t.Fatalf("runtime %d: shares its resolver or semantic model with the one before", i)
		}
		if ctx.Model().SourceText() == nil {
			t.Fatalf("runtime %d: worker's model has no source lookup", i)
		}
		if got := ctx.Model().DocumentationOf(syms[0]); len(got) != 1 || got[0] != "The crew shall return safely." {
			t.Fatalf("runtime %d: DocumentationOf = %q, want the doc body", i, got)
		}
		last = ctx
	}
}
