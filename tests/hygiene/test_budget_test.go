package hygiene

import (
	"strings"
	"testing"
)

const budgetScaleEnv = "OPENSYSML_TEST_BUDGET_SCALE"

// TestCIDoesNotScaleTestBudgets: CI uses each test's declared deadline; the
// scale is a local accommodation, never a way to pass a gate.
func TestCIDoesNotScaleTestBudgets(t *testing.T) {
	for _, path := range []string{".github/workflows/pr.yml", ".circleci/config.yml"} {
		if strings.Contains(readRepoFile(t, path), budgetScaleEnv) {
			t.Errorf("%s sets %s; CI runs every wall-clock deadline at its declared value", path, budgetScaleEnv)
		}
	}
}

// TestBudgetScaleHasOneReader keeps the opt-in in one helper, so no test
// parses a second scale that could change an assertion.
func TestBudgetScaleHasOneReader(t *testing.T) {
	for _, path := range trackedFiles(t) {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		switch path {
		case "tests/testutil/testbudget/budget.go", "tests/hygiene/test_budget_test.go":
			continue
		}
		if strings.Contains(readRepoFile(t, path), budgetScaleEnv) {
			t.Errorf("%s reads %s directly; use tests/testutil/testbudget", path, budgetScaleEnv)
		}
	}
}
