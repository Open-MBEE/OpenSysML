package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// anonymousAssertionSource states assertions no name identifies, both directly
// and inside an element that has no name of its own.
const anonymousAssertionSource = `package TeamCases {
	part def Truck {
		attribute payload = 900.0;
	}
	requirement def PayloadLimit {
		subject truck : Truck;
		require constraint { truck.payload <= 1000.0 }
	}
	requirement payloadHolds : PayloadLimit;
	part loadedTruck : Truck;
	part {
		assert satisfy payloadHolds by loadedTruck;
	}
	verification def CheckPayload {
		subject truck : Truck;
		assert satisfy payloadHolds by loadedTruck;
	}
}
`

// TestAnonymousVerdictsCarryNoElementID: an id is a symbol a caller can look up,
// so an element with no name reports none rather than a qualified name with
// empty segments.
func TestAnonymousVerdictsCarryNoElementID(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, anonymousAssertionSource, "verify-anonymous")

	resp, err := srv.VerifySatisfaction(context.Background(), &pb.VerifySatisfactionRequest{
		ModelHash: hash,
	})
	if err != nil {
		t.Fatalf("VerifySatisfaction: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifySatisfaction reported %q", resp.Error)
	}
	if len(resp.Verdicts) == 0 {
		t.Fatal("no verdicts, so the anonymous assertions were not evaluated")
	}

	for i, verdict := range resp.Verdicts {
		if verdict.Element == "" {
			t.Errorf("verdict %d names no assertion", i)
		}
		for _, id := range []struct {
			what  string
			value string
		}{
			{"element_id", verdict.ElementId},
			{"instance_type_id", verdict.InstanceTypeId},
		} {
			if id.value == "" {
				continue
			}
			if strings.HasSuffix(id.value, "::") || strings.Contains(id.value, "::::") {
				t.Errorf("verdict %d: %s = %q, want no empty name segments", i, id.what, id.value)
				continue
			}
			lookup, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
				ModelHash: hash, SymbolId: id.value,
			})
			if err != nil {
				t.Errorf("verdict %d: GetSymbol(%q): %v", i, id.value, err)
				continue
			}
			if lookup.Error != "" {
				t.Errorf("verdict %d: %s = %q is not a symbol: %s", i, id.what, id.value, lookup.Error)
			}
		}
	}
}
