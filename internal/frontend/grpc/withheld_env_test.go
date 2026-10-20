package grpc

import (
	"slices"
	"testing"
)

func TestWithheldCapabilitiesForTesting(t *testing.T) {
	t.Setenv(TestWithholdCapabilitiesEnv, " query, strict_conformance ,,")
	got := WithheldCapabilitiesForTesting()
	want := []string{"query", "strict_conformance"}
	if !slices.Equal(got, want) {
		t.Errorf("capabilities = %v, want %v", got, want)
	}
	t.Setenv(TestWithholdCapabilitiesEnv, "")
	if got := WithheldCapabilitiesForTesting(); got != nil {
		t.Errorf("capabilities = %v, want none", got)
	}
}
