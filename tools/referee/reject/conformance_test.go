package reject

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestParsePolicy(t *testing.T) {
	for _, name := range []string{policyAuto, policyDefault, policyStrict} {
		got, err := parsePolicy(name)
		if err != nil || got != name {
			t.Errorf("parsePolicy(%q) = %q, %v", name, got, err)
		}
	}
	if _, err := parsePolicy("lenient"); err == nil {
		t.Error("parsePolicy(\"lenient\") must fail rather than pick a mode for us")
	}
	if _, err := parsePolicy(""); err == nil {
		t.Error("parsePolicy(\"\") must fail: an unset flag is the caller's mistake, not a policy")
	}
}

// Only the extensions/ derivation is judged strictly under auto; the two
// explicit policies ask one question of every derivation.
func TestModeFor(t *testing.T) {
	for _, tc := range []struct {
		policy, source string
		want           diag.ConformanceMode
	}{
		{policyAuto, "extensions", diag.ConformanceStrict},
		{policyAuto, "grammar", diag.ConformanceDefault},
		{policyAuto, "xpect", diag.ConformanceDefault},
		{policyDefault, "extensions", diag.ConformanceDefault},
		{policyStrict, "grammar", diag.ConformanceStrict},
	} {
		if got := modeFor(tc.policy, tc.source); got != tc.want {
			t.Errorf("modeFor(%q, %q) = %v, want %v", tc.policy, tc.source, got, tc.want)
		}
	}
}
