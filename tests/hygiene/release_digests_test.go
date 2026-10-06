package hygiene

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

const (
	sharedDigestTable = "../../client/release-digests.json"
	digestRepo        = "Open-MBEE/OpenSysML"
	unsignedRelease   = "published before the pipeline signed its manifest; there is no bundle to verify a digest with"
)

// Releases CHANGELOG.md records that the shared table cannot pin, each with
// the reason. Every other recorded release must be pinned.
var unpinnableReleases = map[string]string{
	"v0.0.4": "published before the service binaries existed; the release has no sysml-grpc assets",
	"v0.0.9": unsignedRelease,
	"v0.1.1": unsignedRelease,
	"v0.1.2": unsignedRelease,
}

var (
	changelogRelease = regexp.MustCompile(`(?m)^## (\S+)`)
	pep440Release    = regexp.MustCompile(`^(\d+\.\d+\.\d+)(?:(a|b|rc)(\d+))?$`)
	versionAssign    = regexp.MustCompile(`(?m)^VERSION\s*=\s*"([^"]*)"`)
)

// TestSharedDigestTablePinsEveryRecordedRelease holds client/release-digests.json
// to the releases CHANGELOG.md records: each is pinned for all five service
// assets, so a checkout, and the package of a later release asked for it,
// install it on a pin rather than on the signed manifest. The release the
// tree itself is cutting is the one exception, until it is tagged: its digests
// exist only once its tag has built.
//
// CHANGELOG.md, not git, is the list of releases: a shallow clone has no tags,
// and the heading is written on the release branch before any tag exists. git
// is consulted only to tell that release, still being cut, from one already
// tagged, whose pin is due; a clone without the tag reads as the former.
func TestSharedDigestTablePinsEveryRecordedRelease(t *testing.T) {
	table := sharedPins(t)
	own := ownReleaseTag(t)
	raw, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	releases := changelogRelease.FindAllStringSubmatch(string(raw), -1)
	if len(releases) == 0 {
		t.Fatal("CHANGELOG.md records no release")
	}
	for _, release := range releases {
		if release[1] == "Unreleased" {
			continue
		}
		tag := "v" + release[1]
		pins, pinned := table[tag]
		if pinned {
			if reason, listed := unpinnableReleases[tag]; listed {
				t.Errorf("%s is pinned, yet unpinnableReleases says it cannot be (%s); drop it from the list", tag, reason)
			}
			for _, asset := range serviceAssets {
				if pins[asset] == "" {
					t.Errorf("%s pins no digest for %s", tag, asset)
				}
			}
			continue
		}
		if _, listed := unpinnableReleases[tag]; listed {
			continue
		}
		if tag == own && !tagExists(tag) {
			continue
		}
		t.Errorf("CHANGELOG.md records %s as released, but client/release-digests.json does not pin it;\n"+
			"merge the chore/pin-%s pull request the tag's pipeline opened against develop, or run\n"+
			"python3 client/python/scripts/pin_release_checksums.py --version %s --write", tag, tag, tag)
	}
}

// sharedPins is the shared table's pins for this repository, by release tag.
func sharedPins(t *testing.T) map[string]map[string]string {
	t.Helper()
	raw, err := os.ReadFile(sharedDigestTable)
	if err != nil {
		t.Fatalf("read %s: %v", sharedDigestTable, err)
	}
	var table map[string]map[string]map[string]string
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatalf("parse %s: %v", sharedDigestTable, err)
	}
	pins, ok := table[digestRepo]
	if !ok {
		t.Fatalf("%s has no table for %s", sharedDigestTable, digestRepo)
	}
	return pins
}

// ownReleaseTag is the tag of the version the tree declares, spelled as the
// release workflow spells it: PEP 440 in _version.py, SemVer on the tag.
func ownReleaseTag(t *testing.T) string {
	t.Helper()
	const versionFile = "../../client/python/opensysml/_version.py"
	raw, err := os.ReadFile(versionFile)
	if err != nil {
		t.Fatalf("read %s: %v", versionFile, err)
	}
	assign := versionAssign.FindSubmatch(raw)
	if assign == nil {
		t.Fatalf("%s declares no VERSION", versionFile)
	}
	parts := pep440Release.FindStringSubmatch(string(assign[1]))
	if parts == nil {
		t.Fatalf("VERSION %q in %s is not a release or an a/b/rc pre-release", assign[1], versionFile)
	}
	tag := "v" + parts[1]
	if parts[2] != "" {
		tag += "-" + map[string]string{"a": "alpha", "b": "beta", "rc": "rc"}[parts[2]] + "." + parts[3]
	}
	return tag
}

// tagExists reports whether the release tag has been pushed, as far as this
// checkout can tell. The tag's own release pipeline runs with the tag checked
// out but cannot have pinned it: the digests are being built in that very run.
func tagExists(tag string) bool {
	if os.Getenv("CIRCLE_TAG") == tag {
		return false
	}
	cmd := exec.Command("git", "tag", "--list", tag)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == tag
}
