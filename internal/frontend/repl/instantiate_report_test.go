package repl

import (
	"strings"
	"testing"
)

// The steps reading an object's feature values belong to the object, so a later command
// reports its own execution rather than replaying the materialization.
func TestInstantiateReportCarriesItsOwnTrace(t *testing.T) {
	s := loadFixture(t, "testdata/derived_package.sysml")
	wants(t, run(t, s, "%trace on"), "trace: on")

	report, err := s.InstantiateReport("Derived::Vehicle")
	if err != nil {
		t.Fatalf("InstantiateReport: %v", err)
	}
	wants(t, strings.Join(report.Lines, "\n"), "[trace] ", "eval feature mass")

	next, err := s.EvalExpr("1 + 1")
	if err != nil {
		t.Fatalf("EvalExpr: %v", err)
	}
	rejects(t, strings.Join(next, "\n"), "eval feature mass")
}
