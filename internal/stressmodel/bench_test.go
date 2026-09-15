package stressmodel

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// networkSizes are the constellations the benchmarks load, in satellites per
// plane over four planes: 32, 128 and 512 satellites. Larger ones are measured
// from the command line, see docs/project/satellite-network-stress-test.md.
var networkSizes = []int{8, 32, 128}

func network(satellitesPerPlane int) SatelliteNetwork {
	return SatelliteNetwork{Planes: 4, Satellites: satellitesPerPlane, GroundStations: satellitesPerPlane / 2}
}

func loadNetwork(tb testing.TB, src string) *repl.Session {
	tb.Helper()
	sess := repl.NewSession()
	sess.SubmitFiles([]repl.SourceFile{{Name: "satnet.sysml", Text: src}})
	if sess.HasErrors() {
		tb.Fatalf("the network did not analyse cleanly:\n%s", strings.Join(sess.DiagnosticLines(), "\n"))
	}
	return sess
}

// liveHeap returns the reachable heap after a collection: what is held.
func liveHeap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// BenchmarkLoad measures parsing, name resolution and validation of a whole
// network, and reports the heap a loaded session keeps alive, per element.
func BenchmarkLoad(b *testing.B) {
	for _, n := range networkSizes {
		src, stats := network(n).Source()
		b.Run(fmt.Sprintf("satellites=%d/elements=%d", stats.Satellites, stats.Elements), func(b *testing.B) {
			before := liveHeap()
			sess := loadNetwork(b, src)
			held := liveHeap() - before
			runtime.KeepAlive(sess)

			var start, end runtime.MemStats
			runtime.ReadMemStats(&start)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				loadNetwork(b, src)
			}
			b.StopTimer()
			runtime.ReadMemStats(&end)
			allocated := (end.TotalAlloc - start.TotalAlloc) / uint64(b.N)
			b.ReportMetric(float64(held), "live-B/op")
			b.ReportMetric(float64(held)/float64(stats.Elements), "live-B/element")
			b.ReportMetric(float64(allocated)/float64(stats.Elements), "B/element")
		})
	}
}

// BenchmarkSatisfy measures checking every satisfy assertion of a loaded network:
// materializing the constellation, running the mode machines and evaluating the
// budgets. The first check builds the session's runtime, a cost of the session.
func BenchmarkSatisfy(b *testing.B) {
	for _, n := range networkSizes {
		src, stats := network(n).Source()
		b.Run(fmt.Sprintf("satellites=%d/assertions=%d", stats.Satellites, stats.Requirements), func(b *testing.B) {
			sess := loadNetwork(b, src)
			check := func() {
				for _, v := range sess.CheckSatisfy("") {
					if !v.Holds() {
						b.Fatalf("%s: %v", v.Subject, v.Lines)
					}
				}
			}
			check()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				check()
			}
		})
	}
}

// BenchmarkEditBeside measures what an editor pays per keystroke in a small file
// open beside a network: reparse of the small file, reindex, and diagnostics.
func BenchmarkEditBeside(b *testing.B) {
	const small = "package Ops { private import SatelliteNetwork::Constellation::*; part spare : Sat0; }"
	for _, n := range networkSizes {
		src, stats := network(n).Source()
		b.Run(fmt.Sprintf("satellites=%d", stats.Satellites), func(b *testing.B) {
			ws := model.NewWorkspace()
			ws.Open("satnet.sysml", []byte(src), 1)
			for _, d := range ws.Diagnostics("satnet.sysml") {
				b.Fatalf("satnet.sysml: %s", d.Message)
			}
			ws.Open("ops.sysml", []byte(small), 1)
			for _, d := range ws.Diagnostics("ops.sysml") {
				b.Fatalf("ops.sysml: %s", d.Message)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ws.Update("ops.sysml", []byte(small+fmt.Sprintf("\n// %d\n", i)), i+2)
				_ = ws.Diagnostics("ops.sysml")
			}
		})
	}
}

// BenchmarkValidateSplit loads a network split one file per plane as the command
// line does, on one worker and on one per CPU.
func BenchmarkValidateSplit(b *testing.B) {
	for _, n := range networkSizes {
		files, stats := splitFiles(network(n))
		for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
			b.Run(fmt.Sprintf("satellites=%d/files=%d/workers=%d", stats.Satellites, len(files), workers), func(b *testing.B) {
				load := func() {
					sess := repl.NewSession()
					if err := sess.SetWorkers(workers); err != nil {
						b.Fatal(err)
					}
					sess.SubmitFiles(files)
					if sess.HasErrors() {
						b.Fatalf("the split network did not analyse cleanly:\n%s", strings.Join(sess.DiagnosticLines(), "\n"))
					}
				}
				load()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					load()
				}
			})
		}
	}
}

// perDocumentRegistry is the default registry without the workspace-wide audits.
func perDocumentRegistry() *passes.Registry {
	reg := passes.NewRegistry()
	for _, p := range passes.DefaultRegistry().Passes() {
		switch p.(type) {
		case passes.OOSEMMethodPass, passes.IdentityMetadataPass, passes.MOSAPass:
			continue
		}
		reg.Register(p)
	}
	return reg
}

// BenchmarkAnalyzeSplitPerDocument analyzes the split network's files over one
// index without the workspace-wide audits: the pool's own speedup.
func BenchmarkAnalyzeSplitPerDocument(b *testing.B) {
	for _, n := range networkSizes {
		files, stats := splitFiles(network(n))
		idx, _ := model.NewIndexWithStdlib()
		roots := make([]*ast.RootNamespace, len(files))
		names := make([]string, len(files))
		for i, f := range files {
			p := parser.New(source.New(f.Name, []byte(f.Text)))
			roots[i] = p.ParseFile()
			if len(p.Diagnostics) > 0 {
				b.Fatalf("%s: %s", f.Name, p.Diagnostics[0].Message)
			}
			names[i] = f.Name
			idx.AddDocument(f.Name, roots[i])
		}
		idx.ExpandWildcardImports()
		batch := &passes.Batch{Documents: names}
		passes.PrepareBatch(idx, batch)
		reg := perDocumentRegistry()
		for _, workers := range []int{1, runtime.GOMAXPROCS(0)} {
			b.Run(fmt.Sprintf("satellites=%d/files=%d/workers=%d", stats.Satellites, len(files), workers), func(b *testing.B) {
				analyze := func() {
					model.ParallelFor(workers, len(files), func(i int) {
						ctx := passes.NewContext(names[i], idx, nil)
						ctx.Batch = batch
						if diags := reg.Run(ctx, names[i], roots[i]); len(diags) > 0 {
							b.Errorf("%s: %s", names[i], diags[0].Message)
						}
					})
				}
				analyze()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					analyze()
				}
			})
		}
	}
}
