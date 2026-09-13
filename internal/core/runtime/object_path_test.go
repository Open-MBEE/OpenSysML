package runtime

import (
	"errors"
	"strings"
	"testing"
)

// pathModel materializes roots of one type, a held part and a collection of parts.
const pathModel = `package Plant {
	part def Tank;
	part def Pump { part valve : Tank; }
	part def Site { part tanks : Tank[2]; part pump : Pump; }
	part tank : Tank;
	part spare : Tank;
	part site : Site;
}`

// Every object's materialization path leads back to it, and spells the same
// whatever number the run gave the object: roots by type and creation rank, a
// held object by its holder's path and feature, indexed within a collection.
func TestObjectPathsRoundTrip(t *testing.T) {
	ctx, idx := contextForSource(t, pathModel)
	var ids []int64
	for _, name := range []string{"Plant::tank", "Plant::spare", "Plant::site"} {
		inst, err := ctx.Instantiate(lookupOne(t, idx, name))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, inst.ID)
	}
	site, _ := ctx.Instance(ids[2])
	held := func(inst *Instance, feature string) *FeatureValue {
		fv, err := inst.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Fatal(err)
		}
		return fv
	}
	tanks := elementsOf(held(site, "tanks").Values)
	pump := held(site, "pump").Value
	if len(tanks) != 2 || pump.Kind != ValInstance {
		t.Fatalf("site holds tanks %v, pump %s", tanks, pump.Kind)
	}
	pumpInst, _ := ctx.Instance(pump.Instance)
	valve := held(pumpInst, "valve").Value
	want := map[int64]string{
		ids[0]:            "Plant::tank#1",
		ids[1]:            "Plant::spare#1",
		ids[2]:            "Plant::site#1",
		tanks[0].Instance: "Plant::site#1.tanks[0]",
		tanks[1].Instance: "Plant::site#1.tanks[1]",
		pump.Instance:     "Plant::site#1.pump",
		valve.Instance:    "Plant::site#1.pump.valve",
	}
	for id, path := range want {
		if got := ctx.objectPath(id); got != path {
			t.Errorf("object #%d: path %q, want %q", id, got, path)
		}
		inst, err := ctx.objectAt(path)
		if err != nil || inst.ID != id {
			t.Errorf("%s: leads to %v, %v, want object #%d", path, inst, err, id)
		}
	}
	if got := ctx.objectPath(999); got != "<unknown object>" {
		t.Errorf("an unknown identity spells as %q", got)
	}
	for _, c := range []struct{ path, reason string }{
		{"Plant::tank", "names no root"},
		{"Plant::tank#0", "names no root"},
		{"Plant::tank#2", "the run made no Plant::tank#2"},
		{"Plant::site#1.tanks", "holds a collection; index it"},
		{"Plant::site#1.tanks[2]", "tanks holds 2 values"},
		{"Plant::site#1.tanks[x]", "indexes no collection"},
		{"Plant::site#1.pump[0]", "holds one value, not a collection"},
		{"Plant::site#1.silo", "has no feature silo"},
	} {
		if _, err := ctx.objectAt(c.path); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: %v, want %q", c.path, err, c.reason)
		}
	}
}

// A witness names its objects by number bound to their path, one line each ahead
// of the inputs and moves; a run reading it back gets the same objects, and a
// malformed, repeated or late object line is refused naming its line.
func TestParseWitnessReadsObjectsBeforeInputsAndChoices(t *testing.T) {
	text := "object #1 = Plant::tank#1\nobject #3 = Plant::site#1.tanks[0]\ninput n = 5\n" +
		"t=0.0: action fill of object #3 first of action fill of object #1, action fill of object #3\n\ntrace\n"
	w, err := ParseWitness(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Objects) != 2 || w.Objects[1] != (ObjectNamed{ID: 3, Path: "Plant::site#1.tanks[0]"}) || len(w.Inputs) != 1 || len(w.Choices) != 1 {
		t.Fatalf("read %+v", w)
	}
	if w.String() != text {
		t.Errorf("spelt back as\n%s", w)
	}
	if o := w.Objects[0]; o.Number() != "object #1" || o.String() != "object #1 = Plant::tank#1" {
		t.Errorf("object spelt as %q, %q", o.Number(), o)
	}
	for _, c := range []struct{ text, reason string }{
		{"object #1 = Plant::tank#1\nobject #1 = Plant::spare#1\n", "bound twice"},
		{"input n = 5\nobject #1 = Plant::tank#1\n", "before the inputs and the moves"},
		{"step 1: 2@b first of 1@a, 2@b\nobject #1 = Plant::tank#1\n", "before the inputs and the moves"},
		{"object #0 = Plant::tank#1\n", "numbered from 1"},
		{"object #1 Plant::tank#1\n", "bound with"},
		{"object #1 = \n", "bound with"},
	} {
		_, err := ParseWitness(c.text)
		var typed *ObjectParseError
		if !errors.As(err, &typed) || !errors.Is(err, ErrInvalidObject) || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%q: %v, want an ObjectParseError %q", c.text, err, c.reason)
		}
	}
	if _, err := ParseChoices("object #1 = Plant::tank#1\nstep 1: 2@b first of 1@a, 2@b\n"); !errors.Is(err, ErrInvalidObject) {
		t.Errorf("ParseChoices over objects: %v", err)
	}
}
