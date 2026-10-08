package opensysml_test

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

const bigIntCapabilitySource = `package B {
	private import ScalarValues::*;
	attribute wide = 2 ** 70;
	attribute narrow = 2 ** 62;
	calc def Same { in x; return : Anything = x; }
}
`

// A service without big_int_values would read an Integer beyond int64 as null,
// so the client refuses to send one, bare or nested; and what such a service
// reports for one is an unsupported null naming it.
func TestBigIntInputsNeedTheirCapability(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityBigIntValues})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	wide := opensysml.NewInteger(new(big.Int).Lsh(big.NewInt(1), 70))
	quantity := opensysml.Quantity{Magnitude: wide, Unit: "m"}
	client := dialClient(t, server.URL)
	ctx := context.Background()
	model := parse(t, client, bigIntCapabilitySource)
	for label, input := range map[string]opensysml.Value{
		"bare":            wide,
		"nested":          opensysml.Sequence{opensysml.Int(1), opensysml.Sequence{wide}},
		"in a set":        opensysml.Set{wide},
		"quantity":        quantity,
		"vector":          opensysml.Vector{opensysml.Int(1), wide},
		"vector quantity": opensysml.VectorQuantity{quantity},
		"tensor quantity": opensysml.TensorQuantity{Dimensions: []int64{1}, Components: []opensysml.Quantity{quantity}},
	} {
		_, err := client.EvaluateCalc(ctx, model, "B::Same", input)
		wantCapabilityRefusal(t, "EvaluateCalc "+label, err, opensysml.CapabilityBigIntValues)
	}
	if got, err := client.EvaluateCalc(ctx, model, "B::Same", opensysml.Int(1<<62)); err != nil || got.Result != opensysml.Int(1<<62) {
		t.Errorf("EvaluateCalc(2**62) = %#v, %v; want Int 2**62", got, err)
	}

	got, err := client.Evaluate(ctx, model, "B::wide")
	if err != nil {
		t.Fatalf("Evaluate(B::wide): %v", err)
	}
	if want := opensysml.Null("unsupported: constant 1180591620717411303424"); got != want {
		t.Errorf("B::wide without the capability = %#v, want %#v", got, want)
	}
	if got, err := client.Evaluate(ctx, model, "B::narrow"); err != nil || got != opensysml.Int(1<<62) {
		t.Errorf("B::narrow without the capability = %#v, %v; want Int 2**62", got, err)
	}
}

// A document query bound to an Integer beyond int64, bare or as a quantity
// magnitude, is refused before it is sent to a service without big_int_values.
func TestBigIntDocumentBindingsNeedTheirCapability(t *testing.T) {
	svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", []string{opensysml.CapabilityBigIntValues})
	if err != nil {
		t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	wide := opensysml.NewInteger(new(big.Int).Lsh(big.NewInt(1), 70)).(opensysml.BigInt)
	client := dialClient(t, server.URL)
	ctx := context.Background()
	model := parse(t, client, bigIntCapabilitySource)
	for label, bound := range map[string]opensysml.Cell{
		"bare":     wide,
		"quantity": opensysml.Quantity{Magnitude: wide, Unit: "m"},
	} {
		_, err := client.RunDocumentQuery(ctx, model, "B::NoSuchQuery", opensysml.Bind("x", bound))
		wantCapabilityRefusal(t, "RunDocumentQuery "+label, err, opensysml.CapabilityBigIntValues)
	}
}
