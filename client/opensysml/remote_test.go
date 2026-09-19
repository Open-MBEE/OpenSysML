package opensysml_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// startService serves the Connect transport in this test process, so the
// remote implementation is tested without a child process.
func startService(t *testing.T) string {
	t.Helper()
	svc, err := sysmlgrpc.NewService(16, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

func dialClient(t *testing.T, address string, opts ...opensysml.DialOption) opensysml.Client {
	t.Helper()
	client, err := opensysml.Dial(address, opts...)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestDialNeedsAnAddress(t *testing.T) {
	if _, err := opensysml.Dial(""); err == nil {
		t.Error("Dial(\"\") did not fail")
	}
}

func TestRemoteAnswersLikeInProcess(t *testing.T) {
	client := dialClient(t, startService(t))
	model, err := client.ParseSource(context.Background(), vehicleSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	value, err := client.Evaluate(context.Background(), model, "2 + 2")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got, ok := value.(opensysml.Int); !ok || got != 4 {
		t.Errorf("value = %#v, want Int(4)", value)
	}
}

func TestRemoteJSONBodyAnswersTheSame(t *testing.T) {
	client := dialClient(t, startService(t), opensysml.WithJSONBody())
	model, err := client.ParseSource(context.Background(), vehicleSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	if model.Hash == "" || model.Root == nil {
		t.Errorf("model = %+v, want a hash and a root", model)
	}
}

func TestARemoteRefusalKeepsItsCode(t *testing.T) {
	client := dialClient(t, startService(t))
	_, err := client.Evaluate(context.Background(), &opensysml.Model{Hash: "no-such-model"}, "2 + 2")
	if !errors.Is(err, opensysml.CodeNotFound) {
		t.Errorf("err = %v, want CodeNotFound", err)
	}
}

func TestAnUnreachableServiceIsUnavailable(t *testing.T) {
	client := dialClient(t, "127.0.0.1:1")
	_, err := client.ServerInfo(context.Background())
	if !errors.Is(err, opensysml.CodeUnavailable) {
		t.Errorf("err = %v, want CodeUnavailable", err)
	}
}
