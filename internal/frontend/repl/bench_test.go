package repl

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// Benchmarks over synthetic models of a stated size: a super-linear cost is
// visible only across sizes. See docs/internals/performance.md.
//
//	go test ./internal/repl -run '^$' -bench . -benchmem
//
// Beyond the standard figures, B/element is memory allocated per model element,
// and live-B/op is memory the loaded model holds with the session reachable.
const benchElementsPerPart = 5 // part def, calc def, action def, state machine, part usage

// modelSizes are the element counts the load benchmarks run at. They double, so
// a super-linear cost shows as a per-element figure that grows with size.
var modelSizes = []int{50, 200, 800}

// emptyModel has no elements, so its load cost is what a session costs before it
// holds anything: the standard library it indexes names against.
const emptyModel = "package BenchModel {\n    import ScalarValues::*;\n}\n"

// syntheticModel returns parts repetitions of a part definition, a calculation,
// an action, a state machine and a part usage. Each part refers to the next, so
// name resolution has work that grows with the model.
func syntheticModel(parts int) string {
	var b strings.Builder
	b.WriteString("package BenchModel {\n    import ScalarValues::*;\n")
	for i := 0; i < parts; i++ {
		fmt.Fprintf(&b, `
    part def Comp%[1]d {
        attribute mass : Real;
        attribute power : Real;
        part sub : Comp%[2]d;
        constraint MassOK {
            mass > 0.0
        }
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
    state SM%[1]d {
        entry; then start;
        state start;
        state idle;
        state running;

        succession first start then idle;
        transition first idle then running;
        transition first running then done;
    }
    part inst%[1]d : Comp%[1]d {
        attribute :>> mass = %[1]d.0;
        attribute :>> power = %[1]d.5;
    }
`, i, (i+1)%parts)
	}
	b.WriteString("}\n")
	return b.String()
}

func warningModel(parts int) string {
	var b strings.Builder
	b.WriteString("package BenchWarnings {\n")
	for i := 0; i < parts; i++ {
		fmt.Fprintf(&b, "    attribute flag%d = 1 == \"one\";\n", i)
	}
	b.WriteString("}\n")
	return b.String()
}

// loadModel loads src into a fresh session, failing if it does not analyse
// cleanly, since an erroring model skips the later passes.
func loadModel(tb testing.TB, src string) *Session {
	tb.Helper()
	sess := NewSession()
	sess.SubmitFiles([]SourceFile{{Name: "bench.sysml", Text: src}})
	if sess.HasErrors() {
		tb.Fatalf("the benchmark model did not analyse cleanly:\n%s", strings.Join(sess.DiagnosticLines(), "\n"))
	}
	return sess
}

// liveHeap returns the reachable heap, collecting first so that what it reports
// is what is held.
func liveHeap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// BenchmarkLoadModel measures parsing, name resolution and the validation passes
// over a whole model, which is what `sysml -validate` spends its time in.
func BenchmarkLoadModel(b *testing.B) {
	sizes := append([]int{0}, modelSizes...)
	for _, parts := range sizes {
		src := emptyModel
		if parts > 0 {
			src = syntheticModel(parts)
		}
		elements := parts * benchElementsPerPart
		b.Run(fmt.Sprintf("elements=%d", elements), func(b *testing.B) {
			before := liveHeap()
			sess := loadModel(b, src)
			held := liveHeap() - before
			runtime.KeepAlive(sess)

			var start, end runtime.MemStats
			runtime.ReadMemStats(&start)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				loadModel(b, src)
			}
			b.StopTimer()
			runtime.ReadMemStats(&end)

			allocated := (end.TotalAlloc - start.TotalAlloc) / uint64(b.N)
			b.ReportMetric(float64(held), "live-B/op")
			if elements > 0 {
				b.ReportMetric(float64(allocated)/float64(elements), "B/element")
			}
		})
	}
}

// benchmarkRun measures one run over an already-loaded model, at each model size:
// a figure that grows with model size is a cost the run pays for the model.
func benchmarkRun(b *testing.B, run func(b *testing.B, sess *Session)) {
	for _, parts := range modelSizes {
		src := syntheticModel(parts)
		b.Run(fmt.Sprintf("elements=%d", parts*benchElementsPerPart), func(b *testing.B) {
			sess := loadModel(b, src)
			// The first run builds the session's runtime and indexes the standard
			// library, a cost of the session rather than of a run.
			run(b, sess)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				run(b, sess)
			}
		})
	}
}

// BenchmarkRunStateMachine measures starting a state machine in a loaded model.
func BenchmarkRunStateMachine(b *testing.B) {
	benchmarkRun(b, func(b *testing.B, sess *Session) {
		if v := sess.RunStateMachine("SM0"); !v.Holds() {
			b.Fatalf("SM0 did not run: %v", v.Lines)
		}
	})
}

// BenchmarkRunCalc measures evaluating a calculation in a loaded model, which is
// the expression evaluator over a model whose scopes it must search.
func BenchmarkRunCalc(b *testing.B) {
	benchmarkRun(b, func(b *testing.B, sess *Session) {
		if v := sess.RunCalc("Calc0(2.0, 3.0)"); !v.Holds() {
			b.Fatalf("Calc0 did not run: %v", v.Lines)
		}
	})
}

// BenchmarkInstantiate measures creating an object of a part definition, which
// is what a check about an object pays before it evaluates anything.
func BenchmarkInstantiate(b *testing.B) {
	benchmarkRun(b, func(b *testing.B, sess *Session) {
		if _, err := sess.InstantiateNamed("Comp0"); err != nil {
			b.Fatal(err)
		}
	})
}

// BenchmarkDiagnostics measures rendering and locating diagnostics across a
// warning-emitting model, one warning per declared attribute.
func BenchmarkDiagnostics(b *testing.B) {
	for _, attributes := range modelSizes {
		src := warningModel(attributes)
		b.Run(fmt.Sprintf("attributes=%d", attributes), func(b *testing.B) {
			sess := NewSession()
			sess.Submit(src)
			diagnostics := len(sess.Diagnostics())
			if diagnostics == 0 {
				b.Fatal("warning model produced no diagnostics")
			}

			var start, end runtime.MemStats
			runtime.ReadMemStats(&start)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sess.DiagnosticLines()
				sess.LocatedDiagnostics()
			}
			b.StopTimer()
			runtime.ReadMemStats(&end)

			allocated := (end.TotalAlloc - start.TotalAlloc) / uint64(b.N)
			b.ReportMetric(float64(allocated)/float64(2*diagnostics), "B/diagnostic")
			runtime.KeepAlive(sess)
		})
	}
}
