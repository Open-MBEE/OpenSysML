package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

const modulePath = "github.com/Open-MBEE/OpenSysML/"

// The pull-request workflow re-runs the differential only on the paths the selector's pilotdiff area
// claims, so a package this referee imports and the area does not claim could move the baseline unseen.
func TestChangedAreasClaimEveryPackageTheRefereeImports(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(root, "scripts", "ci-changed-areas.sh"))
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^pilotdiff_pattern='(.*)'$`).FindSubmatch(script)
	if found == nil {
		t.Fatal("scripts/ci-changed-areas.sh declares no pilotdiff_pattern")
	}
	claimed, err := regexp.Compile(string(found[1]))
	if err != nil {
		t.Fatalf("pilotdiff_pattern does not compile: %v", err)
	}

	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	imported := 0
	for _, pkg := range strings.Fields(string(out)) {
		dir, ours := strings.CutPrefix(pkg, modulePath)
		if !ours {
			continue
		}
		imported++
		if !claimed.MatchString(dir + "/x.go") {
			t.Errorf("the referee imports %s, which scripts/ci-changed-areas.sh's pilotdiff area does not claim", dir)
		}
	}
	if imported == 0 {
		t.Fatal("go list -deps named no package of this module, so nothing was checked")
	}
	for _, unclaimed := range []string{"internal/exec/runtime/executor.go", ".github/workflows/pr.yml", "docs/guide/index.md"} {
		if claimed.MatchString(unclaimed) {
			t.Errorf("pilotdiff_pattern claims %s, which cannot move the baseline", unclaimed)
		}
	}
}
