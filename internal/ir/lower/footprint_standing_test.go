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

// narrowing is a meet that holds only of places p Conflicts with, the contract
// the index relies on, narrowed further by the case's predicate on q.
func narrowing(narrow func(Place) bool) func(p, q Place) bool {
	return func(p, q Place) bool { return p.Conflicts(q) && narrow(q) }
}

func TestStandingWritesReadingAgreesWithTheScan(t *testing.T) {
	x, y, guard := Place{Name: "x", Sym: &symbols.Symbol{}}, Place{Name: "y", Sym: &symbols.Symbol{}}, Place{Name: "guard"}
	a, b := Place{Name: "a", Sym: &symbols.Symbol{}}, Place{Name: "b"}
	held := Place{Name: "a", Sym: a.Sym, Local: true}
	alias := Place{Name: "y2", Sym: y.Sym}
	fp := Footprint{
		Reads:  []Place{x, y, guard},
		Writes: []Place{held, b},
		Flows: []Flow{
			{Reads: []Place{x}, Writes: []Place{a}},
			{Reads: []Place{y}, Writes: []Place{b}},
			{Reads: []Place{x, y}, Writes: []Place{a, b, {Name: ""}}},
			{Reads: []Place{{Name: "", Sym: x.Sym}}, Writes: []Place{b}},
			{Reads: []Place{alias}, Writes: []Place{a}},
		},
	}
	whole := Footprint{Reads: []Place{x, guard}, Writes: []Place{held, b}}
	all := func(Place) bool { return true }
	cases := []struct {
		name string
		fp   Footprint
		p    Place
		meet func(Place) bool
	}{
		{"nothing", fp, x, func(Place) bool { return false }},
		{"x", fp, x, all},
		{"y", fp, y, all},
		{"y by its alias", fp, alias, all},
		{"y by symbol under another name", fp, Place{Name: "other", Sym: y.Sym}, all},
		{"y by name alone", fp, Place{Name: "y"}, all},
		{"x but not its unnamed read", fp, x, func(p Place) bool { return p.Name != "" }},
		{"guard", fp, guard, all},
		{"unnamed read only", fp, Place{Name: "", Sym: x.Sym}, func(p Place) bool { return p.Name == "" }},
		{"a pin of x's", fp, Place{Name: "x", Sym: x.Sym, Local: true}, all},
		{"a stranger", fp, Place{Name: "z", Sym: &symbols.Symbol{}}, all},
		{"whole move", whole, x, all},
		{"whole move unmet", whole, y, all},
	}
	for _, tc := range cases {
		s := tc.fp.Standing()
		for range 2 {
			reads := func(q Place) bool { return tc.p.Conflicts(q) && tc.meet(q) }
			wantWrites, wantMet := writesReadingByScan(tc.fp, reads)
			writes, met := s.WritesReading(tc.p, narrowing(tc.meet))
			if met != wantMet || !slices.Equal(writes, wantWrites) {
				t.Errorf("%s: WritesReading = %v, %t; scan gives %v, %t", tc.name, writes, met, wantWrites, wantMet)
			}
			if got, want := s.MeetsRead(tc.p, narrowing(tc.meet)), slices.ContainsFunc(tc.fp.Reads, reads); got != want {
				t.Errorf("%s: MeetsRead = %t, scan gives %t", tc.name, got, want)
			}
			if got, want := s.MeetsWrite(tc.p, narrowing(tc.meet)), slices.ContainsFunc(tc.fp.Writes, reads); got != want {
				t.Errorf("%s: MeetsWrite = %t, scan gives %t", tc.name, got, want)
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
		writes, ok := s.WritesReading(r, narrowing(func(Place) bool { return true }))
		if ok != wantMet || !slices.Equal(writes, wantWrites) {
			t.Errorf("reading %s: WritesReading = %v, %t; scan gives %v, %t", r.Name, placeNames(writes), ok, placeNames(wantWrites), wantMet)
		}
	}
	writes, ok := s.WritesReading(Place{Name: "n"}, narrowing(func(Place) bool { return true }))
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
