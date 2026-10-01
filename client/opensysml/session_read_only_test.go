package opensysml_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const readOnlySessionSource = `package Shop {
	private import ScalarValues::*;
	part def Till {
		constant attribute float : Integer default 50;
		attribute takings : Integer default 0;
		derived attribute total : Integer = float + takings;
	}
	part till : Till;
}`

func TestSessionSetFeatureRefusesReadOnlyFeatures(t *testing.T) {
	client := newClient(t)
	session, err := opensysml.OpenSession(client, parse(t, client, readOnlySessionSource))
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	till, err := session.Instantiate("Shop::till")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for _, tt := range []struct{ name, kind string }{{"float", "constant"}, {"total", "derived"}} {
		err := session.SetFeature(till, tt.name, opensysml.Int(7))
		if err == nil || !strings.Contains(err.Error(), "feature is read-only") ||
			!strings.Contains(err.Error(), tt.name+" is "+tt.kind) {
			t.Fatalf("SetFeature %s = %v, want a read-only refusal naming it %s", tt.name, err, tt.kind)
		}
	}
	if got, err := session.Feature(till, "float"); err != nil || got.Value != opensysml.Int(50) {
		t.Fatalf("float after the refused write = %#v, %v; want Int(50)", got, err)
	}
	if err := session.SetFeature(till, "takings", opensysml.Int(5)); err != nil {
		t.Fatalf("SetFeature takings: %v", err)
	}
	if got, err := session.Feature(till, "total"); err != nil || got.Value != opensysml.Int(55) {
		t.Fatalf("total after takings := 5 = %#v, %v; want Int(55)", got, err)
	}
}
