package codegen_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

// builtCalc is one fixture calc built for one target, shared by the tests that run it.
type builtCalc struct {
	once    sync.Once
	exe     string
	program *codegen.Program
	err     error
}

var builtCalcs struct {
	sync.Mutex
	dir   string
	calcs map[string]*builtCalc
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "compiled-calcs")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	builtCalcs.dir, builtCalcs.calcs = dir, map[string]*builtCalc{}
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

// fixtureCalc returns the executable and program of a fixture calc for target,
// building them the first time any test asks.
func fixtureCalc(t *testing.T, s *repl.Session, calc string, target codegen.Target) (string, *codegen.Program) {
	t.Helper()
	key := string(target) + "/" + calc
	builtCalcs.Lock()
	b := builtCalcs.calcs[key]
	if b == nil {
		b = &builtCalc{exe: filepath.Join(builtCalcs.dir, string(target)+"-"+calc)}
		builtCalcs.calcs[key] = b
	}
	builtCalcs.Unlock()
	b.once.Do(func() {
		if b.program, b.err = compileCalc(s, "Compiled::"+calc, target); b.err != nil {
			b.err = fmt.Errorf("compile %s: %w", calc, b.err)
		} else if b.err = codegen.Build(b.program, target, b.exe); b.err != nil {
			b.err = fmt.Errorf("build %s: %w", calc, b.err)
		}
	})
	if b.err != nil {
		t.Fatal(b.err)
	}
	return b.exe, b.program
}
