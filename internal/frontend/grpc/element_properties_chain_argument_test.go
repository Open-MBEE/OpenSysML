package grpc

import "testing"

// A feature chain's one argument is its source expression (KerML 8.3.4.8.4):
// the chained feature it instantiates has no inputs to bind arguments against.
func TestGetElementPropertiesAnswersAFeatureChainsArgument(t *testing.T) {
	r := parseForElementProperties(t, `package P {
		private import ScalarValues::*;
		part def L { attribute burnRate : Real; }
		part lander : L;
		attribute rate : Real = lander.burnRate;
	}`)
	resp := r.read("P__rate_pvalue", "argument")
	if resp.Error != "" {
		t.Fatal(resp.Error)
	}
	if len(resp.Properties) != 1 || len(resp.Properties[0].Values) != 1 || resp.Properties[0].Values[0].GetElement() == nil {
		t.Fatalf("argument = %v", resp.Properties)
	}
	if got := resp.Properties[0].Values[0].GetElement().Id; got != "P__rate_pvalue_pa0" {
		t.Errorf("argument = %s, want P__rate_pvalue_pa0", got)
	}
}
