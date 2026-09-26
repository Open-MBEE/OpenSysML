package main

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

// toolCalcStandin builds the tool stand-in of the analysis package's tests once per
// test binary.
func toolCalcStandin(t *testing.T) string {
	t.Helper()
	toolCalcOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolcalc")
		if err != nil {
			toolCalcErr = err
			return
		}
		toolCalcPath = filepath.Join(dir, "toolcalc")
		build := exec.Command("go", gobuild.Args(toolCalcPath)...)
		build.Dir = filepath.Join("..", "..", "internal", "exec", "analysis", "testdata", "toolcalc")
		if out, err := build.CombinedOutput(); err != nil {
			toolCalcErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if toolCalcErr != nil {
		t.Fatalf("building the tool stand-in: %v", toolCalcErr)
	}
	return toolCalcPath
}

// toolCalcManifest writes a manifest naming Thermo and points OPENSYSML_TOOLS at it.
func toolCalcManifest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	entry := `{"kind":"tool","toolName":"Thermo","version":"1.0.0",` +
		`"executable":` + strconv.Quote(toolCalcStandin(t)) + `,` +
		`"variables":["mass","power","warn","Tmax"]}`
	if err := os.WriteFile(filepath.Join(dir, "thermo.json"), []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(analysis.ToolsEnv, dir)
}

// toolCalcModel is a calc the tool computes; its bodyless result is answered from the reply.
const toolCalcModel = `package thermo {
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

// toolCalcDocument is a document whose table column reads a derived attribute that
// calls the tool calc — the formula the render run evaluates.
const toolCalcDocument = toolCalcModel + `package boards {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import ISQ::*;
	private import thermo::Thermal;

	part def Board {
		attribute mass : MassValue = 2 [SI::kg];
		attribute power : PowerValue = 10 [SI::W];
		attribute Tmax : TemperatureValue = Thermal(mass, power);
	}
	part rack {
		part b1 : Board;
	}

	calc def Boards :> Query {
		in root : Element = rack;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name"),
			columns = (Column(name = "Tmax", expression = Board::Tmax))
		)
	}

	part def BoardReport :> Document {
		attribute redefines title = "Board Thermals";

		part temps : Table {
			attribute redefines caption = "Peak per board";
			calc rows : Boards;
		}
	}
}
`

// sysml -calc answers what the tool answered when the manifest registers it.
func TestCLIToolCalcAnswersFromTheTool(t *testing.T) {
	binary := buildCLI(t)
	toolCalcManifest(t)

	wantReport(t, check(t, binary, toolCalcModel,
		"-calc", "thermo::Thermal(2 [SI::kg], 10 [SI::W])"), 0, "= 20 [SI::K]")
}

// Without a manifest entry the same invocation fails as not registered.
func TestCLIToolCalcRefusesAnUnregisteredTool(t *testing.T) {
	binary := buildCLI(t)
	t.Setenv(analysis.ToolsEnv, t.TempDir())

	wantReport(t, check(t, binary, toolCalcModel,
		"-calc", "thermo::Thermal(2 [SI::kg], 10 [SI::W])"), 2,
		"tool 'Thermo' is not registered; set OPENSYSML_TOOLS")
}

// A document whose table column reads a derived attribute calling the tool calc
// renders the value the tool answered.
func TestCLIToolCalcInADocumentFormula(t *testing.T) {
	binary := buildCLI(t)
	toolCalcManifest(t)

	wantReport(t, check(t, binary, toolCalcDocument,
		"-render-document", "boards::BoardReport"), 0, "Tmax", "20")
}
