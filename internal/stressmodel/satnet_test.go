package stressmodel

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
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
	s.SetConformanceMode(conformance.ModeOf(true))
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

// TestSatelliteNetworkSplitValidates keeps the split in step with the single
// file: it declares the same network, loads clean under strict conformance at
// one worker and at several, and every satisfy assertion holds across files.
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

	for _, workers := range []int{1, 4} {
		s := repl.NewSession()
		s.SetConformanceMode(conformance.ModeOf(true))
		if err := s.SetWorkers(workers); err != nil {
			t.Fatal(err)
		}
		for _, d := range s.SubmitFiles(files).Diagnostics {
			t.Errorf("workers=%d: diagnostic: %s", workers, d.Message)
		}
		verdicts := s.CheckSatisfy("")
		if len(verdicts) != stats.Requirements {
			t.Fatalf("workers=%d: got %d satisfy verdicts, want %d", workers, len(verdicts), stats.Requirements)
		}
		for _, v := range verdicts {
			if !v.Holds() {
				t.Errorf("workers=%d: %s: %v", workers, v.Subject, v.Lines)
			}
		}
	}
}
