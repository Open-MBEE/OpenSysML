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
		if pattern.MatchString(name) {
			if _, err := os.Stat(filepath.Join(conformanceDir, name+".sysml")); err != nil {
				continue
			}
			cases = append(cases, name)
		}
	}
	sort.Strings(cases)
	known := loadKnownFailures(t, conformanceDir)
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
			if expected.Type != "instance" {
				t.Log("no root instance exists before behavior execution; normal execution is the only applicable routing comparison")
				return
			}
			runConformanceCaseWithOwned(t, conformanceDir, name, DefaultSchedulePolicy, true)
		})
	}
}
