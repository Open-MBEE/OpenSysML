package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// Every anonymous association is a connection def named after its end types,
// with one connection typed by it joining usages of the two types. An end a
// class owns crosses the property it is written as, reached through the
// opposite end; where that is unsound the def is still written, the end
// crosses nothing and the report says why. An end the association owns
// carries its multiplicity as the cross multiplicity.
func TestAnonymousAssociationEndsCrossTheirPropertiesWhereSound(t *testing.T) {
	r := migrateFixtureFile(t, "anonymous_associations")
	wantClean(t, "anonymous_associations.sysml", r)
	for _, line := range []string{
		"connection def WheelToCar {",
		"end wheels : Wheel crosses car.wheels;",
		"end car : Car crosses wheels.car;",
		"connection 'wheel to car' : WheelToCar connect wheel to car;",
		"connection def CarToGarage {",
		"end cars : Car crosses garage.cars;",
		"end garage : Garage crosses cars.garage;",
		"ref part cars : Car;",
		"connection 'car to garage' : CarToGarage connect car to garage;",
		"connection def DriverToCar {",
		"end driver : Driver crosses car.driver;",
		"end [0..*] ref car : Car;",
		"connection def ReadingToSource {",
		"end [0..*] ref reading : Reading;",
		"end source;",
		"connection def SinkToLogger {",
		"end sink;",
		"end logger : Logger;",
		"ref part Pump : Fleet::Pump;",
		"end Pump : Fleet::Pump crosses tank.Pump;",
		"end operator :> Operator;",
		"connection operates : Operates connect Operator to console;",
		"view Wheels {",
		"expose WheelToCar;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "connection 'operates 2'")
	wantNoLine(t, r.Notation, "private ref part cars")
	if got := strings.Count(string(r.Notation), " connect "); got != 5 {
		t.Errorf("connections = %d, want one for each def whose end types have usages\n%s", got, r.Notation)
	}
	wantNote(t, r, "_carWheel", migrate.Approximated, "the anonymous Association is written as connection def WheelToCar; its one usage is the connection wheel to car joining wheel and car")
	wantNote(t, r, "_carDriverAssoc", migrate.Approximated, "its one usage is the connection driver to car joining driver and car")
	wantNote(t, r, "_garageCars", migrate.Approximated, "private visibility is not written: the connection def of the association (_garageCarAssoc) in Fleet crosses it")
	wantNote(t, r, "_sensorReadingAssoc", migrate.Approximated, "end reading crosses no property: the opposite end is untyped, so the property cannot be reached through it")
	wantNote(t, r, "_sensorReadingAssoc", migrate.Approximated, "no connection usage joins usages of its end types: end source is untyped")
	wantNote(t, r, "_loggerSinkAssoc", migrate.Approximated, "end sink crosses no property: the end's type is not written")
	wantNote(t, r, "_loggerSinkAssoc", migrate.Approximated, "no connection usage joins usages of its end types")
	wantNote(t, r, "_tankPumpAssoc", migrate.Approximated, "the anonymous Association is written as connection def PumpToTank; its one usage is the connection pump to tank joining pump and tank")
	wantNote(t, r, "_operates", migrate.Approximated, "end operator crosses no property: its type Fleet::Operator is written as a usage, not a definition")
	for _, id := range []string{"_sensorReadingAssoc", "_loggerSinkAssoc", "_tankPumpAssoc", "_operates"} {
		if es := entriesFor(r, id); len(es) != 1 || strings.Contains(es[0].Note, "member-end properties") {
			t.Errorf("%s entries = %+v", id, es)
		}
	}
}
