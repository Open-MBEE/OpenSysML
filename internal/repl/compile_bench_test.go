package repl

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

// BenchmarkCompiledCalc times each invocation interpreted and as each backend's
// executable; the executable amortizes start-up over one --repeat b.N run.
func BenchmarkCompiledCalc(b *testing.B) {
	for _, c := range []compiledCase{
		{"Fib", []string{"25"}},
		{"SumTo", []string{"1000000"}},
		{"Collatz", []string{"27"}},
		{"Hypot", []string{"3.0", "4.0"}},
	} {
		name := c.calc + "(" + strings.Join(c.args, ",") + ")"
		b.Run(name+"/interpreted", func(b *testing.B) {
			s := loadCompileFixture(b)
			invocation := "Compiled::" + c.calc + "(" + strings.Join(c.args, ", ") + ")"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if v := s.RunCalc(invocation); v.Status != VerdictHolds {
					b.Fatal(v.Lines)
				}
			}
		})
		for _, target := range codegen.Targets() {
			b.Run(name+"/"+string(target), func(b *testing.B) {
				if target == codegen.TargetC {
					if _, err := exec.LookPath("cc"); err != nil {
						b.Skip("no C compiler on PATH")
					}
				}
				s := loadCompileFixture(b)
				program, err := s.CompileCalc("Compiled::" + c.calc)
				if err != nil {
					b.Fatal(err)
				}
				exe := filepath.Join(b.TempDir(), c.calc)
				if err := codegen.Build(program, target, exe); err != nil {
					b.Fatal(err)
				}
				args := append([]string{"--repeat", strconv.Itoa(b.N)}, c.args...)
				b.ResetTimer()
				if out, err := exec.Command(exe, args...).CombinedOutput(); err != nil {
					b.Fatalf("%v\n%s", err, out)
				}
			})
		}
	}
}
