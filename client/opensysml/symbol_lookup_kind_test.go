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

const lookupKindSource = `package Col__V__c_pend0 { part marker; }
package Col {
    port def P;
    part def V { port q : P; port r : P; connection c connect q to r; }
}`

// A name and an id of one spelling name two elements, told apart by the stated kind; a service
// without symbol_lookup_kind is asked in the legacy form, where the declaration shadows the id.
func TestLookupSymbolStatesItsLookupKind(t *testing.T) {
	for label, tc := range map[string]struct {
		withheld     []string
		byName, byID string
		readByName   string
	}{
		"advertised": {nil, "package", "connectorEnd", "Package"},
		"withheld":   {[]string{opensysml.CapabilitySymbolLookupKind}, "package", "package", "ReferenceUsage"},
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

			ctx := context.Background()
			client := dialClient(t, server.URL)
			model := parse(t, client, lookupKindSource)
			for _, lookup := range []struct {
				option opensysml.SymbolOption
				want   string
			}{
				{opensysml.ByQualifiedName(), tc.byName},
				{opensysml.ByElementID(), tc.byID},
			} {
				symbol, err := client.LookupSymbol(ctx, model, "Col__V__c_pend0", lookup.option)
				if err != nil {
					t.Fatalf("LookupSymbol: %v", err)
				}
				if symbol.Kind != lookup.want {
					t.Errorf("LookupSymbol(Col__V__c_pend0) = %s %s, want a %s", symbol.Kind, symbol.ID, lookup.want)
				}
			}
			legacy, err := client.LookupSymbol(ctx, model, "Col__V__c_pend0")
			if err != nil || legacy.Kind != "package" {
				t.Errorf("legacy LookupSymbol(Col__V__c_pend0) = %v, %v, want the package", legacy, err)
			}

			named, err := client.ElementPropertiesByName(ctx, model, "Col__V__c_pend0", "name")
			if err != nil || named.Element.Metaclass != tc.readByName {
				t.Errorf("ElementPropertiesByName = %v, %v, want a %s", named, err, tc.readByName)
			}
			read, err := client.ElementPropertiesByID(ctx, model, "Col__V__c_pend0", "name")
			if err != nil || read.Element.Metaclass != "ReferenceUsage" {
				t.Errorf("ElementPropertiesByID = %v, %v, want the end's ReferenceUsage", read, err)
			}
		})
	}
}
