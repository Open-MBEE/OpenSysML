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
	repoRoot          = "../.."
	sharedDigestTable = "../../client/release-digests.json"
	changelog         = "../../CHANGELOG.md"
	changelogArchive  = "../../docs/project/archive/CHANGELOG-0.0-0.8.md"
	digestRepo        = "Open-MBEE/OpenSysML"
	unsignedRelease   = "published before the pipeline signed its manifest; there is no bundle to verify a digest with"
)

// Releases the changelog records that the shared table cannot pin, each with
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
// tree itself is cutting is the one exception, until it is tagged and the tree
// has moved past the tagged commit: its digests exist only once its tag has
// built, and they are pinned on develop afterwards, so the tagged commit
// itself (main, after a release) can never hold them.
//
// CHANGELOG.md and the archive its older releases moved to, not git, are the
// list of releases: a shallow clone has no tags, and the heading is written on
// the release branch before any tag exists. git
// is consulted only to tell that release, still being cut, from one already
// tagged, whose pin is due; a clone without the tag reads as the former.
func TestSharedDigestTablePinsEveryRecordedRelease(t *testing.T) {
	table := sharedPins(t)
	own := ownReleaseTag(t)
	var releases [][]string
	for _, path := range []string{changelog, changelogArchive} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		found := changelogRelease.FindAllStringSubmatch(string(raw), -1)
		if len(found) == 0 {
			t.Fatalf("%s records no release", path)
		}
		releases = append(releases, found...)
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
			// Releases from the one that introduced the kernel pin it whole.
			if pinsAny(pins, kernelAssets) {
				for _, asset := range kernelAssets {
					if pins[asset] == "" {
						t.Errorf("%s pins some kernel binaries but no digest for %s", tag, asset)
					}
				}
			}
			continue
		}
		if _, listed := unpinnableReleases[tag]; listed {
			continue
		}
		if tag == own && !ownPinDue(repoRoot, tag, os.Getenv("CIRCLE_TAG")) {
			continue
		}
		t.Errorf("the changelog records %s as released, but client/release-digests.json does not pin it;\n"+
			"merge the chore/pin-%s pull request the tag's pipeline opened against develop, or run\n"+
			"python3 client/python/scripts/pin_release_checksums.py --version %s --write", tag, tag, tag)
	}
}

func pinsAny(pins map[string]string, assets []string) bool {
	for _, asset := range assets {
		if pins[asset] != "" {
			return true
		}
	}
	return false
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

// ownPinDue reports whether the tree's own release must be pinned: its tag is in
// the checkout and HEAD has moved past the tagged commit; the tag pipeline is exempt.
func ownPinDue(repo, tag, circleTag string) bool {
	if circleTag == tag || !tagExists(repo, tag) {
		return false
	}
	head := gitRevParse(repo, "HEAD")
	return head == "" || head != gitRevParse(repo, tag+"^{commit}")
}

// tagExists reports whether the release tag is in the checkout.
func tagExists(repo, tag string) bool {
	cmd := exec.Command("git", "tag", "--list", tag)
	cmd.Dir = repo
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == tag
}

// gitRevParse is the commit a revision names, or "" when git cannot tell.
func gitRevParse(repo, rev string) string {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", rev)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// TestOwnPinDueFollowsTheTag walks a release through a scratch repository: the
// pin is due only once the tag exists and the checkout has moved past it.
func TestOwnPinDueFollowsTheTag(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.name=hygiene", "-c", "user.email=hygiene@example.invalid", "-c", "commit.gpgsign=false",
		}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	const tag = "v1.2.3"
	expect := func(state string, want bool) {
		t.Helper()
		if got := ownPinDue(repo, tag, ""); got != want {
			t.Errorf("%s: ownPinDue = %v, want %v", state, got, want)
		}
		if ownPinDue(repo, tag, tag) {
			t.Errorf("%s: the tag's own pipeline (CIRCLE_TAG=%s) must stay exempt", state, tag)
		}
	}

	git("init", "--quiet")
	git("commit", "--quiet", "--allow-empty", "-m", "release")
	expect("release branch, tag not yet pushed", false)
	git("tag", tag)
	expect("the tagged commit (main after the release)", false)
	git("commit", "--quiet", "--allow-empty", "-m", "next")
	expect("a commit past the tag (develop, a hotfix branch)", true)
	git("checkout", "--quiet", "--detach", tag)
	expect("detached at the tag, as CI checks a release out", false)
}
