package migrate_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// The association backing a value property or constraint parameter is skipped
// with its owned end; the same shape backing a part stays a connection def.
func TestBackingAssociationOfValuePropertyIsSkipped(t *testing.T) {
	r := migrateStream(t, "backing_associations")
	checkGolden(t, "testdata/xmi/backing_associations.golden.sysml", r.Notation)
	var report bytes.Buffer
	if err := r.Report.WriteText(&report); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "testdata/xmi/backing_associations.golden.report.txt", report.Bytes())
	for _, d := range errors(t, "backing_associations.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	for _, line := range []string{
		"in attribute input : ScalarValues::Real[1];",
		"in attribute output : ScalarValues::Real[1];",
		"attribute mass : Mass;",
		"part amplifier : Amplifier;",
		"connection def AmplifierToSensor {",
		"end amplifier : Amplifier crosses sensor.amplifier;",
		"connection 'amplifier to sensor' : AmplifierToSensor connect amplifier to sensor;",
		"metadata DiagramLayout::Route about AmplifierToSensor { points = (320, 180, 240, 70); }",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, text := range []string{"GainToReal", "RealToGain", "MassToSensor", "SensorToMass", "input :", "output :", "mass :"} {
		if strings.Count(string(r.Notation), text) != strings.Count(string(r.Notation), "attribute "+text) {
			t.Errorf("notation names %q outside the feature\n%s", text, r.Notation)
		}
	}
	if got := strings.Count(string(r.Notation), "connection def"); got != 1 {
		t.Errorf("connection defs = %d, want only the composition's\n%s", got, r.Notation)
	}
	wantNote(t, r, "_gainInAssoc", migrate.Skipped, "the association only backs Optics::Gain::input, which is written as a feature")
	wantNote(t, r, "_gainOutAssoc", migrate.Skipped, "the association only backs Optics::Gain::output, which is written as a feature")
	wantNote(t, r, "_gainOutEnd", migrate.Skipped, "the end types the owner of Optics::Gain::output")
	wantNote(t, r, "_sensorMassAssoc", migrate.Skipped, "the association only backs Optics::Sensor::mass, which is written as a feature")
	wantNote(t, r, "_sensorAmpAssoc", migrate.Approximated, "the anonymous Association is written as connection def AmplifierToSensor")
	wantNote(t, r, "_gainIn", migrate.Mapped, "")
	wantNote(t, r, "_sensorMass", migrate.Mapped, "")
	for _, id := range []string{"_gainInAssoc", "_gainOutAssoc", "_sensorMassAssoc"} {
		if es := entriesFor(r, id); len(es) != 1 {
			t.Errorf("%s entries = %+v, want one skipped entry", id, es)
		}
	}
}
