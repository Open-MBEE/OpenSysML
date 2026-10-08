// SPDX-License-Identifier: Apache-2.0

package corpus

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Qualifying the ambiguous imports must clear their downstream diagnostics too.
// Edits are made only in memory; the pinned corpus remains unchanged.
func TestPilotImportedNameClashQualification(t *testing.T) {
	const vehicle = "SimpleVehicleModel::VehicleConfigurations::VehicleConfiguration_b::PartsTree::vehicle_b"
	controls := map[string]struct {
		primary int
		names   map[string]string
	}{
		"kerml-examples/Simple Tests/Imports.kerml": {3, map[string]string{
			"A": "Imports::P::A", "D": "Imports::Q::D",
		}},
		"sysml-examples/Vehicle Example/Annex_A_VehicleViews.sysml":                     {6, map[string]string{"vehicle_b": vehicle}},
		"sysml-examples/Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml": {9, map[string]string{"vehicle_b": vehicle}},
		"sysml-validation/13-Model Containment/13a-Model Containment.sysml": {3, map[string]string{
			"vehicle1_c1":  "'8-Requirements'::'Vehicle Usages'::vehicle1_c1",
			"Engine":       "'8-Requirements'::'Vehicle Definitions'::Engine",
			"Transmission": "'8-Requirements'::'Vehicle Definitions'::Transmission",
		}},
	}
	files := pilotCorporaGate.files(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checked := 0
	for _, root := range pilotCorporaGate.roots {
		for _, batch := range languageBatches(files[root.name]) {
			ws := model.NewWorkspace()
			contents := make(map[string]string, len(batch))
			for _, rel := range batch {
				content, err := os.ReadFile(filepath.Join(root.dir, rel))
				if err != nil {
					t.Fatal(err)
				}
				contents[rel] = string(content)
				ws.Open(rel, content, 1)
			}
			var changed []string
			for _, rel := range batch {
				control, ok := controls[pilotCorporaGate.key(root, rel)]
				if !ok {
					continue
				}
				checked++
				content := contents[rel]
				diagnostics := ws.Diagnostics(rel)
				sort.Slice(diagnostics, func(i, j int) bool { return diagnostics[i].Span.Offset > diagnostics[j].Span.Offset })
				edits := 0
				for _, d := range diagnostics {
					name := contents[rel][d.Span.Offset:d.Span.End()]
					replacement, ok := control.names[name]
					if !ok || !strings.HasPrefix(d.Message, "unresolved reference:") {
						continue
					}
					content = content[:d.Span.Offset] + replacement + content[d.Span.End():]
					edits++
				}
				if edits != control.primary {
					t.Fatalf("%s: qualified %d ambiguous references, want %d", rel, edits, control.primary)
				}
				contents[rel] = content
				changed = append(changed, rel)
			}
			for _, rel := range changed {
				ws.Open(rel, []byte(contents[rel]), 2)
			}
			for _, rel := range changed {
				if diagnostics := ws.Diagnostics(rel); len(diagnostics) != 0 {
					t.Errorf("%s: qualification must clear all diagnostics: %v", rel, diagnostics)
				}
			}
		}
	}
	if checked != len(controls) {
		t.Fatalf("checked %d corpus files, want %d", checked, len(controls))
	}
}
