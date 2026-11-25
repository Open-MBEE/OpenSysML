package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const oslcShadowedFeaturesModel = `package Demo {
	private import ScalarValues::*;
	part shadow {
		attribute isActive : Boolean = true;
		attribute annotations : String = "mine";
	}
}`

// OSLC filters and projections over a model feature named like a built-in read
// that feature; the "@" spellings still reach the evaluator's own properties.
func TestOSLCQueriesReadFeaturesShadowingBuiltinNames(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: oslcShadowedFeaturesModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, tc := range []struct{ where, property, value string }{
		{`oslc.where=sysx:isActive=true`, QueryPropIsActive, "true"},
		{`oslc.where=sysx:annotations="mine"`, QueryPropAnnotations, "mine"},
	} {
		t.Run(tc.property, func(t *testing.T) {
			resp := oslcQuery(t, srv, parsed.ModelHash,
				tc.where+`&oslc.select=sysx:`+tc.property)
			if len(resp.Elements) != 1 || resp.Elements[0].Id != "Demo::shadow" {
				t.Fatalf("rows = %v, want Demo::shadow alone", resp.Elements)
			}
			if got := resp.Elements[0].Properties[tc.property]; got != tc.value {
				t.Fatalf("%s = %q, want %q", tc.property, got, tc.value)
			}
			aliased := oslcQuery(t, srv, parsed.ModelHash,
				`oslc.where=sysml:name="shadow"&oslc.select=sysx:@`+tc.property)
			if len(aliased.Elements) != 1 {
				t.Fatalf("alias rows = %v, want Demo::shadow alone", aliased.Elements)
			}
			if _, found := aliased.Elements[0].Properties["@"+tc.property]; found {
				t.Fatalf("sysx:@%s read the declared feature on a part = %q",
					tc.property, aliased.Elements[0].Properties["@"+tc.property])
			}
		})
	}
	// One request can filter on the feature and project the evaluator's own
	// property under the unambiguous spelling.
	resp := oslcQuery(t, srv, parsed.ModelHash,
		`oslc.where=sysx:isActive=true&oslc.select=sysx:@isActive`)
	if len(resp.Elements) != 1 || resp.Elements[0].Id != "Demo::shadow" {
		t.Fatalf("rows = %v, want Demo::shadow alone", resp.Elements)
	}
	if _, found := resp.Elements[0].Properties["@isActive"]; found {
		t.Fatalf("sysx:@isActive read the declared feature on a part = %q",
			resp.Elements[0].Properties["@isActive"])
	}
}
