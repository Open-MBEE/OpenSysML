//go:build !sysml_re && !sysml_nofmi

package opensysml_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
)

// Every engine is listed with its origin: a built-in one as such, a manifest
// tool and a manifest engine with the entry they were registered from.
func TestListEnginesReportsEachEnginesOrigin(t *testing.T) {
	command, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	dir := t.TempDir()
	toolFile := filepath.Join(dir, "solver.json")
	engineFile := filepath.Join(dir, "standin.json")
	entries := map[string]string{
		toolFile: `{"kind":"tool","toolName":"Solver","version":"1.0","executable":"` + command + `",` +
			`"variables":["x","y"],"invocation":{"args":["--x","{x}"],"stdin":"none"},` +
			`"reply":{"format":"csv","outputs":{"y":{"column":"y"}}}}`,
		engineFile: `{"kind":"engine","name":"standin","version":"1.0.0","command":["` + command + `"],` +
			`"protocol":1,"answers":["holds"],"model":["sources"],"witness":"schedule","authority":"bounded"}`,
	}
	for file, entry := range entries {
		if err := os.WriteFile(file, []byte(entry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(analysis.ToolsEnv, dir)
	t.Setenv(analysis.EnginesEnv, "")

	clients := map[string]opensysml.Client{
		"in-process": newClient(t),
		"remote":     dialClient(t, startService(t)),
	}
	for name, client := range clients {
		t.Run(name, func(t *testing.T) {
			engines, err := client.ListEngines(context.Background())
			if err != nil {
				t.Fatalf("ListEngines: %v", err)
			}
			seen := map[string]bool{}
			for _, engine := range engines {
				seen[engine.Name] = true
				origin := opensysml.EngineInfo{
					Name: engine.Name, Kind: engine.Kind, Protocol: engine.Protocol, Source: engine.Source,
					Command: engine.Command, Version: engine.Version, Served: engine.Served,
				}
				want := opensysml.EngineInfo{Name: engine.Name, Kind: "built-in", Protocol: "-", Served: true}
				switch engine.Name {
				case "tool:fmi":
					want.Kind, want.Protocol = "tool", "fmi/1"
				case "tool:Solver":
					want = opensysml.EngineInfo{
						Name: engine.Name, Kind: "tool", Protocol: "argv+none/csv", Source: toolFile,
						Command: command, Version: "1.0", Served: true,
					}
				case "standin":
					want = opensysml.EngineInfo{
						Name: engine.Name, Kind: "engine", Protocol: "stdio/1", Source: engineFile,
						Command: command, Version: "1.0.0",
					}
					if engine.Ready || engine.Unavailable != "" || engine.Authority != "bounded" || !slices.Equal(engine.Answers, []string{"holds"}) {
						t.Errorf("standin = %+v, want an unserved engine answering holds at bounded, not ready with no fault", engine)
					}
				}
				if !reflect.DeepEqual(origin, want) {
					t.Errorf("%s origin = %+v, want %+v", engine.Name, origin, want)
				}
			}
			for _, want := range []string{"run", "tool:fmi", "tool:Solver", "standin"} {
				if !seen[want] {
					t.Errorf("engines = %v, want %s among them", engineNames(engines), want)
				}
			}
		})
	}
}
