package stressmodel

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// residencyNetworks are the split constellations BenchmarkPlaneResidency holds:
// the benchmark sizes over four planes, and the 1 600-satellite network of
// docs/project/satellite-network-stress-test.md over 32.
var residencyNetworks = []SatelliteNetwork{
	network(8), network(32), network(128),
	{Planes: 32, Satellites: 50, GroundStations: 160},
}

// BenchmarkPlaneResidency measures what a workspace holds for a network split
// one file per plane with every plane loaded, against the planes held as their
// interface records and only the library and the constellation loaded. It
// reports the reachable heap after a collection in both states, per satellite,
// and the size of a plane's record on disk.
func BenchmarkPlaneResidency(b *testing.B) {
	b.Setenv("XDG_CACHE_HOME", b.TempDir())
	cache, err := libs.NewCache()
	if err != nil {
		b.Fatal(err)
	}
	digest := libs.SourceDigest(libs.DefaultSource())
	for _, n := range residencyNetworks {
		files, stats := n.Split()
		b.Run(fmt.Sprintf("satellites=%d/files=%d", stats.Satellites, len(files)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				measurePlaneResidency(b, cache, digest, files, stats)
			}
		})
	}
}

func measurePlaneResidency(b *testing.B, cache *libs.Cache, digest string, files []File, stats Stats) {
	inputs := make([]model.Input, len(files))
	names := make([]string, len(files))
	var planes []string
	for i, f := range files {
		inputs[i] = model.Input{Name: f.Name, Content: []byte(f.Source), Version: 1}
		names[i] = f.Name
		if strings.HasPrefix(f.Name, "plane") {
			planes = append(planes, f.Name)
		}
	}
	before := liveHeap()

	loaded := model.NewWorkspace()
	loaded.OpenAll(inputs)
	if n := countDiagnostics(loaded.DiagnosticsAll(names)); n != 0 {
		b.Fatalf("the split network did not analyse cleanly: %d diagnostics", n)
	}
	heldLoaded := liveHeap() - before

	records := make([]*libs.InterfaceRecord, 0, len(planes))
	var onDisk int64
	for _, plane := range planes {
		rec, err := loaded.InterfaceRecord(plane)
		if err != nil {
			b.Fatal(err)
		}
		records = append(records, rec)
		key := cache.InterfaceKey([]byte(sourceOf(files, plane)), digest, diag.ConformanceDefault)
		if err := cache.StoreInterface(key, rec); err != nil {
			b.Fatal(err)
		}
		onDisk += recordFileSize(b, key)
	}
	loaded = nil

	recorded := model.NewWorkspace()
	var kept []model.Input
	for _, in := range inputs {
		if !strings.HasPrefix(in.Name, "plane") {
			kept = append(kept, in)
		}
	}
	recorded.OpenAll(kept)
	beforeRecords := liveHeap()
	for _, rec := range records {
		if err := recorded.OpenRecorded(rec); err != nil {
			b.Fatal(err)
		}
	}
	records = nil
	heldRecords := liveHeap() - beforeRecords
	if n := countDiagnostics(recorded.DiagnosticsAll(names)); n != 0 {
		b.Fatalf("the network with its planes recorded did not analyse cleanly: %d diagnostics", n)
	}
	heldRecorded := liveHeap() - before
	runtime.KeepAlive(recorded)

	sats := float64(stats.Satellites)
	b.ReportMetric(float64(heldLoaded)/sats, "loaded-B/satellite")
	b.ReportMetric(float64(heldRecorded)/sats, "recorded-B/satellite")
	b.ReportMetric(float64(heldRecords)/sats, "records-B/satellite")
	b.ReportMetric(float64(heldLoaded)/(1<<20), "loaded-MiB")
	b.ReportMetric(float64(heldRecorded)/(1<<20), "recorded-MiB")
	b.ReportMetric(float64(heldRecords)/(1<<20), "records-MiB")
	b.ReportMetric(float64(onDisk)/float64(len(planes))/1024, "record-KiB/plane")
}

func sourceOf(files []File, name string) string {
	for _, f := range files {
		if f.Name == name {
			return f.Source
		}
	}
	return ""
}

func countDiagnostics(all [][]diag.Diagnostic) int {
	n := 0
	for _, d := range all {
		n += len(d)
	}
	return n
}

// recordFileSize is the size of the record the cache stored under key.
func recordFileSize(b *testing.B, key string) int64 {
	b.Helper()
	matches, err := filepath.Glob(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "sysml-ls", "libs", key+".*"))
	if err != nil || len(matches) != 1 {
		b.Fatalf("record for %s not found in the cache: %v %v", key, matches, err)
	}
	info, err := os.Stat(matches[0])
	if err != nil {
		b.Fatal(err)
	}
	return info.Size()
}

// TestPlaneResidencyProcess holds the 1 600-satellite split network in the
// state OPENSYSML_RESIDENCY names, so that a process running it alone can be
// measured from outside (`/usr/bin/time -v`): "loaded" holds every plane as a
// loaded document; "write" does the same and writes the planes' records to the
// cache under XDG_CACHE_HOME; "recorded" reads those records back and holds
// the planes as records, never having parsed them. It is skipped when the
// variable is unset.
func TestPlaneResidencyProcess(t *testing.T) {
	state := os.Getenv("OPENSYSML_RESIDENCY")
	if state == "" {
		t.Skip("OPENSYSML_RESIDENCY unset")
	}
	cache, err := libs.NewCache()
	if err != nil {
		t.Fatal(err)
	}
	digest := libs.SourceDigest(libs.DefaultSource())
	files, _ := residencyNetworks[len(residencyNetworks)-1].Split()
	inputs := make([]model.Input, len(files))
	names := make([]string, len(files))
	var kept []model.Input
	var planes []model.Input
	for i, f := range files {
		inputs[i] = model.Input{Name: f.Name, Content: []byte(f.Source), Version: 1}
		names[i] = f.Name
		if strings.HasPrefix(f.Name, "plane") {
			planes = append(planes, inputs[i])
		} else {
			kept = append(kept, inputs[i])
		}
	}
	ws := model.NewWorkspace()
	switch state {
	case "loaded", "write":
		ws.OpenAll(inputs)
	case "recorded":
		ws.OpenAll(kept)
		for _, plane := range planes {
			rec, ok := cache.LoadInterface(cache.InterfaceKey(plane.Content, digest, diag.ConformanceDefault))
			if !ok {
				t.Fatalf("no record for %s in the cache: run with OPENSYSML_RESIDENCY=write first", plane.Name)
			}
			if err := ws.OpenRecorded(rec); err != nil {
				t.Fatal(err)
			}
		}
	default:
		t.Fatalf("OPENSYSML_RESIDENCY=%q: want loaded, write or recorded", state)
	}
	if n := countDiagnostics(ws.DiagnosticsAll(names)); n != 0 {
		t.Fatalf("the split network did not analyse cleanly: %d diagnostics", n)
	}
	if state == "write" {
		for _, plane := range planes {
			rec, err := ws.InterfaceRecord(plane.Name)
			if err != nil {
				t.Fatal(err)
			}
			if err := cache.StoreInterface(cache.InterfaceKey(plane.Content, digest, diag.ConformanceDefault), rec); err != nil {
				t.Fatal(err)
			}
		}
	}
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("%s: live heap %d MiB", state, ms.HeapAlloc>>20)
	runtime.KeepAlive(ws)
}
