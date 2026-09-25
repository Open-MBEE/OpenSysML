package stressmodel

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// TestSatelliteNetworkValidates keeps the generator in step with the grammar and
// the validation passes: a small constellation loads under strict conformance
// without a diagnostic, and every satisfy assertion it states holds.
func TestSatelliteNetworkValidates(t *testing.T) {
	n := SatelliteNetwork{Planes: 2, Satellites: 2, GroundStations: 1}
	src, stats := n.Source()
	if stats.Satellites != 4 || stats.GroundStations != 1 {
		t.Fatalf("stats = %+v, want 4 satellites and 1 station", stats)
	}
	if stats.Requirements != 3*stats.Satellites {
		t.Errorf("Requirements = %d, want three per satellite", stats.Requirements)
	}
	if stats.Bytes != len(src) {
		t.Errorf("Bytes = %d, want %d", stats.Bytes, len(src))
	}

	s := repl.NewSession()
	s.SetConformanceMode(diag.ConformanceModeOf(true))
	for _, d := range s.Submit(src).Diagnostics {
		t.Errorf("diagnostic: %s", d.Message)
	}
	verdicts := s.CheckSatisfy("")
	if len(verdicts) != stats.Requirements {
		t.Fatalf("got %d satisfy verdicts, want %d", len(verdicts), stats.Requirements)
	}
	for _, v := range verdicts {
		if !v.Holds() {
			t.Errorf("%s: %v", v.Subject, v.Lines)
		}
	}
}

// TestSatelliteNetworkScales checks the shape grows with the configuration:
// every satellite carries the same components, and links join the planes.
func TestSatelliteNetworkScales(t *testing.T) {
	one, _ := SatelliteNetwork{Planes: 1, Satellites: 1}.Source()
	_, small := SatelliteNetwork{Planes: 1, Satellites: 4, GroundStations: 1}.Source()
	_, large := SatelliteNetwork{Planes: 2, Satellites: 4, GroundStations: 1}.Source()
	if large.Satellites != 2*small.Satellites {
		t.Fatalf("satellites: %d vs %d", large.Satellites, small.Satellites)
	}
	if large.Components-large.GroundStations*len(stationComponents) != 2*(small.Components-small.GroundStations*len(stationComponents)) {
		t.Errorf("components do not double with the satellites: %+v vs %+v", large, small)
	}
	if large.Connections <= 2*small.Connections {
		t.Errorf("a second plane adds no inter-plane links: %+v vs %+v", large, small)
	}
	if !strings.Contains(one, "part def Sat0 :> Spacecraft") {
		t.Errorf("first satellite definition missing from:\n%s", one)
	}
}

// splitFiles is a network split by plane as the CLI loads it, one source per file.
func splitFiles(n SatelliteNetwork) ([]repl.SourceFile, Stats) {
	files, stats := n.Split()
	srcs := make([]repl.SourceFile, len(files))
	for i, f := range files {
		srcs[i] = repl.SourceFile{Name: f.Name, Text: f.Source}
	}
	return srcs, stats
}

// TestSatelliteNetworkSplitValidates: the split declares the single file's
// network, loads clean at one worker and at several, and satisfies across files.
func TestSatelliteNetworkSplitValidates(t *testing.T) {
	n := SatelliteNetwork{Planes: 2, Satellites: 2, GroundStations: 1}
	_, whole := n.Source()
	files, stats := splitFiles(n)
	if len(files) != n.Planes+2 {
		t.Fatalf("got %d files, want one per plane beside the library and the constellation", len(files))
	}
	// The split declares one package per plane over the single file's elements.
	whole.Bytes, stats.Bytes = 0, 0
	whole.Elements += n.Planes
	if stats != whole {
		t.Errorf("split declares %+v, the single file %+v", stats, whole)
	}

	for _, jobs := range []int{1, 4} {
		s := repl.NewSession()
		s.SetConformanceMode(diag.ConformanceModeOf(true))
		if err := s.SetJobs(jobs); err != nil {
			t.Fatal(err)
		}
		for _, d := range s.SubmitFiles(files).Diagnostics {
			t.Errorf("jobs=%d: diagnostic: %s", jobs, d.Message)
		}
		verdicts := s.CheckSatisfy("")
		if len(verdicts) != stats.Requirements {
			t.Fatalf("jobs=%d: got %d satisfy verdicts, want %d", jobs, len(verdicts), stats.Requirements)
		}
		for _, v := range verdicts {
			if !v.Holds() {
				t.Errorf("jobs=%d: %s: %v", jobs, v.Subject, v.Lines)
			}
		}
	}
}

// TestSatelliteNetworkFilesValidate keeps the multi-file form in step with the
// single one: the same constellation split by plane loads clean under strict
// conformance, one document per file, and every satisfy assertion holds.
func TestSatelliteNetworkFilesValidate(t *testing.T) {
	n := SatelliteNetwork{Planes: 2, Satellites: 2, GroundStations: 1}
	files, stats := n.Split()
	if len(files) != n.Planes+2 {
		t.Fatalf("got %d files, want the library, %d planes and the network", len(files), n.Planes)
	}
	_, whole := n.Source()
	if stats.Satellites != whole.Satellites || stats.Requirements != whole.Requirements || stats.Connections != whole.Connections {
		t.Fatalf("split stats %+v, single-file stats %+v", stats, whole)
	}
	ws := model.NewWorkspace(model.WithConformanceMode(diag.ConformanceModeOf(true)))
	for _, f := range files {
		ws.Open(f.Name, []byte(f.Source), 1)
	}
	for _, f := range files {
		for _, d := range ws.Diagnostics(f.Name) {
			t.Errorf("%s: %s", f.Name, d.Message)
		}
	}

	s := repl.NewSession()
	s.SetConformanceMode(diag.ConformanceModeOf(true))
	sources := make([]repl.SourceFile, 0, len(files))
	for _, f := range files {
		sources = append(sources, repl.SourceFile{Name: f.Name, Text: f.Source})
	}
	for _, d := range s.SubmitFiles(sources).Diagnostics {
		t.Errorf("diagnostic: %s", d.Message)
	}
	verdicts := s.CheckSatisfy("")
	if len(verdicts) != stats.Requirements {
		t.Fatalf("got %d satisfy verdicts, want %d", len(verdicts), stats.Requirements)
	}
	for _, v := range verdicts {
		if !v.Holds() {
			t.Errorf("%s: %v", v.Subject, v.Lines)
		}
	}
}
