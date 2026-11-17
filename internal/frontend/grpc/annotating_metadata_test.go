package grpc

import (
	"context"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const annotatedModel = `
package Demo {
    metadata def FailureMode { attribute severity : ScalarValues::Integer; }
    part def Pump;
    metadata pump_fm : FailureMode about Pump { severity = 2; }
}
`

func annotatingMetadataOfPump(t *testing.T, srv *Service) []string {
	t.Helper()
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: annotatedModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
		ModelHash: parsed.ModelHash, SymbolId: "Demo::Pump",
	})
	if err != nil || resp.Symbol == nil {
		t.Fatalf("GetSymbol: %v %s", err, resp.GetError())
	}
	return resp.Symbol.AnnotatingMetadataIds
}

// TestAnnotatingMetadataNamesAnAboutUsage verifies GetSymbol names the usage
// annotating an element from elsewhere.
func TestAnnotatingMetadataNamesAnAboutUsage(t *testing.T) {
	srv, err := NewService(10, "test")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := annotatingMetadataOfPump(t, srv), []string{"Demo::pump_fm"}; !slices.Equal(got, want) {
		t.Errorf("annotating_metadata_ids = %v, want %v", got, want)
	}
}

// TestAnnotatingMetadataWithheldWithoutCapability verifies a build without
// symbol_annotating_metadata names no usage rather than some of them.
func TestAnnotatingMetadataWithheldWithoutCapability(t *testing.T) {
	srv, err := NewServiceWithUnavailableCapabilitiesForTesting(10, "test", []string{CapabilitySymbolAnnotatingMetadata})
	if err != nil {
		t.Fatal(err)
	}
	if got := annotatingMetadataOfPump(t, srv); len(got) != 0 {
		t.Errorf("annotating_metadata_ids = %v, want none without the capability", got)
	}
}
