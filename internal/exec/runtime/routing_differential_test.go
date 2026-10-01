package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestRoutingUnchangedByConnectorObjects(t *testing.T) {
	const conformanceDir = "testdata/conformance"
	pattern := regexp.MustCompile(`^(send_|port_|.*_routing|action_port_communication|w7d_send_via_port_to_receiver|binding_)`)
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatal(err)
	}
	var cases []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".expected.json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".expected.json")
		if pattern.MatchString(name) || name == "connector_object_binding_flow" {
			if _, err := os.Stat(filepath.Join(conformanceDir, name+".sysml")); err != nil {
				continue
			}
			cases = append(cases, name)
		}
	}
	sort.Strings(cases)
	known := loadKnownFailures(t, conformanceDir)
	forcedCases := make(map[string]bool)
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			if known[name] {
				t.Skip("known conformance failure")
			}
			data, err := os.ReadFile(filepath.Join(conformanceDir, name+".expected.json"))
			if err != nil {
				t.Fatal(err)
			}
			var expected ExpectedOutcome
			if err := json.Unmarshal(data, &expected); err != nil {
				t.Fatal(err)
			}
			runConformanceCase(t, conformanceDir, name, DefaultSchedulePolicy)
			// Only a case with an object before execution has connectors to force;
			// the others skip inside the harness.
			if expected.Type == "instance" || expected.Type == "state" && len(expected.Performers) > 0 {
				forcedCases[name] = true
			}
			runConformanceCaseWithOwned(t, conformanceDir, name, DefaultSchedulePolicy, true)
		})
	}
	for _, name := range cases {
		if strings.HasPrefix(name, "send_bind_relay_") || name == "connector_object_binding_flow" {
			if !forcedCases[name] {
				t.Fatalf("required routing differential case %q did not run under forceOwned", name)
			}
		}
	}
}
