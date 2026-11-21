package hygiene

import (
	"os"
	"regexp"
	"testing"
)

// The release, the nightly and the PyPI workflow sign and verify one manifest
// format, so they pin one cosign: a bump that misses a file fails here.
func TestCosignIsPinnedAtOneVersionEverywhere(t *testing.T) {
	pins := map[string]*regexp.Regexp{
		"../../.circleci/config.yml":               regexp.MustCompile(`github\.com/sigstore/cosign/v\d+/cmd/cosign@(\S+)`),
		"../../.github/workflows/nightly.yml":      regexp.MustCompile(`cosign-release:\s*(\S+)`),
		"../../.github/workflows/release-pypi.yml": regexp.MustCompile(`cosign-release:\s*(\S+)`),
	}
	exact := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	versions := map[string]string{}
	for path, pin := range pins {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		found := pin.FindAllStringSubmatch(string(raw), -1)
		if len(found) == 0 {
			t.Errorf("%s installs no pinned cosign, so the check would pass over nothing", path)
		}
		for _, match := range found {
			if !exact.MatchString(match[1]) {
				t.Errorf("%s pins cosign at %q, not an exact release", path, match[1])
			}
			versions[match[1]] = path
		}
	}
	if len(versions) > 1 {
		t.Errorf("cosign is pinned at more than one version: %v", versions)
	}
}
