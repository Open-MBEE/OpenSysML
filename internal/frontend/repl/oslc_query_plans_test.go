package repl

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/doc/querytranslate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const planQueryModel = `package Garage {
    part def Wheel { attribute diameter : ScalarValues::Real; attribute pressure : ScalarValues::Integer; }
    part def Car { part wheels : Wheel[2] { attribute :>> diameter = 16.0; attribute :>> pressure = 30; } }
    part car : Car;
    part spare : Wheel { attribute :>> diameter = 15.0; attribute :>> pressure = 28; }
}`

// TestQuerySelectsHeldObjectsByPath: rdf:type=elmt:<id> answers the objects of
// that element, each by its path, with its features as sysx: properties, in
// the order asked; an unknown feature names the nearest.
func TestQuerySelectsHeldObjectsByPath(t *testing.T) {
	s := NewSession()
	if result := s.Submit(planQueryModel); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	run(t, s, "%instantiate Garage::car")
	run(t, s, "%instantiate Garage::spare")
	wheel := "rdf:type=elmt:" + rdf.EncodeElementID("Garage::Wheel")
	lines, err := s.Query("oslc.where=" + url.QueryEscape(wheel) + "&oslc.orderBy=-sysx:pressure&oslc.select=sysx:pressure")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Garage::car.wheels[1]  Garage::Car::wheels  PartUsage  sysx:pressure=30",
		"Garage::car.wheels[2]  Garage::Car::wheels  PartUsage  sysx:pressure=30",
		"Garage::spare  Garage::spare  PartUsage  sysx:pressure=28",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("objects =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	// Declared elements are still what a metaclass selects.
	lines, err = s.Query(`oslc.where=rdf:type=sysml:PartUsage and sysml:name="spare"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "Garage::spare  PartUsage" {
		t.Fatalf("elements = %v", lines)
	}
	_, err = s.Query("oslc.where=" + url.QueryEscape(wheel) + "&oslc.select=sysx:pressur")
	var failure *queryexec.Error
	if !errors.As(err, &failure) || failure.Kind != queryexec.ErrorUnknownProperty || failure.Outcome == nil {
		t.Fatalf("unknown feature: %v", err)
	}
	if !strings.Contains(err.Error(), "did you mean pressure") {
		t.Fatalf("unknown feature = %v, want the nearest feature", err)
	}
	// A metamodel property the OSLC grammar misspells is refused before the plan runs.
	_, err = s.Query(`oslc.where=sysml:nam="spare"`)
	var translation *querytranslate.Error
	if !errors.As(err, &translation) || translation.Kind != querytranslate.ErrUnknownProperty || !strings.Contains(err.Error(), "sysml:name") {
		t.Fatalf("misspelt property: %v", err)
	}
	_, err = s.Query(`oslc.where=sysml:name="a" or sysml:name="b"`)
	if !errors.As(err, &translation) || translation.Kind != querytranslate.ErrUnsupportedDisjunction {
		t.Fatalf("or: %v", err)
	}
}

// TestQueryReadsASendsArgumentAsWritten: a send's argument is an expression
// element with no identity, so %query prints it as written, and a path through
// it reaches what it refers to.
func TestQueryReadsASendsArgumentAsWritten(t *testing.T) {
	s := NewSession()
	if result := s.Submit(`package Demo {
		item def Alarm { attribute level : ScalarValues::Integer; }
		part def Monitor {
			port alarmPort;
			action supervise { action notifyVia send new Alarm(3) via alarmPort; }
		}
	}`); len(result.Diagnostics) != 0 {
		t.Fatalf("Submit diagnostics = %v", result.Diagnostics)
	}
	lines, err := s.Query(`oslc.where=rdf:type=sysml:SendActionUsage&oslc.select=sysml:payloadArgument,sysml:senderArgument{sysml:referent{sysml:name}}`)
	if err != nil {
		t.Fatal(err)
	}
	want := "Demo::Monitor::supervise::notifyVia  SendActionUsage  sysml:payloadArgument=new Alarm(3)  sysml:senderArgument{sysml:referent{sysml:name}}=alarmPort"
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("send =\n%s\nwant\n%s", strings.Join(lines, "\n"), want)
	}
}
