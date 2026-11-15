package opensysml_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// A service without symbol_name_resolution reads every id as one it reported, so the client
// asks it that way: an alias answers as itself, where a service with the capability names its element.
func TestLookupSymbolNeedsNameResolutionCapabilityToResolveAName(t *testing.T) {
	const source = `package Vis { package Lib { part def Pump; alias PumpAlias for Pump; } }`
	for label, tc := range map[string]struct {
		withheld []string
		want     string
	}{
		"advertised": {nil, "Vis::Lib::Pump"},
		"withheld":   {[]string{opensysml.CapabilitySymbolNameResolution}, "Vis::Lib::PumpAlias"},
	} {
		t.Run(label, func(t *testing.T) {
			svc, err := sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(16, "test", tc.withheld)
			if err != nil {
				t.Fatalf("NewServiceWithUnavailableCapabilitiesForTesting: %v", err)
			}
			t.Cleanup(svc.Close)
			mux := http.NewServeMux()
			mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc)))
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)

			client := dialClient(t, server.URL)
			symbol, err := client.LookupSymbol(context.Background(), parse(t, client, source), "Vis::Lib::PumpAlias")
			if err != nil {
				t.Fatalf("LookupSymbol: %v", err)
			}
			if symbol.ID != tc.want {
				t.Errorf("LookupSymbol(Vis::Lib::PumpAlias) = %s, want %s", symbol.ID, tc.want)
			}
		})
	}
}
