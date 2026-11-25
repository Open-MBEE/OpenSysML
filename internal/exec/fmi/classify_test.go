package fmi

import "testing"

// TestClassifyGivesFMUNamesFirstClaim pins the one classifier every surface
// shares: an exact archive name wins over a reserved selector spelling.
func TestClassifyGivesFMUNamesFirstClaim(t *testing.T) {
	d := &Description{Variables: []Variable{
		{Name: "fmi:series:y", Causality: CausalityOutput},
		{Name: "y", Causality: CausalityOutput},
		{Name: "fmi:startTime", Causality: CausalityParameter},
	}}
	for name, want := range map[string]struct {
		kind      outputKind
		seriesVar string
	}{
		"fmi:series:y":   {fmuOutput, ""},
		"y":              {fmuOutput, ""},
		"fmi:series:z":   {sampleSeries, "z"},
		"fmi:seriesTime": {sampleTimes, ""},
		"fmi:time":       {replyTime, ""},
		"missing":        {unknownOutput, ""},
	} {
		t.Run(name, func(t *testing.T) {
			kind, variable, seriesVar := classifyOutput(name, d)
			if kind != want.kind || seriesVar != want.seriesVar {
				t.Fatalf("classifyOutput(%q) = (%v, %q), want (%v, %q)", name, kind, seriesVar, want.kind, want.seriesVar)
			}
			if want.kind == fmuOutput && variable.Name != name {
				t.Fatalf("classifyOutput(%q) resolved %q", name, variable.Name)
			}
		})
	}
	for name, want := range map[string]inputKind{
		"fmi:startTime": fmuInput,
		"fmi:samples":   reservedInput,
		"missing":       unknownInput,
	} {
		t.Run("in "+name, func(t *testing.T) {
			kind, _ := classifyInput(name, d)
			if kind != want {
				t.Fatalf("classifyInput(%q) = %v, want %v", name, kind, want)
			}
		})
	}
}
