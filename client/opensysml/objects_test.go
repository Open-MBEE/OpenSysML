package opensysml_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const objectQuerySource = `package Garage {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Wheel {
		attribute pressure : Integer default 30;
		assert constraint inflated { pressure >= 25 }
	}
	part def Car {
		attribute mass : Integer = 1200;
		part wheels : Wheel[2];
	}
	part car : Car;
	part spare : Wheel {
		attribute :>> pressure = 20;
	}

	calc def Parts :> Query {
		in root : Element;
		Project(source = OwnedElements(source = root), properties = ("name", "pressure"))
	}
	calc def Drive :> Query {
		in root : Element;
		Project(source = root, properties = ("mass", "wheels"))
	}
	calc def Pressure :> Query {
		in root : Element;
		Project(source = root, properties = ("name", "pressure"))
	}
	calc def Wheels :> Query {
		Project(source = Objects(type = "Wheel"), properties = ("pressure"))
	}
	calc def Checks :> Query {
		in root : Element;
		Project(source = Verdicts(source = root), properties = ("path", "verdict"))
	}
}`

func holdCar(t *testing.T, client opensysml.Client, model *opensysml.Model) *opensysml.Instantiation {
	t.Helper()
	held, err := client.Instantiate(context.Background(), model, "Garage::car")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return held
}

func TestRunDocumentQueryBindsAHeldObjectByID(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, objectQuerySource)
	held := holdCar(t, client, model)

	rows, err := client.RunDocumentQuery(context.Background(), model, "Garage::Parts",
		opensysml.Bind("root", opensysml.ObjectByID(held.Root.ID)))
	if err != nil {
		t.Fatalf("RunDocumentQuery: %v", err)
	}
	if strings.Join(rows.Columns, ",") != "name,pressure" {
		t.Errorf("columns = %v, want name, pressure", rows.Columns)
	}
	if len(rows.Rows) != 2 {
		t.Fatalf("rows = %d, want the car's two wheels", len(rows.Rows))
	}
	for i, row := range rows.Rows {
		if row.Object == nil {
			t.Fatalf("row %d is not an object row: %+v", i, row)
		}
		wantPath := "#1.wheels[" + string(rune('1'+i)) + "]"
		if row.Object.Path != wantPath || row.Object.ID == 0 || row.Object.ID == held.Root.ID {
			t.Errorf("row %d object = %+v, want path %s and an id of its own", i, *row.Object, wantPath)
		}
		want := opensysml.Element{ID: "Garage::Car::wheels", Type: "PartUsage"}
		if row.Object.Element != want || row.Element != want {
			t.Errorf("row %d element = %+v / %+v, want %+v", i, row.Object.Element, row.Element, want)
		}
		if got := opensysml.CellText(row.Cells[1][0]); got != "30" {
			t.Errorf("row %d pressure = %q, want 30", i, got)
		}
	}
}

func TestRunDocumentQueryBindsAHeldObjectByPath(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, objectQuerySource)
	holdCar(t, client, model)

	rows, err := client.RunDocumentQuery(context.Background(), model, "Garage::Drive",
		opensysml.Bind("root", opensysml.ObjectByPath("car")))
	if err != nil {
		t.Fatalf("RunDocumentQuery: %v", err)
	}
	if len(rows.Rows) != 1 {
		t.Fatalf("rows = %d, want the bound car itself", len(rows.Rows))
	}
	row := rows.Rows[0]
	want := opensysml.Object{ID: 1, Path: "Garage::car", Element: opensysml.Element{ID: "Garage::car", Type: "PartUsage"}}
	if row.Object == nil || *row.Object != want {
		t.Errorf("row object = %+v, want %+v", row.Object, want)
	}
	if got := opensysml.CellText(row.Cells[0][0]); got != "1200" {
		t.Errorf("mass = %q, want 1200", got)
	}
	var wheels []string
	for _, cell := range row.Cells[1] {
		wheel, ok := cell.(opensysml.Object)
		if !ok {
			t.Fatalf("wheels cell %#v is not an object", cell)
		}
		wheels = append(wheels, wheel.String())
	}
	if want := []string{"Garage::car.wheels[1]", "Garage::car.wheels[2]"}; !reflect.DeepEqual(wheels, want) {
		t.Errorf("wheels = %v, want %v", wheels, want)
	}

	nested, err := client.RunDocumentQuery(context.Background(), model, "Garage::Pressure",
		opensysml.Bind("root", opensysml.ObjectByPath("car.wheels[2]")))
	if err != nil {
		t.Fatalf("RunDocumentQuery by nested path: %v", err)
	}
	if len(nested.Rows) != 1 || nested.Rows[0].Object == nil || nested.Rows[0].Object.Path != "Garage::car.wheels[2]" {
		t.Fatalf("rows = %+v, want the second wheel", nested.Rows)
	}
	if got := opensysml.CellText(nested.Rows[0].Cells[0][0]) + "=" + opensysml.CellText(nested.Rows[0].Cells[1][0]); got != "wheels[2]=30" {
		t.Errorf("wheel cells = %q, want wheels[2]=30", got)
	}
}

func TestRunDocumentQueryEnumeratesHeldObjectsAndTheirVerdicts(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, objectQuerySource)
	holdCar(t, client, model)
	if _, err := client.Instantiate(context.Background(), model, "Garage::spare"); err != nil {
		t.Fatalf("Instantiate spare: %v", err)
	}

	rows, err := client.RunDocumentQuery(context.Background(), model, "Garage::Wheels")
	if err != nil {
		t.Fatalf("RunDocumentQuery: %v", err)
	}
	var wheels []string
	for _, row := range rows.Rows {
		if row.Object == nil {
			t.Fatalf("row %+v is not an object row", row)
		}
		wheels = append(wheels, row.Object.String()+"="+opensysml.CellText(row.Cells[0][0]))
	}
	// Held roots first, in label order, then the objects they hold.
	want := []string{"Garage::spare=20", "Garage::car.wheels[1]=30", "Garage::car.wheels[2]=30"}
	if !reflect.DeepEqual(wheels, want) {
		t.Errorf("Objects rows = %v, want %v", wheels, want)
	}

	checks, err := client.RunDocumentQuery(context.Background(), model, "Garage::Checks",
		opensysml.Bind("root", opensysml.ObjectByPath("spare")))
	if err != nil {
		t.Fatalf("RunDocumentQuery Checks: %v", err)
	}
	if len(checks.Rows) != 1 || checks.Rows[0].Verdict == nil {
		t.Fatalf("rows = %+v, want the spare's one verdict", checks.Rows)
	}
	if got := checks.Rows[0].Verdict.String(); got != "assert constraint inflated on Garage::spare: violated" {
		t.Errorf("verdict = %q", got)
	}
}

func TestAnObjectBindingNamesWhatItCannotReach(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, objectQuerySource)

	_, err := client.RunDocumentQuery(context.Background(), model, "Garage::Parts",
		opensysml.Bind("root", opensysml.ObjectByID(1)))
	if !errors.Is(err, opensysml.CodeNotFound) || !strings.Contains(err.Error(), "holds no objects") {
		t.Errorf("nothing held: err = %v, want CodeNotFound saying the model holds no objects", err)
	}

	holdCar(t, client, model)
	for _, tc := range []struct {
		object opensysml.Object
		code   opensysml.Code
		text   string
	}{
		{opensysml.ObjectByID(99), opensysml.CodeNotFound, "no object #99"},
		{opensysml.ObjectByPath("car.hood"), opensysml.CodeInvalidArgument, `no feature "hood"`},
		{opensysml.ObjectByPath("car.mass"), opensysml.CodeInvalidArgument, "not an object"},
		{opensysml.ObjectByPath("car..wheels"), opensysml.CodeInvalidArgument, "not an object reference"},
		{opensysml.ObjectByPath("spare"), opensysml.CodeNotFound, `no instance of "Garage::spare"`},
		{opensysml.Object{ID: 2, Path: "car"}, opensysml.CodeInvalidArgument, "is object #1, not #2"},
	} {
		_, err := client.RunDocumentQuery(context.Background(), model, "Garage::Parts", opensysml.Bind("root", tc.object))
		if !errors.Is(err, tc.code) || !strings.Contains(err.Error(), tc.text) {
			t.Errorf("%v: err = %v, want %v containing %q", tc.object, err, tc.code, tc.text)
		}
	}

	_, err = client.RunDocumentQuery(context.Background(), model, "Garage::Parts", opensysml.Bind("root", opensysml.Object{}))
	if !errors.Is(err, opensysml.CodeInvalidArgument) {
		t.Errorf("empty object: err = %v, want CodeInvalidArgument", err)
	}
}

func TestObjectStringNamesTheObjectAsASessionDoes(t *testing.T) {
	if got := opensysml.ObjectByID(2).String(); got != "#2" {
		t.Errorf("by id = %q, want #2", got)
	}
	if got := opensysml.ObjectByPath("car.wheels[2]").String(); got != "car.wheels[2]" {
		t.Errorf("by path = %q", got)
	}
	if got := opensysml.CellText(opensysml.Object{ID: 3, Path: "Garage::car.wheels[2]"}); got != "Garage::car.wheels[2]" {
		t.Errorf("CellText = %q", got)
	}
}
