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
