package hygiene

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// codegenImporters are the packages that may import internal/translate/codegen,
// the native compilation backend behind `sysml -compile`: the command that
// drives it and the black-box suites under tests/. No other package, nor any
// other package's tests, reaches it, so a build or test run of the runtime
// never pays for it.
var codegenImporters = map[string]bool{
	"cmd/sysml": true,
}

// TestCodegenReachedOnlyFromItsCommand checks every package in the module,
// its tests included, against codegenImporters.
func TestCodegenReachedOnlyFromItsCommand(t *testing.T) {
	const codegen = "internal/translate/codegen"
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`, "./...")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var importers []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		from := strings.TrimPrefix(fields[0], modulePath)
		if from == codegen || strings.HasPrefix(from, "tests/") {
			continue
		}
		for _, imp := range fields[1:] {
			if strings.TrimPrefix(imp, modulePath) != codegen {
				continue
			}
			if !codegenImporters[from] {
				t.Errorf("%s imports %s; only cmd/sysml and the suites under tests/ may", from, codegen)
			}
			importers = append(importers, from)
			break
		}
	}
	sort.Strings(importers)
	for pkg := range codegenImporters {
		if !contains(importers, pkg) {
			t.Errorf("%s no longer imports %s; remove it from codegenImporters", pkg, codegen)
		}
	}
}
