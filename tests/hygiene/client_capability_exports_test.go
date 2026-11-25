package hygiene

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"testing"
)

var (
	nodeCapabilityDecl   = regexp.MustCompile(`export const (CAPABILITY_\w+)\s*=`)
	nodeCapabilityBarrel = regexp.MustCompile(`export\s*\{([^}]*)\}\s*from\s*"\./capabilities\.js"`)
	identifier           = regexp.MustCompile(`\b(CAPABILITY_\w+)\b`)
	juliaCapabilityDecl  = regexp.MustCompile(`\bconst (CAPABILITY_\w+)\s*=`)
	juliaCapabilityExp   = regexp.MustCompile(`:(CAPABILITY_\w+)`)
)

func namesIn(t *testing.T, source string, pattern *regexp.Regexp, group int) []string {
	t.Helper()
	found := map[string]bool{}
	for _, match := range pattern.FindAllStringSubmatch(source, -1) {
		found[match[group]] = true
	}
	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func readHygieneFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, path))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return string(data)
}

// TestClientCapabilityExports holds each client's public capability re-exports
// to the capabilities its module declares, so a new one cannot be left private.
func TestClientCapabilityExports(t *testing.T) {
	nodeDeclared := namesIn(t, readHygieneFile(t, "client/node/src/core/capabilities.ts"), nodeCapabilityDecl, 1)
	barrel := nodeCapabilityBarrel.FindString(readHygieneFile(t, "client/node/src/core/index.ts"))
	if barrel == "" {
		t.Fatal("client/node/src/core/index.ts has no capabilities re-export block")
	}
	nodeExported := namesIn(t, barrel, identifier, 1)
	for _, name := range nodeDeclared {
		if !slices.Contains(nodeExported, name) {
			t.Errorf("client/node/src/core/index.ts does not re-export %s", name)
		}
	}

	juliaDeclared := namesIn(t, readHygieneFile(t, "client/julia/OpenSysML/src/capabilities.jl"), juliaCapabilityDecl, 1)
	juliaExported := namesIn(t, readHygieneFile(t, "client/julia/OpenSysML/src/OpenSysML.jl"), juliaCapabilityExp, 1)
	for _, name := range juliaDeclared {
		if !slices.Contains(juliaExported, name) {
			t.Errorf("client/julia/OpenSysML/src/OpenSysML.jl does not export %s", name)
		}
	}
}
