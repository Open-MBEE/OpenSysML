package repl

import (
	"encoding/json"
	"strings"
	"testing"
)

// The reported shape: a context of twelve stations whose counters sit two levels
// down. The default listing is bounded and says how to lift the bound; `all`
// lifts it and reads out the whole tree, counters included.
func TestFeaturesAllListsTheWholeTree(t *testing.T) {
	s := loadFixture(t, "testdata/large_instance_tree.sysml")
	run(t, s, "%instantiate Plant::Context")

	bounded := run(t, s, "%features Plant::Context")
	wants(t, bounded, "… (listing truncated; %features Plant::Context all shows it whole")
	if n := strings.Count(bounded, "\n"); n > maxFeatureValueLines+10 {
		t.Errorf("default listing ran to %d lines, want it bounded near %d", n, maxFeatureValueLines)
	}

	whole := run(t, s, "%features Plant::Context all")
	rejects(t, whole, "listing truncated", "not expanded")
	if n := strings.Count(whole, "scrapped = 0"); n != 24 {
		t.Errorf("counters read out %d times, want one per infeed and outfeed of twelve stations:\n%.400s", n, whole)
	}
}

// A depth asked for bounds nesting at that depth and nothing else: everything
// above it is listed, and what is below is named as unexpanded rather than cut.
func TestFeaturesDepthBoundsNestingOnly(t *testing.T) {
	s := loadFixture(t, "testdata/large_instance_tree.sysml")
	run(t, s, "%instantiate Plant::Context")

	got := run(t, s, "%features Plant::Context depth 1")
	wants(t, got, `name = "station"`, "machine : Machine (not expanded: depth 1)")
	rejects(t, got, "listing truncated", "scrapped = 0")
	if n := strings.Count(got, `name = "station"`); n != 12 {
		t.Errorf("listed %d stations, want all 12 within the depth asked for:\n%.400s", n, got)
	}

	// Depth 0 is the object's own features, with nothing expanded under them.
	shallow := run(t, s, "%features Plant::Context depth 0")
	wants(t, shallow, "stations : Station (not expanded: depth 0)")
	rejects(t, shallow, `name = "station"`)
}

// The whole graph is available as the JSON the API's Instantiate returns, so a
// client reads one shape whether it asked the service or the REPL.
func TestFeaturesJSONMatchesTheAPIShape(t *testing.T) {
	s := loadFixture(t, "testdata/large_instance_tree.sysml")
	run(t, s, "%instantiate Plant::Context")

	var resp struct {
		Instance struct {
			ID            string                     `json:"id"`
			TypeSymbolID  string                     `json:"typeSymbolId"`
			FeatureValues map[string]json.RawMessage `json:"featureValues"`
		} `json:"instance"`
		Instances []struct {
			ID            string `json:"id"`
			FeatureValues map[string]struct {
				FeatureName string `json:"featureName"`
			} `json:"featureValues"`
		} `json:"instances"`
		Diagnostics []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	got := run(t, s, "%features Plant::Context all json")
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("json listing does not parse: %v\n%.400s", err, got)
	}
	if resp.Instance.TypeSymbolID != "Plant::Context" {
		t.Errorf("instance.typeSymbolId = %q, want the object's type", resp.Instance.TypeSymbolID)
	}
	if _, ok := resp.Instance.FeatureValues["stations"]; !ok {
		t.Errorf("root instance carries no stations feature value:\n%.400s", got)
	}
	// Root, twelve stations, a machine each and two counter parts per machine.
	if len(resp.Instances) != 1+12*4 {
		t.Errorf("instances = %d, want the whole graph (49)", len(resp.Instances))
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none for a graph that fit", resp.Diagnostics)
	}
	var counters int
	for _, inst := range resp.Instances {
		if _, ok := inst.FeatureValues["scrapped"]; ok {
			counters++
		}
	}
	if counters != 24 {
		t.Errorf("counter objects = %d, want one per infeed and outfeed of twelve stations", counters)
	}
}

// A JSON graph the object bound cuts short reports it as a warning diagnostic,
// since what came back is a real answer about part of the run.
func TestFeaturesJSONReportsTruncation(t *testing.T) {
	s := NewSession()
	s.Submit("part def Leaf { attribute v = 1.0; } part def Mid { part leaves : Leaf[40]; } part def Top { part mids : Mid[40]; }")
	run(t, s, "%instantiate Top")

	var resp struct {
		Instances   []json.RawMessage `json:"instances"`
		Diagnostics []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	got := run(t, s, "%features Top json")
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("json listing does not parse: %v\n%.400s", err, got)
	}
	if len(resp.Instances) > maxFeatureGraphInstances {
		t.Errorf("instances = %d, want the default graph bound of %d", len(resp.Instances), maxFeatureGraphInstances)
	}
	if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Severity != "warning" ||
		!strings.Contains(resp.Diagnostics[0].Message, "%features Top all") {
		t.Errorf("diagnostics = %v, want one warning saying how to see the whole graph", resp.Diagnostics)
	}
}

// Options the listing cannot honor are reported with the usage rather than
// silently taken as a default or read as an object name.
func TestFeaturesRejectsBadOptions(t *testing.T) {
	s := loadFixture(t, "testdata/large_instance_tree.sysml")
	run(t, s, "%instantiate Plant::Context")

	for _, tc := range []struct{ line, want string }{
		{"%features Plant::Context depth", "depth needs a number"},
		{"%features Plant::Context depth x", `depth "x" is not a non-negative integer`},
		{"%features Plant::Context depth -1", `depth "-1" is not a non-negative integer`},
		{"%features Plant::Context all depth 2", "all and depth <n> cannot both be given"},
		{"%features Plant::Context depth 2 all", "all and depth <n> cannot both be given"},
		{"%features Plant::Context json json", "json is given twice"},
		{"%features Plant::Context deep", `unknown option "deep"`},
	} {
		got := run(t, s, tc.line)
		wants(t, got, tc.want, featuresUsage)
		rejects(t, got, "Features:")
	}
}
