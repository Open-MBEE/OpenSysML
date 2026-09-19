package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// OMG's `provide power` writes `first start;` and eight `first X then Y;` successions;
// it lowers, and runs until it parks at `accept engineStart`, which nothing posts.
func TestPilotProvidePowerRunsUntilItsAccept(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "pilot-corpora", "sysml-validation",
		"03-Function-based Behavior", "3a-Function-based Behavior-2.sysml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.Getenv("OPENSYSML_REQUIRE_PILOT_CORPORA") != "" {
			t.Fatalf("pilot corpora required but absent: %v", err)
		}
		t.Skipf("pilot corpora absent; run ./scripts/download-pilot-corpora.sh: %v", err)
	}
	file := parser.New(source.New(path, data)).ParseFile()
	idx, _, ctx := buildRuntimeWithLibraries(t, path, file)

	const fqn = "3a-Function-based Behavior-2::Usages::provide power"
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s: %d matching symbols, want 1", fqn, len(matches))
	}
	exec, err := ctx.CreateActionExecutor(matches[0])
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}

	err = exec.RunToCompletion()
	if !errors.Is(err, ErrAcceptDeadlock) || !strings.Contains(err.Error(), "engineStart") {
		t.Fatalf("RunToCompletion: got %v, want an accept deadlock at engineStart", err)
	}
}
