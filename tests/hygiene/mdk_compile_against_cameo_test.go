package hygiene

import (
	"strings"
	"testing"
)

// TestMDKCompileAgainstCameoIsTestedInCI fails when a plugin job stops running the helper's test,
// the only run of compile-against-cameo.sh without a licensed Cameo installation.
func TestMDKCompileAgainstCameoIsTestedInCI(t *testing.T) {
	const test = "editors/mdk/scripts/compile-against-cameo-test.sh"
	for path, step := range map[string]string{
		".github/workflows/pr.yml": "run: " + test,
		".circleci/config.yml":     "command: " + test,
	} {
		if !strings.Contains(readRepoFile(t, path), step) {
			t.Errorf("%s no longer runs %s", path, test)
		}
	}
}
