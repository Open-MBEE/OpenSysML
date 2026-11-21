package opensysml_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
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

// startGuardedService is startService behind the bearer-token door a
// sysml-grpc serving other machines has: a call without the token is refused.
func startGuardedService(t *testing.T, token string) string {
	t.Helper()
	svc, err := sysmlgrpc.NewService(16, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)
	door := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if req.Header().Get("Authorization") != "Bearer "+token {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("a bearer token is required"))
			}
			return next(ctx, req)
		}
	})
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc), connect.WithInterceptors(door)))
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

func TestAGuardedServiceAdmitsTheRightTokenAndRefusesTheRest(t *testing.T) {
	const token = "remote-token_1"
	address := startGuardedService(t, token)
	for _, test := range []struct {
		name string
		opts []opensysml.DialOption
		ok   bool
	}{
		{"no token", nil, false},
		{"wrong token", []opensysml.DialOption{opensysml.WithToken("nope")}, false},
		{"right token", []opensysml.DialOption{opensysml.WithToken(token)}, true},
		{"right token as JSON", []opensysml.DialOption{opensysml.WithToken(token), opensysml.WithJSONBody()}, true},
	} {
		_, err := dialClient(t, address, test.opts...).ServerInfo(context.Background())
		if test.ok && err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
		if !test.ok && !errors.Is(err, opensysml.CodeUnauthenticated) {
			t.Errorf("%s: %v, want UNAUTHENTICATED", test.name, err)
		}
	}
	t.Setenv(opensysml.TokenEnv, token)
	if _, err := dialClient(t, address).ServerInfo(context.Background()); err != nil {
		t.Errorf("the token from $%s was refused: %v", opensysml.TokenEnv, err)
	}
	if _, err := dialClient(t, address, opensysml.WithToken("nope")).ServerInfo(context.Background()); !errors.Is(err, opensysml.CodeUnauthenticated) {
		t.Errorf("WithToken did not win over $%s: %v", opensysml.TokenEnv, err)
	}
}

// TestDialRefusesATokenInClear checks a bearer token is not sent over plain http
// to an address other machines reach: Dial refuses it, from the option and from
// the environment alike, and accepts https, this machine, and the explicit opt-in.
func TestDialRefusesATokenInClear(t *testing.T) {
	for _, address := range []string{"sysml.example.com:50051", "http://sysml.example.com", "http://10.0.0.7:50051", "HTTP://[2001:db8::1]:50051"} {
		_, err := opensysml.Dial(address, opensysml.WithToken("s3cret"))
		if !errors.Is(err, opensysml.CodeInvalidArgument) {
			t.Errorf("Dial(%q) with a token: err = %v, want CodeInvalidArgument", address, err)
		} else if strings.Contains(err.Error(), "s3cret") {
			t.Errorf("Dial(%q): the refusal quotes the token: %v", address, err)
		}
		if client, err := opensysml.Dial(address, opensysml.WithToken("s3cret"), opensysml.WithInsecureTokenTransport()); err != nil {
			t.Errorf("Dial(%q) with the opt-in: %v", address, err)
		} else {
			_ = client.Close()
		}
		if client, err := opensysml.Dial(address); err != nil {
			t.Errorf("Dial(%q) without a token: %v", address, err)
		} else {
			_ = client.Close()
		}
	}
	for _, address := range []string{"https://sysml.example.com", "HTTPS://sysml.example.com", "localhost:50051", "http://127.0.0.1:50051", "http://[::1]:50051"} {
		if client, err := opensysml.Dial(address, opensysml.WithToken("s3cret")); err != nil {
			t.Errorf("Dial(%q) with a token: %v", address, err)
		} else {
			_ = client.Close()
		}
	}
	t.Setenv(opensysml.TokenEnv, "s3cret")
	if _, err := opensysml.Dial("http://sysml.example.com"); !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("Dial with $%s set: err = %v, want CodeInvalidArgument", opensysml.TokenEnv, err)
	}
}

// TestTheTokenNeverLeavesForARefusedAddress checks the refusal is made before any
// request: a plain-http server other machines reach sees no Authorization header.
func TestTheTokenNeverLeavesForARefusedAddress(t *testing.T) {
	seen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case seen <- r.Header.Get("Authorization"):
		default:
		}
	}))
	t.Cleanup(server.Close)
	// The server listens on loopback; a client that resolves a foreign name to it
	// stands for a remote plain-http service.
	via := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}}
	if _, err := opensysml.Dial("http://sysml.example.com", opensysml.WithToken("s3cret"), opensysml.WithHTTPClient(via)); err == nil {
		t.Fatal("Dial accepted a token over plain http")
	}
	client := dialClient(t, "http://sysml.example.com", opensysml.WithToken("s3cret"), opensysml.WithHTTPClient(via), opensysml.WithInsecureTokenTransport())
	_, _ = client.ServerInfo(context.Background())
	select {
	case got := <-seen:
		if got != "Bearer s3cret" {
			t.Errorf("with the opt-in the server saw Authorization %q", got)
		}
	default:
		t.Error("with the opt-in no request reached the server")
	}
}

func TestAResponsePastTheBoundIsResourceExhausted(t *testing.T) {
	address := startService(t)
	if _, err := dialClient(t, address).ParseSource(context.Background(), vehicleSource); err != nil {
		t.Fatalf("ParseSource under the default bound: %v", err)
	}
	_, err := dialClient(t, address, opensysml.WithMaxResponseBytes(8)).ParseSource(context.Background(), vehicleSource)
	if !errors.Is(err, opensysml.CodeResourceExhausted) {
		t.Errorf("err = %v, want CodeResourceExhausted", err)
	}
}
