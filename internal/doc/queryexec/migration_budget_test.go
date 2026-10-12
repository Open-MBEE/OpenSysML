package queryexec

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestMigratedExcludingFilterFitsBudgetBelowThreeTraversals(t *testing.T) {
	data, err := os.ReadFile("../../../tests/migrate/testdata/xmi/documents.xmi")
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := migrate.Migrate("documents.xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	notation := string(migrated.Notation)
	const syntheticElements = 32_000
	const visitBudget = 80_000
	if 3*syntheticElements <= visitBudget {
		t.Fatalf("visit budget %d does not rule out three traversals of %d elements", visitBudget, syntheticElements)
	}
	packagePrefix := "package Fleet {\n    package Structure {"
	opening := strings.Index(notation, packagePrefix)
	if opening < 0 {
		t.Fatal("migrated Fleet::Structure package is missing")
	}
	insertAt := opening + len(packagePrefix)
	var additions strings.Builder
	for i := 0; i < syntheticElements; i++ {
		fmt.Fprintf(&additions, "\n        part def BudgetElement%d;", i)
	}
	notation = notation[:insertAt] + additions.String() + notation[insertAt:]

	fixture := loadExecutionSource(t, notation)
	queryNames := fixture.index.FQNsEndingIn("Fleet Handbook Fleet Parts Rows", 2)
	if len(queryNames) != 1 {
		t.Fatalf("lookup Fleet Handbook Fleet Parts Rows: got %d candidates", len(queryNames))
	}
	matches := symbols.PreferDeclared(fixture.index.LookupQualified(queryNames[0]))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", queryNames[0], len(matches))
	}
	program, err := queryplan.Compile(fixture.index, fixture.model, fixture.resolver, matches[0])
	if err != nil {
		t.Fatalf("compile Fleet Handbook Fleet Parts Rows: %v", err)
	}
	context := Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}
	expected, err := Execute(program, context, Bindings{}, Options{})
	if err != nil {
		t.Fatalf("execute Fleet Handbook Fleet Parts Rows with default budget: %v", err)
	}
	if got := len(expected.Rows()); got < syntheticElements {
		t.Fatalf("default query returned %d rows, want at least %d synthetic rows", got, syntheticElements)
	}
	actual, err := Execute(program, context, Bindings{}, Options{VisitBudget: visitBudget})
	if err != nil {
		t.Fatalf("execute Fleet Handbook Fleet Parts Rows with visit budget %d: %v", visitBudget, err)
	}
	if want, got := migratedRows(t, fixture, expected), migratedRows(t, fixture, actual); !slices.Equal(want, got) {
		t.Fatalf("rows with visit budget %d = %v, want %v", visitBudget, got, want)
	}
}

func migratedRows(t *testing.T, fixture executionFixture, rows *RowSet) []string {
	t.Helper()
	var names []string
	for _, row := range rows.Rows() {
		declaration := row.Element().Declaration()
		if declaration == nil {
			t.Fatal("migrated query returned a row without a declaration")
		}
		names = append(names, fixture.index.GetFQN(declaration))
	}
	return names
}
