package stressmodel

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
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

func fleet(satellitesPerPlane int) SatelliteNetwork {
	n := network(satellitesPerPlane)
	n.Fleet = true
	return n
}

const fleetNetwork = "SatelliteNetwork::Constellation::network"

// BenchmarkFleetInstantiate measures creating the fleet-form network and reading
// the dry mass of every occurrence in every plane: what it costs to materialize
// N spacecraft declared as `Spacecraft[N]` and evaluate a summed attribute over
// each one's component tree.
func BenchmarkFleetInstantiate(b *testing.B) {
	for _, n := range networkSizes {
		src, stats := fleet(n).Source()
		b.Run(fmt.Sprintf("satellites=%d/elements=%d", stats.Satellites, stats.Elements), func(b *testing.B) {
			sess := loadNetwork(b, src)
			read := func() {
				if _, err := sess.InstantiateNamed(fleetNetwork); err != nil {
					b.Fatal(err)
				}
				for p := 0; p < 4; p++ {
					if _, err := sess.EvalExpr(fmt.Sprintf("%s.plane%d.sats.dryMass", fleetNetwork, p)); err != nil {
						b.Fatal(err)
					}
				}
			}
			read()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				read()
			}
			b.StopTimer()
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(stats.Satellites), "ns/satellite")
		})
	}
}

// BenchmarkFleetSatisfy measures the warm re-check of every satisfy assertion of
// a fleet-form network: three per block and three per unit stating values of its
// own, each unit checked as one occurrence of its plane's fleet.
func BenchmarkFleetSatisfy(b *testing.B) {
	for _, n := range networkSizes {
		src, stats := fleet(n).Source()
		assertions := 3 * (stats.Definitions + stats.Units)
		b.Run(fmt.Sprintf("satellites=%d/assertions=%d", stats.Satellites, assertions), func(b *testing.B) {
			sess := loadNetwork(b, src)
			check := func() {
				verdicts := sess.CheckSatisfy("")
				if len(verdicts) != assertions {
					b.Fatalf("got %d verdicts, want %d", len(verdicts), assertions)
				}
				for _, v := range verdicts {
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

// openFiles opens every file of a network in a workspace and analyzes each,
// failing on any diagnostic.
func openFiles(tb testing.TB, ws *model.Workspace, files []File) {
	tb.Helper()
	for _, f := range files {
		ws.Open(f.Name, []byte(f.Source), 1)
	}
	for _, f := range files {
		for _, d := range ws.Diagnostics(f.Name) {
			tb.Fatalf("%s: %s", f.Name, d.Message)
		}
	}
}

// BenchmarkLoadFiles measures analyzing the network split into one document per
// plane, every document through one workspace: what a multi-file model costs
// over the same model in one file.
func BenchmarkLoadFiles(b *testing.B) {
	for _, n := range networkSizes {
		files, stats := network(n).Split()
		b.Run(fmt.Sprintf("satellites=%d/files=%d", stats.Satellites, len(files)), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				openFiles(b, model.NewWorkspace(), files)
			}
		})
	}
}

// BenchmarkEditImported measures the worst edit in a multi-file network: a
// keystroke in the library every plane imports, followed by the diagnostics of
// every open file, as the editor's sweep asks for them.
func BenchmarkEditImported(b *testing.B) {
	for _, n := range networkSizes {
		files, stats := network(n).Split()
		lib := files[0]
		b.Run(fmt.Sprintf("satellites=%d/files=%d", stats.Satellites, len(files)), func(b *testing.B) {
			ws := model.NewWorkspace()
			openFiles(b, ws, files)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ws.Update(lib.Name, []byte(lib.Source+fmt.Sprintf("\n// %d\n", i)), i+2)
				for _, f := range files {
					_ = ws.Diagnostics(f.Name)
				}
			}
		})
	}
}

// TestEditsHoldNoStaleState checks that a workspace edited many times keeps no
// more of the replaced documents than one edited once: the heap a long editing
// session holds is that of the current model, not of every version it saw.
func TestEditsHoldNoStaleState(t *testing.T) {
	if testing.Short() {
		t.Skip("edits a 32-satellite network a thousand times")
	}
	const small = "package Ops { private import SatelliteNetwork::Constellation::*; part spare : Sat0; }"
	src, _ := network(networkSizes[0]).Source()
	ws := model.NewWorkspace()
	ws.Open("satnet.sysml", []byte(src), 1)
	ws.Open("ops.sysml", []byte(small), 1)
	edit := func(i int) {
		ws.Update("ops.sysml", []byte(small+fmt.Sprintf("\n// %d\n", i)), i+2)
		_ = ws.Diagnostics("ops.sysml")
		if i%50 == 0 {
			ws.Update("satnet.sysml", []byte(src+fmt.Sprintf("\n// %d\n", i)), i+2)
		}
		_ = ws.Diagnostics("satnet.sysml")
	}
	edit(0)
	warm := liveHeap()
	for i := 1; i <= 1000; i++ {
		edit(i)
	}
	after := liveHeap()
	runtime.KeepAlive(ws)
	// Memo tables grow a bounded amount under delete-and-reinsert churn; a
	// quarter more is state that outlived the document it was computed for.
	if after > warm+warm/4 {
		t.Fatalf("live heap grew from %d to %d bytes over 1000 edits", warm, after)
	}
	t.Logf("live heap after 1 edit %d bytes, after 1000 edits %d bytes", warm, after)
}
