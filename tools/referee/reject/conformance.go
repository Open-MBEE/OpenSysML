package reject

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// The conformance policies the harness can be run under. "auto" asks each case
// the question its derivation is about; the other two ask one question of all of
// them, which is how the default-mode baseline stays reproducible.
const (
	policyAuto    = "auto"
	policyDefault = "default"
	policyStrict  = "strict"
)

// strictSource is the corpus derivation holding OpenSysML notation extensions:
// notation the pinned reference rejects and we accept by default, so only the
// strict question is a fair comparison for it.
const strictSource = "extensions"

// parsePolicy validates a -conformance value.
func parsePolicy(name string) (string, error) {
	switch name {
	case policyAuto, policyDefault, policyStrict:
		return name, nil
	default:
		return "", fmt.Errorf("unknown conformance policy %q: want %q, %q or %q",
			name, policyAuto, policyDefault, policyStrict)
	}
}

// modeFor is the conformance mode a case of the named derivation is judged under.
func modeFor(policy, src string) diag.ConformanceMode {
	switch policy {
	case policyStrict:
		return diag.ConformanceStrict
	case policyDefault:
		return diag.ConformanceDefault
	default:
		return diag.ConformanceModeOf(src == strictSource)
	}
}
