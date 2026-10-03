package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestGRPCRobustnessLazyRequired instantiates an object holding a billion required values:
// the response lists the values each feature holds within the element budget and reports
// the budget on a feature reaching every one, rather than listing them.
func TestGRPCRobustnessLazyRequired(t *testing.T) {
	service := mustNewService(t, 10)
	hash := mustParsedModel(t, service, `package test {
    private import ScalarValues::*;
    part def C { attribute m : Real default := 2.0; }
    part def Holder {
        part p : C[5000];
        part big : C[1000000000];
    }
    part h : Holder;
}`)
	resp, err := service.Instantiate(context.Background(), &pb.InstantiateRequest{ModelHash: hash, SymbolId: "test::h"})
	if err != nil || resp.Error != "" {
		t.Fatalf("Instantiate: %v %s", err, resp.GetError())
	}
	features := resp.Instance.FeatureValues
	if n := len(features["p"].GetValues()); n != 5000 {
		t.Errorf("p lists %d values, want 5000", n)
	}
	if got := features["big"].GetError(); !strings.Contains(got, "collection element limit exceeded") {
		t.Errorf("big error = %q, want the element budget", got)
	}
}
