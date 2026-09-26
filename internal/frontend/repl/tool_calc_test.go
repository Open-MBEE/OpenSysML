package repl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

var (
	toolCalcOnce sync.Once
	toolCalcPath string
	toolCalcErr  error
)

// toolCalcBinary builds the tool stand-in of the analysis package's tests once per
// test binary.
func toolCalcBinary(t *testing.T) string {
	t.Helper()
	toolCalcOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolcalc")
		if err != nil {
			toolCalcErr = err
			return
		}
		toolCalcPath = filepath.Join(dir, "toolcalc")
		build := exec.Command("go", gobuild.Args(toolCalcPath)...)
		build.Dir = filepath.Join("..", "..", "exec", "analysis", "testdata", "toolcalc")
		if out, err := build.CombinedOutput(); err != nil {
			toolCalcErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if toolCalcErr != nil {
		t.Fatalf("building the tool stand-in: %v", toolCalcErr)
	}
	return toolCalcPath
}

// toolCalcModel is a calc the prompt computes through the tool its ToolExecution names.
const toolCalcModel = `
package thermo {
	private import ScalarValues::*;
	private import AnalysisTooling::*;
	private import ISQ::*;

	calc def Thermal {
		metadata ToolExecution { toolName = "Thermo"; uri = "thermo://local"; }
		in m : MassValue { @ToolVariable { name = "mass"; } }
		in p : PowerValue { @ToolVariable { name = "power"; } }
		out warn : Boolean { @ToolVariable { name = "warn"; } }
		return : TemperatureValue { @ToolVariable { name = "Tmax"; } }
	}
}
`

// toolCalcManifest writes a manifest naming Thermo and returns its directory.
func toolCalcManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entry := `{"kind":"tool","toolName":"Thermo","version":"1.0.0",` +
		`"executable":` + strconv.Quote(toolCalcBinary(t)) + `,` +
		`"variables":["mass","power","warn","Tmax"]}`
	if err := os.WriteFile(filepath.Join(dir, "thermo.json"), []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A %calc of a tool-computed calc prints the value the tool answers.
func TestToolCalcAtThePrompt(t *testing.T) {
	s := loadSource(t, toolCalcModel)
	t.Setenv(analysis.ToolsEnv, toolCalcManifest(t))
	engines, err := analysis.DefaultFromEnv()
	if err != nil {
		t.Fatalf("DefaultFromEnv: %v", err)
	}
	if err := s.SetEngines(engines); err != nil {
		t.Fatalf("SetEngines: %v", err)
	}

	wants(t, run(t, s, "%calc Thermal(2 [SI::kg], 10 [SI::W])"), "20 [SI::K]")
}

// Without a manifest the same %calc fails as not registered, never evaluating a body.
func TestToolCalcUnregisteredAtThePrompt(t *testing.T) {
	s := loadSource(t, toolCalcModel)
	t.Setenv(analysis.ToolsEnv, t.TempDir())
	engines, err := analysis.DefaultFromEnv()
	if err != nil {
		t.Fatalf("DefaultFromEnv: %v", err)
	}
	if err := s.SetEngines(engines); err != nil {
		t.Fatalf("SetEngines: %v", err)
	}

	wants(t, run(t, s, "%calc Thermal(2 [SI::kg], 10 [SI::W])"),
		"tool 'Thermo' is not registered; set OPENSYSML_TOOLS")
}
