package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// toolCalcEnv are the stand-in's own variables, from testdata/toolcalc.
const (
	toolCalcMode = "TOOL_CALC_MODE"
	toolCalcUnit = "TOOL_CALC_UNIT"
)

// toolCalcDriver is a calc def computed by the stand-in and a usage of it.
const toolCalcDriver = `package Calc {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;

	calc def Thermal {
		metadata ToolExecution { toolName = "Thermo"; uri = "thermo://x"; }
		in m : MassValue   { @ToolVariable { name = "mass"; } }
		in p : PowerValue  { @ToolVariable { name = "power"; } }
		out warn : Boolean { @ToolVariable { name = "warn"; } }
		return : TemperatureValue { @ToolVariable { name = "Tmax"; } }
	}

	calc t : Thermal {
		in m = 2 [SI::kg];
		in p = 10 [SI::W];
	}
}`

var (
	toolCalcOnce    sync.Once
	toolCalcBinPath string
	toolCalcBinErr  error
)

// toolcalc builds the calc stand-in once per test binary and returns its path.
func toolcalc(t *testing.T) string {
	t.Helper()
	toolCalcOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolcalc")
		if err != nil {
			toolCalcBinErr = err
			return
		}
		toolCalcBinPath = filepath.Join(dir, "toolcalc")
		build := exec.Command("go", gobuild.Args(toolCalcBinPath)...)
		build.Dir = filepath.Join("testdata", "toolcalc")
		if out, err := build.CombinedOutput(); err != nil {
			toolCalcBinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if toolCalcBinErr != nil {
		t.Fatalf("building the calc stand-in: %v", toolCalcBinErr)
	}
	return toolCalcBinPath
}

// thermoEntry is the manifest entry the driver's ToolExecution resolves to.
func thermoEntry(executable string) ToolEntry {
	return ToolEntry{ToolName: "Thermo", Version: "stand-in", Executable: executable,
		Variables: []string{"mass", "power", "warn", "Tmax"}}
}

// cprobe is the calc driver indexed over the standard libraries.
type cprobe struct {
	idx *symbols.Index
	pkg *symbols.Scope
}

func parseCProbe(t *testing.T) *cprobe {
	t.Helper()
	idx := libs.NewModelIndex()
	p := parser.New(source.New("cprobe.sysml", []byte(toolCalcDriver)))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx.AddDocument("cprobe.sysml", file)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("cprobe.sysml").LookupLocal("Calc")
	if !ok || pkg.Scope == nil {
		t.Fatal("Calc package not indexed")
	}
	return &cprobe{idx: idx, pkg: pkg.Scope}
}

func (p *cprobe) context() *runtime.Context {
	resolver := resolve.New(p.idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	return runtime.NewContext(model, fixtureSteps)
}

func (p *cprobe) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := p.pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	return sym
}

// evalText evaluates one expression in the driver's scope, for invocation arguments.
func (p *cprobe) evalText(t *testing.T, ctx *runtime.Context, text string) runtime.Value {
	t.Helper()
	expr, ok := parser.ParseOneExpression("cprobe.sysml", text)
	if !ok {
		t.Fatalf("parsing %q", text)
	}
	value, err := runtime.NewEvalContextIn(ctx, p.pkg, nil).Eval(expr)
	if err != nil {
		t.Fatalf("evaluating %q: %v", text, err)
	}
	return value
}

// tooled attaches the registry's runner to the context, as a surface does.
func (p *cprobe) tooled(r *Registry, ctx *runtime.Context) {
	ctx.SetToolRunner(r.ToolRunner(context.Background(), ctx, Budget{}, Auto()))
}

// invoke puts one calculation of the driver's Thermal to the stand-in.
func (p *cprobe) invoke(t *testing.T, r *Registry) (runtime.Value, error) {
	t.Helper()
	ctx := p.context()
	p.tooled(r, ctx)
	args := []runtime.Value{p.evalText(t, ctx, "2 [SI::kg]"), p.evalText(t, ctx, "10 [SI::W]")}
	return ctx.InvokeCalc(p.symbol(t, "Thermal"), args, p.pkg)
}

// The registry's runner attached to a held context computes the annotated calc: the
// result is the stand-in's answer converted to the result parameter's coherent unit.
func TestToolCalcThroughTheRegistryRunner(t *testing.T) {
	p := parseCProbe(t)
	t.Setenv(ToolEnvPassthroughEnv, toolCalcUnit)
	t.Setenv(toolCalcUnit, "K")
	r := toolRegistry(t, manifestDir(t, thermoEntry(toolcalc(t))))

	result, err := p.invoke(t, r)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if got := runtime.FormatValue(result); got != "20 [SI::K]" {
		t.Fatalf("result = %s, want 20 [SI::K]", got)
	}
}

// Every way the stand-in fails is the typed error of its kind: the refusal it
// states, its process's exit, its silence past the timeout.
func TestToolCalcFailsWithTheToolsFault(t *testing.T) {
	cases := []struct {
		mode string
		kind runtime.ToolErrorKind
		text string
	}{
		{"error", runtime.ToolRefused, "thermal model did not converge"},
		{"exit", runtime.ToolProcessFailed, "license server unreachable"},
		{"hang", runtime.ToolTimeout, ToolTimeoutEnv},
	}
	p := parseCProbe(t)
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv(ToolEnvPassthroughEnv, toolCalcMode)
			t.Setenv(toolCalcMode, tc.mode)
			if tc.mode == "hang" {
				t.Setenv(ToolTimeoutEnv, "200ms")
			}
			r := toolRegistry(t, manifestDir(t, thermoEntry(toolcalc(t))))
			_, err := p.invoke(t, r)
			var fault *runtime.ToolError
			if !errors.As(err, &fault) || fault.Kind != tc.kind || fault.Tool != "Thermo" {
				t.Fatalf("InvokeCalc = %v, want a ToolError of kind %s", err, tc.kind)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("error %q does not carry %q", err, tc.text)
			}
		})
	}
}

// A manifest naming another tool leaves Thermo unregistered: the calculation is
// refused and no answer is invented.
func TestToolCalcRefusesAnUnregisteredTool(t *testing.T) {
	p := parseCProbe(t)
	other := ToolEntry{ToolName: "Other", Executable: toolcalc(t), Variables: []string{"mass"}}
	r := toolRegistry(t, manifestDir(t, other))
	_, err := p.invoke(t, r)
	var refusal *runtime.ToolNotRegisteredError
	if !errors.As(err, &refusal) || refusal.Tool != "Thermo" {
		t.Fatalf("InvokeCalc = %v, want ToolNotRegisteredError", err)
	}
	if want := "tool 'Thermo' is not registered; set OPENSYSML_TOOLS"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not carry %q", err, want)
	}
}

// A usage of the annotated calc, read as a feature, computes through the tool as
// the invocation does: its inputs bind from its own member values.
func TestToolCalcUsageThroughTheRegistryRunner(t *testing.T) {
	p := parseCProbe(t)
	t.Setenv(ToolEnvPassthroughEnv, toolCalcUnit)
	r := toolRegistry(t, manifestDir(t, thermoEntry(toolcalc(t))))
	ctx := p.context()
	p.tooled(r, ctx)

	outputs, err := ctx.CalcUsageOutputs(p.symbol(t, "t"), p.pkg, nil)
	if err != nil {
		t.Fatalf("CalcUsageOutputs: %v", err)
	}
	got := map[string]string{}
	for _, out := range outputs {
		got[out.Name] = runtime.FormatValue(out.Value)
	}
	if got["warn"] != "false" {
		t.Fatalf("outputs %v, want warn = false", got)
	}
}
