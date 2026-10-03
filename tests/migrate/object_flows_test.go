package migrate_test

import (
	"strings"
	"testing"
)

// Each object flow of the object_flows fixture is written as a succession
// flow of its payload type, through the object features of the fork, join and
// merge nodes it crosses, except the one into a streaming parameter; every
// activity runs and delivers the values its flows carry.
func TestObjectFlowsAreSuccessionFlowsThroughControlNodeObjects(t *testing.T) {
	r := migrateFixtureFile(t, "object_flows")
	notation := string(r.Notation)
	for _, want := range []string{
		"succession flow of ScalarValues::Real from produce.y to consume.v;",
		"succession flow of ScalarValues::Real from produce.y to left.v;\n        succession flow of ScalarValues::Real from produce.y to right.v;",
		"out ref outputObject2 : ScalarValues::Real = inputObject1;",
		"out ref outputObject1 : ScalarValues::Real nonunique = (inputObject1, inputObject2);",
		"succession flow of ScalarValues::Real from pick.outputObject1 to consume.v;",
		"first produce if on then consume;",
		"flow produce.y to feed.v;",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("notation lacks %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "if true") {
		t.Errorf("a guard of literal true is written:\n%s", notation)
	}
	s := session(t, r)
	meta(t, s, "%instantiate Pipeline")
	for action, want := range map[string]map[string]string{
		"Pipeline::relay":  {"consume.v": "2.5"},
		"Pipeline::gate":   {"consume.v": "4.0"},
		"Pipeline::spread": {"left.v": "3.0", "right.v": "3.0"},
		"Pipeline::fan":    {"left.v": "5.0", "right.v": "5.0"},
		"Pipeline::hold":   {"consume.v": "7.0"},
		"Pipeline::either": {"consume.v": "9.0"},
		"Pipeline::count":  {"step.x": "2", "step.y": "3"},
		"Pipeline::Pair":   {"keep.v": "[1.5, 8.5]"},
		"Pipeline::Stream": {"feed.v": "6.0"},
	} {
		t.Run(action, func(t *testing.T) {
			wantValues(t, runValues(t, s, action, "#1"), want)
		})
	}
}
