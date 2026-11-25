package grpc

import (
	"os"
	"path/filepath"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// bundledScalarValues is a bundled library file submitted as a document: a
// workspace stands it in, while a fresh parse indexes the copy as ordinary input.
func bundledScalarValues(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "workspace", "libs", "stdlib",
		"Kernel Libraries", "Kernel Data Type Library", "ScalarValues.kerml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// An edit to a set holding a library stand-in still answers from the records,
// without priming the warm lineage with the fresh path's analysis stamps.
func TestPersistentParseCacheRestoresAnEditedLibraryStandIn(t *testing.T) {
	app := "package App { private import ScalarValues::*; part p : Real; }\n"
	request := func(second string) *pb.ParseSourcesRequest {
		documents := inlineDocuments("copy.kerml", bundledScalarValues(t), "app.sysml", second)
		documents[0].Language = "kerml"
		return &pb.ParseSourcesRequest{Documents: documents}
	}
	want := answersOf(t, mustNewService(t, 10), request(app+"// edited\n"))

	cache, _ := recordCacheIn(t)
	srv := recordingService(t, cache)
	answersOf(t, srv, request(app))
	got := answersOf(t, srv, request(app+"// edited\n"))
	requireSameAnswers(t, "edited library stand-in", got, want)
}
