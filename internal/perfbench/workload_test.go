package perfbench

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// syntheticBlocks sizes the generated model: 1000 blocks is ~1.4 MiB of source.
const syntheticBlocks = 1000

// vehiclePath is the largest pilot-corpus model; benchmarks skip while it is absent.
const vehiclePath = "../../examples/pilot-corpora/sysml-examples/Vehicle Example/SysML v2 Spec Annex A SimpleVehicleModel.sysml"

// syntheticModel writes n blocks, each with a part def and constraints, a
// requirement, a calc, an action with a loop, a state machine and a satisfy.
func syntheticModel(n int) []byte {
	var w bytes.Buffer
	fmt.Fprintln(&w, "package Perf {")
	fmt.Fprintln(&w, "    private import ScalarValues::*;")
	for i := 0; i < n; i++ {
		next := (i + 1) % n
		fmt.Fprintf(&w, `
    part def Comp%[1]d {
        attribute mass : Real;
        attribute power : Real;
        attribute limit : Real = 100.0;
        part sub : Comp%[2]d;
        constraint massOK { mass > 0.0 and mass < limit }
        constraint powerOK { power >= 0.0 }
    }
    requirement def Req%[1]d {
        subject c : Comp%[1]d;
        require constraint { c.mass < 1000.0 }
    }
    calc def Calc%[1]d {
        in a : Real;
        in b : Real;
        return : Real = a * b + %[1]d;
    }
    action def Act%[1]d {
        in x : Real;
        out y : Real = x * 2.0;
    }
    action proc%[1]d {
        attribute total : Integer = 0;
        first start;
        action iterate {
            for i in 1..10 {
                assign total := total + i;
            }
        }
        done;
        succession first start then iterate;
        succession first iterate then done;
    }
    state def SM%[1]d {
        attribute count : Integer = 0;
        entry; then s0;
        state s0;
        accept after 1 [SI::s] if count < 50 then s1;
        accept after 1 [SI::s] if count >= 50 then done;
        state s1 {
            entry assign count := count + 1;
        }
        accept after 1 [SI::s] then s0;
    }
    part inst%[1]d : Comp%[1]d {
        attribute :>> mass = %[1]d.0 + 1.0;
        attribute :>> power = %[1]d.5;
        exhibit state sm : SM%[1]d;
    }
    requirement req%[1]d : Req%[1]d;
    satisfy req%[1]d by inst%[1]d;
`, i, next)
	}
	fmt.Fprintln(&w, "}")
	return w.Bytes()
}

var (
	syntheticOnce sync.Once
	syntheticSrc  []byte
	syntheticDir  string
	syntheticFile string
)

// TestMain removes the shared synthetic model once every benchmark is done with it.
func TestMain(m *testing.M) {
	code := m.Run()
	if syntheticDir != "" {
		os.RemoveAll(syntheticDir)
	}
	os.Exit(code)
}

// synthetic returns the generated model's source and a file holding it.
func synthetic(tb testing.TB) ([]byte, string) {
	syntheticOnce.Do(func() {
		syntheticSrc = syntheticModel(syntheticBlocks)
		dir, err := os.MkdirTemp("", "perfbench")
		if err != nil {
			tb.Fatal(err)
		}
		syntheticDir = dir
		syntheticFile = filepath.Join(dir, "perf.sysml")
		if err := os.WriteFile(syntheticFile, syntheticSrc, 0o600); err != nil {
			tb.Fatal(err)
		}
	})
	return syntheticSrc, syntheticFile
}

func syntheticSource(tb testing.TB) []byte {
	src, _ := synthetic(tb)
	return src
}

func syntheticPath(tb testing.TB) string {
	_, path := synthetic(tb)
	return path
}

// vehicle returns the pilot Vehicle model, skipping when the corpora are absent.
func vehicle(tb testing.TB) []byte {
	src, err := os.ReadFile(vehiclePath)
	if err != nil {
		tb.Skipf("pilot corpora absent: %v", err)
	}
	return src
}

// workload is a named model the size-sensitive benchmarks run over.
type workload struct {
	name string
	src  func(testing.TB) []byte
}

var workloads = []workload{
	{"synthetic", syntheticSource},
	{"vehicle", vehicle},
}

// TestSyntheticModelValidates keeps the generator in step with the grammar.
func TestSyntheticModelValidates(t *testing.T) {
	p := parser.New(source.New("perf.sysml", syntheticModel(3)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("synthetic model: %v", p.Diagnostics[0])
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("perf.sysml", root)
	idx.ExpandWildcardImports()
	for _, d := range passes.AnalyzeWithOptions("perf.sysml", source.KindSysML, root, nil, idx, passes.Options{}) {
		if d.Severity == passes.SeverityError {
			t.Errorf("synthetic model: %s", d.Message)
		}
	}
}
