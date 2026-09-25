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
	if stats.Requirements != 3*stats.Satellites || stats.Assertions != stats.Requirements {
		t.Errorf("Requirements = %d, Assertions = %d, want three per satellite", stats.Requirements, stats.Assertions)
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
	if len(verdicts) != stats.Assertions {
		t.Fatalf("got %d satisfy verdicts, want %d", len(verdicts), stats.Assertions)
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

// TestFleetValidates keeps the fleet form in step with the grammar and the
// runtime: a constellation of a few blocks with occurrence counts loads under
// strict conformance without a diagnostic, its satisfy assertions hold, and the
// occurrences read the block's defaults where a unit states nothing of its own.
func TestFleetValidates(t *testing.T) {
	n := SatelliteNetwork{Planes: 2, Satellites: 2 * fleetUnitStride, GroundStations: 1, Fleet: true}
	src, stats := n.Source()
	if stats.Satellites != 4*fleetUnitStride || stats.GroundStations != 1 {
		t.Fatalf("stats = %+v, want %d satellites and 1 station", stats, 4*fleetUnitStride)
	}
	if stats.Definitions != 2 || stats.Units != 4 {
		t.Errorf("stats = %+v, want 2 blocks and 4 diverging units", stats)
	}
	if stats.Requirements != 3*stats.Definitions || stats.Assertions != 3*(stats.Definitions+stats.Units) {
		t.Errorf("Requirements = %d, Assertions = %d, want three per block and three more per unit", stats.Requirements, stats.Assertions)
	}
	if stats.Bytes != len(src) {
		t.Errorf("Bytes = %d, want %d", stats.Bytes, len(src))
	}
	for _, want := range []string{
		"attribute :>> dataRate default = ",
		"attribute :>> area default = ",
		"connect [1] sats.comms.crosslinkTx to [1] sats.comms.crosslinkRx",
		"connect [1] plane0.sats.comms.crosslinkTx to [1] plane1.sats.comms.crosslinkRx",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source lacks %q", want)
		}
	}

	s := repl.NewSession()
	s.SetConformanceMode(diag.ConformanceModeOf(true))
	for _, d := range s.Submit(src).Diagnostics {
		t.Errorf("diagnostic: %s", d.Message)
	}
	verdicts := s.CheckSatisfy("")
	if want := stats.Assertions; len(verdicts) != want {
		t.Fatalf("got %d satisfy verdicts, want %d", len(verdicts), want)
	}
	for _, v := range verdicts {
		if !v.Holds() {
			t.Errorf("%s: %v", v.Subject, v.Lines)
		}
	}

	const network = "SatelliteNetwork::Constellation::network"
	for expr, want := range map[string]string{
		network + ".plane1.sats#(1).catalogId":                          "= 40032",
		network + ".plane1.sats#(2).slot":                               "= 16",
		network + ".plane1.sats#(3).catalogId":                          "= 50000",
		network + ".plane1.unit0.catalogId":                             "= 40032",
		network + ".plane1.unit16.comms.crosslinkTerminal.serialNumber": `= "CROSSLINKTERMINAL-00048-1"`,
		network + ".plane1.unit16.comms.crosslinkTerminal.dataRate":     "= 244.0",
		network + ".plane1.sats#(3).comms.crosslinkTerminal.dataRate":   "= 103.0",
		network + ".plane1.sats#(3).eps.solarArray.area":                "= 5.1 ['m²']",
		network + ".plane0.sats#(3).plane":                              "= 0",
		network + ".satelliteCount":                                     "= 64",
	} {
		lines, err := s.EvalExpr(expr)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if got := strings.TrimSpace(lines[len(lines)-1]); got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}

// TestFleetScales checks the fleet form declares the blocks, the planes and the
// links but not the satellites: the element count grows with the planes and the
// diverging units, not with the occurrences.
func TestFleetScales(t *testing.T) {
	_, small := SatelliteNetwork{Planes: 2, Satellites: fleetUnitStride, GroundStations: 1, Fleet: true}.Source()
	_, wide := SatelliteNetwork{Planes: 2, Satellites: 2 * fleetUnitStride, GroundStations: 1, Fleet: true}.Source()
	_, wider := SatelliteNetwork{Planes: 2, Satellites: 8 * fleetUnitStride, GroundStations: 1, Fleet: true}.Source()
	_, tall := SatelliteNetwork{Planes: 4, Satellites: fleetUnitStride, GroundStations: 1, Fleet: true}.Source()
	_, legacy := SatelliteNetwork{Planes: 2, Satellites: fleetUnitStride, GroundStations: 1}.Source()
	if wider.Satellites != 8*small.Satellites || wider.Definitions != small.Definitions {
		t.Errorf("more satellites per plane changed the blocks: %+v vs %+v", wider, small)
	}
	perUnit := wide.Elements - small.Elements
	if wide.Units-small.Units != 2 || wider.Units != 8*small.Units || wider.Elements-small.Elements != (wider.Units-small.Units)*perUnit/2 {
		t.Errorf("elements do not grow only with the diverging units: %+v, %+v vs %+v", wider, wide, small)
	}
	if tall.Definitions != 4 || tall.Connections <= small.Connections {
		t.Errorf("more planes add no blocks or links: %+v vs %+v", tall, small)
	}
	if legacy.Satellites != small.Satellites || legacy.Elements <= 4*small.Elements {
		t.Errorf("the fleet form is not much smaller than one definition per satellite: %+v vs %+v", small, legacy)
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
	if stats.Satellites != whole.Satellites || stats.Assertions != whole.Assertions || stats.Connections != whole.Connections {
		t.Fatalf("split stats %+v, single-file stats %+v", stats, whole)
	}
	validateFiles(t, files, stats.Assertions)
}

// TestFleetFilesValidate keeps the split fleet form in step with the whole one:
// the library and the constellation as two documents declare the same network,
// load clean under strict conformance, and hold every satisfy assertion.
func TestFleetFilesValidate(t *testing.T) {
	n := SatelliteNetwork{Planes: 2, Satellites: 2 * fleetUnitStride, GroundStations: 1, Fleet: true}
	files, stats := n.Split()
	if len(files) != 2 {
		t.Fatalf("got %d files, want the library and the constellation", len(files))
	}
	_, whole := n.Source()
	if stats.Elements != whole.Elements || stats.Definitions != whole.Definitions || stats.Units != whole.Units ||
		stats.Satellites != whole.Satellites || stats.Assertions != whole.Assertions || stats.Connections != whole.Connections {
		t.Fatalf("split stats %+v, single-file stats %+v", stats, whole)
	}
	validateFiles(t, files, stats.Assertions)
}

// validateFiles opens files as one workspace and one session, wanting no
// diagnostic and the given number of satisfy assertions, all holding.
func validateFiles(t *testing.T, files []File, assertions int) {
	t.Helper()
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
	if len(verdicts) != assertions {
		t.Fatalf("got %d satisfy verdicts, want %d", len(verdicts), assertions)
	}
	for _, v := range verdicts {
		if !v.Holds() {
			t.Errorf("%s: %v", v.Subject, v.Lines)
		}
	}
}
