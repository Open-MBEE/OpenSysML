package corpus

import (
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

var updatePSSMMigration = flag.Bool("update-pssm-migration", false,
	"rewrite testdata/pssm_migration_expected.txt from the current results")

// The pinned suite, installed by scripts/download-pssm-suite.sh where the
// referee reads it; the gate shares the referee's require variable.
const pssmSuite = "../../build/pssm/PSSM_TestSuite.xmi"

var pssmMigrationGate = corpusGate{
	name:       "pssm-migration",
	expected:   "testdata/pssm_migration_expected.txt",
	requireEnv: "OPENSYSML_REQUIRE_PSSM_SUITE",
	absent:     "The OMG PSSM test suite is absent, so this run proves nothing about migrating it.",
	fetch:      "Fetch it with ./scripts/download-pssm-suite.sh and re-run",
	runPattern: "TestPSSMSuiteMigration",
	skipHint:   "PSSM test suite not downloaded (run ./scripts/download-pssm-suite.sh)",
}

// Byte-identical to the committed file's header, so regenerating without a
// movement rewrites it unchanged.
const pssmMigrationExpectedHeader = "# Migrating the pinned OMG PSSM test suite (build/pssm/PSSM_TestSuite.xmi) to\n" +
	"# SysML v2 notation: the migration report's totals by verdict and the number\n" +
	"# of error diagnostics the written notation analyses with, as \"<count>\\t<figure>\".\n" +
	"# The notation parsing clean is asserted, not recorded; these figures are a\n" +
	"# ratchet whose every movement is adjudicated. See docs/project/pssm-migration.md.\n" +
	"# Regenerate with:\n" +
	"#   go test ./tests/corpus -run TestPSSMSuiteMigration -update-pssm-migration\n"

// pssmFigures are the recorded figures, in the order they are reported.
var pssmFigures = []string{"mapped", "approximated", "unmapped", "skipped", "validation-errors"}

// The migrated suite must parse (asserted); its report totals and validation
// errors ratchet in testdata/pssm_migration_expected.txt (docs/project/pssm-migration.md).
func TestPSSMSuiteMigration(t *testing.T) {
	data, err := os.ReadFile(pssmSuite)
	if os.IsNotExist(err) {
		pssmMigrationGate.skip(t, pssmSuite+" is missing")
	}
	if err != nil {
		t.Fatalf("read %s: %v", pssmSuite, err)
	}
	if len(data) == 0 {
		pssmMigrationGate.skip(t, pssmSuite+" is empty")
	}

	// An empty semantic cache measures the implementation, not the machine.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got := pssmMigrationCounts(t, data)

	if *updatePSSMMigration {
		pssmMigrationGate.writeExpected(t, pssmMigrationExpectedHeader, nil, got)
		t.Logf("wrote %s: %v", pssmMigrationGate.expected, got)
		return
	}

	_, want := pssmMigrationGate.readExpected(t)
	for _, figure := range pssmFigures {
		if _, ok := want[figure]; !ok {
			t.Errorf("%s records no %s figure; regenerate with -update-pssm-migration",
				pssmMigrationGate.expected, figure)
			continue
		}
		if got[figure] != want[figure] {
			direction := "more"
			if got[figure] < want[figure] {
				direction = "fewer"
			}
			t.Errorf("%s: %d, expected %d (%s than recorded); "+
				"adjudicate the change, then regenerate with -update-pssm-migration",
				figure, got[figure], want[figure], direction)
		}
	}
	for _, figure := range sortedKeys(want) {
		if _, ok := got[figure]; !ok {
			t.Errorf("%s records %q, which is not a figure this gate measures (%v)",
				pssmMigrationGate.expected, figure, pssmFigures)
		}
	}
}

// pssmMigrationCounts migrates the suite, asserts that the notation parses,
// and measures the ratcheted figures.
func pssmMigrationCounts(t *testing.T, data []byte) map[string]int {
	t.Helper()

	const name = "PSSM_TestSuite.xmi"
	m, err := convert.Migrate(name, data, convert.FormatSysML, migrate.Options{})
	var syntax *convert.SyntaxError
	switch {
	case errors.As(err, &syntax):
		t.Fatalf("the migrated notation must parse, but has %d syntax error(s); "+
			"a v1 shape is written in a form the parser rejects:\n  %s",
			len(syntax.Messages), strings.Join(syntax.Messages, "\n  "))
	case err != nil:
		t.Fatalf("migrate %s: %v", name, err)
	}

	counts := m.Report.Count()
	got := map[string]int{
		"mapped":       counts[migrate.Mapped],
		"approximated": counts[migrate.Approximated],
		"unmapped":     counts[migrate.Unmapped],
		"skipped":      counts[migrate.Skipped],
	}

	const written = "PSSM_TestSuite.sysml"
	ws := model.NewWorkspace()
	ws.Open(written, m.Output, 1)
	for _, d := range ws.Diagnostics(written) {
		if d.Severity == diag.SeverityError {
			got["validation-errors"]++
			t.Logf("%s: %s", written, d.Message)
		}
	}
	t.Logf("%s: %s; the notation analyses with %d error(s)",
		name, m.Report.Summary(), got["validation-errors"])
	return got
}
