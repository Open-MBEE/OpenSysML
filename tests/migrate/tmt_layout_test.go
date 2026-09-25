package migrate_test

import (
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
)

// TestTMTLayout runs the layout augment over the real OpenMBEE TMT pair: the
// model's .mdzip and its MTIP export. It skips unless OPENSYSML_TMT_MDZIP and
// OPENSYSML_TMT_MTIP name the files.
func TestTMTLayout(t *testing.T) {
	mdzip, mtipPath := os.Getenv("OPENSYSML_TMT_MDZIP"), os.Getenv("OPENSYSML_TMT_MTIP")
	if mdzip == "" || mtipPath == "" {
		t.Skip("OPENSYSML_TMT_MDZIP and OPENSYSML_TMT_MTIP do not name the TMT model pair")
	}
	data, err := os.ReadFile(mdzip)
	if err != nil {
		t.Fatal(err)
	}
	layoutData, err := os.ReadFile(mtipPath)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := mtip.Parse(layoutData)
	if err != nil {
		t.Fatalf("mtip.Parse: %v", err)
	}
	r, err := migrate.MigrateOptions(mdzip, data, migrate.Options{Layout: layout, LayoutSource: mtipPath})
	if err != nil {
		t.Fatalf("MigrateOptions: %v", err)
	}
	l := r.Report.Layout
	if l == nil {
		t.Fatal("no layout summary")
	}
	if l.DiagramsJoined != 721 {
		t.Errorf("joined %d, want 721", l.DiagramsJoined)
	}
	if l.DiagramsUnmatched != 0 {
		t.Errorf("unmatched %d, want 0", l.DiagramsUnmatched)
	}
	if l.Malformed != 0 {
		t.Errorf("malformed %d, want 0", l.Malformed)
	}
	if l.PlacementsWritten == 0 {
		t.Error("no placements written")
	}
	if l.RoutesWritten == 0 {
		t.Error("no routes written")
	}
	t.Logf("layout summary: %+v", l)
	t.Logf("summary: %s", r.Report.Summary())
	for _, d := range errors(t, "tmt.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}
