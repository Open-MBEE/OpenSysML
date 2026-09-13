package grpc

import (
	"context"
	"os"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// BenchmarkInstantiateWarmModel measures Instantiate on a model the service already
// holds: the request path a client repeats, over the published vehicle example.
func BenchmarkInstantiateWarmModel(b *testing.B) {
	content, err := os.ReadFile("../../examples/pilot-corpora/sysml-examples/Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml")
	if err != nil {
		b.Skipf("vehicle example absent: %v", err)
	}
	svc, err := NewService(4, "bench")
	if err != nil {
		b.Fatalf("NewService: %v", err)
	}
	defer svc.Close()
	parsed, err := svc.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: string(content)},
	})
	if err != nil {
		b.Fatalf("ParseFile: %v", err)
	}
	req := &pb.InstantiateRequest{ModelHash: parsed.ModelHash, SymbolId: "SimpleVehicleModel::Definitions::PartDefinitions::FuelTank"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := svc.Instantiate(context.Background(), req)
		if err != nil {
			b.Fatalf("Instantiate: %v", err)
		}
		if resp.Error != "" {
			b.Fatalf("Instantiate: %s", resp.Error)
		}
	}
}
