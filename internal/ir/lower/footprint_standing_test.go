package lower

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// writesReadingByScan is WritesReading by a scan of the flows, the definition the
// index must agree with: the writes of the flows reading a place met, every write
// when none does.
func writesReadingByScan(f Footprint, reads func(Place) bool) ([]Place, bool) {
	if !slices.ContainsFunc(f.Reads, reads) {
		return nil, false
	}
	var out []Place
	inFlow := false
	for _, flow := range f.Flows {
		if slices.ContainsFunc(flow.Reads, reads) {
			inFlow = true
			for _, p := range flow.Writes {
				out = appendPlace(out, f.writeOf(p))
			}
		}
	}
	if !inFlow {
		return f.Writes, true
	}
	return out, true
}

func TestStandingWritesReadingAgreesWithTheScan(t *testing.T) {
	x, y, guard := Place{Name: "x", Sym: &symbols.Symbol{}}, Place{Name: "y", Sym: &symbols.Symbol{}}, Place{Name: "guard"}
	a, b := Place{Name: "a", Sym: &symbols.Symbol{}}, Place{Name: "b"}
	held := Place{Name: "a", Sym: a.Sym, Local: true}
	fp := Footprint{
		Reads:  []Place{x, y, guard},
		Writes: []Place{held, b},
		Flows: []Flow{
			{Reads: []Place{x}, Writes: []Place{a}},
			{Reads: []Place{y}, Writes: []Place{b}},
			{Reads: []Place{x, y}, Writes: []Place{a, b, {Name: ""}}},
			{Reads: []Place{{Name: "", Sym: x.Sym}}, Writes: []Place{b}},
		},
	}
	whole := Footprint{Reads: []Place{x, guard}, Writes: []Place{held, b}}
	cases := []struct {
		name string
		fp   Footprint
		met  func(Place) bool
	}{
		{"nothing", fp, func(Place) bool { return false }},
		{"x", fp, func(p Place) bool { return p.Name == "x" }},
		{"y", fp, func(p Place) bool { return p.Name == "y" }},
		{"x or y", fp, func(p Place) bool { return p.Name == "x" || p.Name == "y" }},
		{"guard", fp, func(p Place) bool { return p.Name == "guard" }},
		{"everything", fp, func(Place) bool { return true }},
		{"unnamed read only", fp, func(p Place) bool { return p.Name == "" }},
		{"by symbol", fp, func(p Place) bool { return p.Sym == x.Sym }},
		{"whole move", whole, func(p Place) bool { return p.Name == "x" }},
		{"whole move unmet", whole, func(p Place) bool { return p.Name == "y" }},
	}
	for _, tc := range cases {
		s := tc.fp.Standing()
		for range 2 {
			wantWrites, wantMet := writesReadingByScan(tc.fp, tc.met)
			writes, met := s.WritesReading(tc.met)
			if met != wantMet || !slices.Equal(writes, wantWrites) {
				t.Errorf("%s: WritesReading = %v, %t; scan gives %v, %t", tc.name, writes, met, wantWrites, wantMet)
			}
		}
	}
}

func TestStandingWritesReadingOnALoweredBody(t *testing.T) {
	graph := scopedActionGraph(t, `
		part def Counter { attribute count : Integer = 0; }
		action def Body {
			attribute c : Counter;
			attribute n : Integer = 0;
			attribute m : Integer = 0;
			first start;
			action step {
				assign c.count := n;
				assign m := m + 1;
			}
			done;
			succession first start then step;
			succession first step then done;
		}
	`, "Body")
	fp := footprintNamed(t, graph, "step")
	if len(fp.Flows) < 2 {
		t.Fatalf("step has %d flows, want one per statement:\n%s", len(fp.Flows), fp)
	}
	s := fp.Standing()
	for _, r := range fp.Reads {
		met := func(p Place) bool { return p.Conflicts(r) }
		wantWrites, wantMet := writesReadingByScan(fp, met)
		writes, ok := s.WritesReading(met)
		if ok != wantMet || !slices.Equal(writes, wantWrites) {
			t.Errorf("reading %s: WritesReading = %v, %t; scan gives %v, %t", r.Name, placeNames(writes), ok, placeNames(wantWrites), wantMet)
		}
	}
	writes, ok := s.WritesReading(func(p Place) bool { return p.Name == "n" })
	if !ok || !slices.Equal(placeNames(writes), []string{"c.count"}) && !slices.Equal(placeNames(writes), []string{"count"}) {
		t.Errorf("writes standing on n = %v, %t; want the count written from it", placeNames(writes), ok)
	}
}

func TestFootprintPlacesEqual(t *testing.T) {
	x, a := Place{Name: "x"}, Place{Name: "a"}
	fp := Footprint{Reads: []Place{x}, Writes: []Place{a}, Flows: []Flow{{Reads: []Place{x}, Writes: []Place{a}}}}
	same := Footprint{Reads: slices.Clone(fp.Reads), Writes: slices.Clone(fp.Writes), Flows: slices.Clone(fp.Flows), Dynamic: true}
	if !fp.PlacesEqual(same) || !fp.PlacesEqual(fp) {
		t.Errorf("footprints of the same places are not PlacesEqual")
	}
	for name, other := range map[string]Footprint{
		"reads":  {Reads: []Place{a}, Writes: fp.Writes, Flows: fp.Flows},
		"writes": {Reads: fp.Reads, Writes: []Place{x}, Flows: fp.Flows},
		"flows":  {Reads: fp.Reads, Writes: fp.Writes, Flows: []Flow{{Reads: []Place{x}, Writes: nil}}},
		"whole":  {Reads: fp.Reads, Writes: fp.Writes},
	} {
		if fp.PlacesEqual(other) {
			t.Errorf("footprints differing in %s are PlacesEqual", name)
		}
	}
}
