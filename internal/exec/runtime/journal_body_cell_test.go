package runtime

import "testing"

// A journalled write from outside to a feature a machine's body cell mirrors is
// undone whole by a rollback: the machine's data holds what it held, as the
// feature does.
func TestJournalledOutsideWriteToMirroredCellRollsBack(t *testing.T) {
	src := `
	package T {
		private import ScalarValues::*;
		state def Life {
			attribute flag : Boolean = false;
			entry; then work;
			state work;
		}
		part def Host { exhibit state life : Life; }
		part host : Host;
	}`
	ctx, idx := libraryShapeContext(t, src)
	root := idx.DocumentRoot("<test>")
	hostValue, err := evalIn(t, ctx, root, "T::host")
	if err != nil {
		t.Fatalf("T::host: %v", err)
	}
	host, _ := ctx.Instance(hostValue.Instance)
	life := instanceAtPath(t, ctx, host, "life")
	exec := objectMachine(t, host, "life")
	trueValue, err := evalIn(t, ctx, root, "true")
	if err != nil {
		t.Fatalf("true: %v", err)
	}
	_, rollback := ctx.beginJournal()
	if err := life.SetFeatureValue(ctx, "flag", trueValue); err != nil {
		t.Fatalf("write flag: %v", err)
	}
	if got := exec.StateData()["flag"]; !got.Const.Bool {
		t.Fatalf("flag = %s during the journal, want true: the data mirrors the write", FormatValue(got))
	}
	rollback()
	if got := exec.StateData()["flag"]; got.Const.Bool {
		t.Errorf("flag = %s after rollback, want false: the data holds what it held", FormatValue(got))
	}
	fv, err := life.GetFeatureValue(ctx, "flag")
	if err != nil {
		t.Fatalf("read flag: %v", err)
	}
	if fv.HeldValue().Const.Bool {
		t.Errorf("feature flag = %s after rollback, want false", FormatValue(fv.HeldValue()))
	}
}

// A body binding reading a feature written from outside re-derives from the write:
// `d = flag` reads true once another performance wrote `flag`, and the machine's
// data agrees with the occurrence (the binding kept its stale stand-in before).
func TestBodyBindingRederivesFromAnOutsideWrite(t *testing.T) {
	src := `
	package T {
		private import ScalarValues::*;
		state def Life {
			attribute flag : Boolean[1];
			attribute d : Boolean = flag;
			entry; then work;
			state work;
		}
		part def Host { exhibit state life : Life; }
		part host : Host;
	}`
	ctx, idx := libraryShapeContext(t, src)
	root := idx.DocumentRoot("<test>")
	hostValue, err := evalIn(t, ctx, root, "T::host")
	if err != nil {
		t.Fatalf("T::host: %v", err)
	}
	host, _ := ctx.Instance(hostValue.Instance)
	life := instanceAtPath(t, ctx, host, "life")
	exec := objectMachine(t, host, "life")
	trueValue, err := evalIn(t, ctx, root, "true")
	if err != nil {
		t.Fatalf("true: %v", err)
	}
	if err := life.SetFeatureValue(ctx, "flag", trueValue); err != nil {
		t.Fatalf("write flag: %v", err)
	}
	if got := exec.StateData()["d"]; got.Kind != ValConst || !got.Const.Bool {
		t.Errorf("data d = %s after the outside write of flag, want true", FormatValue(got))
	}
	fv, err := life.GetFeatureValue(ctx, "d")
	if err != nil {
		t.Fatalf("read d: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || !got.Const.Bool {
		t.Errorf("feature d = %s after the outside write of flag, want true", FormatValue(got))
	}
}
