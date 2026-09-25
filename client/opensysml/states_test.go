package opensysml_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const stateQuerySource = `package Lamps {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	attribute def Toggle;
	state def LampMachine {
		entry; then off;
		state off;
		transition off_on first off accept Toggle then on;
		state on parallel {
			state light {
				entry; then run;
				state run;
			}
			state fan {
				entry; then slow;
				state slow;
			}
		}
	}
	part def Lamp { exhibit state lp : LampMachine; }
	part lamp : Lamp;

	calc def CurrentStates :> Query {
		in root : Element;
		Project(source = States(source = root), properties = ("machine", "statePath", "region"))
	}
	calc def Off :> Query {
		Project(source = InState(name = "off"), properties = ("qualifiedName"))
	}
	calc def Steps :> Query {
		in root : Element;
		Project(source = Events(source = root, kind = "entry"), properties = ("time", "state"))
	}
}`

func TestRunDocumentQueryAnswersStateAndEventRows(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, stateQuerySource)
	if _, err := client.Instantiate(context.Background(), model, "Lamps::lamp"); err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	lamp := opensysml.Bind("root", opensysml.ObjectByPath("Lamps::lamp"))

	states, err := client.RunDocumentQuery(context.Background(), model, "Lamps::CurrentStates", lamp)
	if err != nil {
		t.Fatalf("RunDocumentQuery CurrentStates: %v", err)
	}
	if len(states.Rows) != 1 || states.Rows[0].State == nil || states.Rows[0].Object == nil {
		t.Fatalf("rows = %+v, want the lamp's one state row", states.Rows)
	}
	state := *states.Rows[0].State
	want := opensysml.DocumentState{
		Object:  opensysml.Object{ID: 1, Path: "Lamps::lamp", Element: opensysml.Element{ID: "Lamps::lamp", Type: "PartUsage"}},
		Machine: "lp", Name: "off", Path: "off",
		State: opensysml.Element{ID: "Lamps::LampMachine::off", Type: "StateUsage"},
	}
	if !reflect.DeepEqual(state, want) {
		t.Errorf("state = %+v, want %+v", state, want)
	}
	if got := state.String(); got != "Lamps::lamp.lp in off" {
		t.Errorf("state String() = %q", got)
	}
	if got := states.Rows[0].Element; got != want.Object.Element {
		t.Errorf("row element = %+v, want the lamp's usage", got)
	}

	off, err := client.RunDocumentQuery(context.Background(), model, "Lamps::Off")
	if err != nil {
		t.Fatalf("RunDocumentQuery Off: %v", err)
	}
	if len(off.Rows) != 1 || off.Rows[0].Object == nil || off.Rows[0].Object.String() != "Lamps::lamp" {
		t.Fatalf("InState rows = %+v, want the lamp as an object row", off.Rows)
	}

	steps, err := client.RunDocumentQuery(context.Background(), model, "Lamps::Steps", lamp)
	if err != nil {
		t.Fatalf("RunDocumentQuery Steps: %v", err)
	}
	if len(steps.Rows) != 1 || steps.Rows[0].Event == nil {
		t.Fatalf("rows = %+v, want the one entry record", steps.Rows)
	}
	event := *steps.Rows[0].Event
	if event.Kind != "entry" || event.State != "off" || event.Machine != "lp" || event.Text != "enter: off" {
		t.Errorf("event = %+v", event)
	}
	if event.Object == nil || event.Object.String() != "Lamps::lamp" || steps.Rows[0].Object == nil {
		t.Errorf("event object = %+v, want the lamp", event.Object)
	}
	at, ok := event.Time.(opensysml.Quantity)
	if !ok || at.Magnitude != opensysml.Real(0) || at.Unit != "s" {
		t.Errorf("event time = %#v, want 0 [s]", event.Time)
	}
	if got := event.String(); got != "0 s: enter: off" {
		t.Errorf("event String() = %q", got)
	}
	if cell := steps.Rows[0].Cells[0][0]; !reflect.DeepEqual(cell, event.Time) {
		t.Errorf("time cell = %#v, want the event's time", cell)
	}
}

func TestStateAndEventRowsAreNotBound(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, stateQuerySource)
	for _, cell := range []opensysml.Cell{opensysml.DocumentState{}, opensysml.DocumentEvent{}} {
		_, err := client.RunDocumentQuery(context.Background(), model, "Lamps::CurrentStates", opensysml.Bind("root", cell))
		if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), "answered by queries, not bound to them") {
			t.Errorf("%T: err = %v, want CodeInvalidArgument refusing the binding", cell, err)
		}
	}
}
