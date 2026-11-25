package hygiene

import (
	"strings"
	"testing"
)

const budgetScaleEnv = "OPENSYSML_TEST_BUDGET_SCALE"

// TestCIDoesNotScaleTestBudgets: CI's deadlines are the tests' declared values.
// The scale is a local, opt-in accommodation for a contended host, never a way
// to make a CI gate pass.
func TestCIDoesNotScaleTestBudgets(t *testing.T) {
	for _, path := range []string{".github/workflows/pr.yml", ".circleci/config.yml"} {
		if strings.Contains(readRepoFile(t, path), budgetScaleEnv) {
			t.Errorf("%s sets %s; CI runs every wall-clock deadline at its declared value", path, budgetScaleEnv)
		}
	}
}

// TestBudgetScaleHasOneReader keeps the opt-in in one helper: a test that
// reaches for the environment directly would add a second, differently parsed
// scale and could silently change an assertion rather than a deadline.
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
