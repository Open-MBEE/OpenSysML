package project

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// canonicalSysmod is the usage a package manager writes for MBSE4U/sysmod-sysmlv2
// at 5.1.6: an ECMA-427 package URL of type generic whose vcs_url qualifier
// carries the repository, percent-encoded.
const canonicalSysmod = "pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2%405.1.6"

// A pkg:generic resource with a vcs_url qualifier names the repository the
// qualifier spells, read as the pkg:git/<host>/<path> spelling is: the path's
// last two segments, the @version and the #subpath come from the decoded value.
func TestUsesReadsTheRepositoryFromTheVCSURLQualifier(t *testing.T) {
	sysmod := &Interchange{Publisher: "mbse4u", Name: "sysmod"}
	repo := &Interchange{Publisher: "MBSE4U", Name: "sysmod-sysmlv2"}
	gears := &Interchange{Publisher: "acme", Name: "gears"}
	widgets := &Interchange{Publisher: "acme", Name: "widgets"}
	cases := []struct {
		resource string
		uses     *Interchange
		not      *Interchange
	}{
		{canonicalSysmod, repo, sysmod},
		{"pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2", repo, sysmod},
		{"PKG:GENERIC/sysmod-sysmlv2?vcs_url=git%2Bhttps%3A%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2", repo, sysmod},
		{"pkg:generic/sysmod-sysmlv2@5.1.6?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2", repo, sysmod},
		{"pkg:generic/widgets?vcs_url=git%2Bhttps:%2F%2Fgit.example.org%2Facme%2Fwidgets%401.0.0%23packages%2Fgears", gears, widgets},
		{"pkg:generic/widgets?vcs_url=hg%2Bstatic-http:%2F%2Fhg.example.org%2Facme%2Fwidgets%23packages%2Fgears%2F", gears, widgets},
		{"pkg:generic/widgets?vcs_url=svn%2Bsvn:%2F%2Fsvn.example.org%2Facme%2Fwidgets#packages/gears", gears, widgets},
		{"pkg:generic/widgets?vcs_url=git%2Bhttps:%2F%2Fgit.example.org%2Facme%2Fwidgets.git%401.0.0", widgets, gears},
		{"pkg:generic/rover?vcs_url=git%2Bfile:%2F%2F%2Fsrv%2Fgit%2Facme%2Fgears.git%401.0.0", gears, widgets},
		{"git+https://github.com/MBSE4U/sysmod-sysmlv2@5.1.6", repo, sysmod},
		{"svn+ssh://svn.example.org/acme/widgets#packages/gears", gears, widgets},
		{"pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.6", repo, sysmod},
		{"pkg:generic/acme/gears", gears, widgets},
	}
	for _, c := range cases {
		if !c.uses.Uses(Usage{Resource: c.resource}) {
			t.Errorf("%s/%s does not answer %s", c.uses.Publisher, c.uses.Name, c.resource)
		}
		if c.not.Uses(Usage{Resource: c.resource}) {
			t.Errorf("%s/%s answers %s", c.not.Publisher, c.not.Name, c.resource)
		}
	}
}

// The version of a pkg:generic resource is the one after @ in its vcs_url, else
// the one after @ in the purl's name; the vcs_url's wins when both are written.
func TestResourceVersionReadsTheVCSURL(t *testing.T) {
	for resource, want := range map[string]string{
		canonicalSysmod: "5.1.6",
		"pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2":                     "",
		"pkg:generic/sysmod-sysmlv2@5.1.5?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2":               "5.1.5",
		"pkg:generic/sysmod-sysmlv2@5.1.5?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2%405.1.6":       "5.1.6",
		"pkg:generic/widgets?vcs_url=git%2Bhttps:%2F%2Fgit.example.org%2Facme%2Fwidgets%401.0.0%23packages%2Fgears":     "1.0.0",
		"pkg:generic/widgets?vcs_url=git%2Bhttps:%2F%2Fgit.example.org%2Facme%2Fwidgets%23packages%2Fgears%401.0.0":     "",
		"git+https://github.com/MBSE4U/sysmod-sysmlv2@5.1.6":                                                            "5.1.6",
		"pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.6":                                                                "5.1.6",
		"pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2%405.1.6&other=x%ZZ":  "",
		"pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2%405.1.6&vcs_url=git": "",
	} {
		if got := (Usage{Resource: resource}).constraint(); got != want {
			t.Errorf("constraint(%s) = %q, want %q", resource, got, want)
		}
	}
}

// The three spellings of one repository share a key, so a project learnt for
// one answers the others; the version is not part of it.
func TestKeyIsSharedByTheSpellingsOfOneRepository(t *testing.T) {
	want := (Usage{Resource: "pkg:git/github.com/MBSE4U/sysmod-sysmlv2"}).key()
	if want != "github-com/mbse4u/sysmod-sysmlv2" {
		t.Fatalf("key of the pkg:git spelling = %q", want)
	}
	for _, resource := range []string{canonicalSysmod, "pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod-sysmlv2", "git+https://github.com/MBSE4U/sysmod-sysmlv2@5.1.6", "pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.5"} {
		if got := (Usage{Resource: resource}).key(); got != want {
			t.Errorf("key(%s) = %q, want %q", resource, got, want)
		}
	}
}

func TestSpellsLocationReadsTheECMA427Forms(t *testing.T) {
	for resource, want := range map[string]bool{
		canonicalSysmod: true,
		"pkg:generic/rover?vcs_url=hg%2Bhttps:%2F%2Fhg.example.org%2Fmodels%2Frover%23packages%2Fpower": true,
		"pkg:generic/rover?vcs_url=git%2Bfile:%2F%2F%2Fsrv%2Fgit%2Frover.git%401.0.0":                   true,
		"pkg:generic/rover?vcs_url=not-a-url":                                                           true,
		"git+https://github.com/MBSE4U/sysmod-sysmlv2@5.1.6":                                            true,
		"svn+ssh://svn.example.org/acme/widgets":                                                        true,
		"hg+static-http://hg.example.org/models/rover":                                                  true,
		"pkg:generic/acme/gears":                                                                        false,
		"pkg:generic/gears?checksum=sha256:abc":                                                         false,
		"pkg:generic/gears@1.0.0":                                                                       false,
	} {
		if got := (Usage{Resource: resource}).spellsLocation(); got != want {
			t.Errorf("spellsLocation(%s) = %v, want %v", resource, got, want)
		}
	}
}

// A vcs_url that does not decode to a <vcs>+<transport>://<host>/<path> URL
// is an error naming what was found; the resource then names no project.
func TestLocationRefusesAMalformedVCSURL(t *testing.T) {
	for resource, want := range map[string]string{
		"pkg:generic/rover?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2FMBSE4U%2Fsysmod%ZZ": "the qualifiers do not decode",
		"pkg:generic/rover?vcs_url=":                                             "an empty vcs_url qualifier",
		"pkg:generic/rover?vcs_url=a&vcs_url=b":                                  "2 vcs_url qualifiers",
		"pkg:generic/rover?vcs_url=not-a-url":                                    `"not-a-url" is not a <vcs>+<transport>://<host>/<path> URL`,
		"pkg:generic/rover?vcs_url=git%2Bhttps:%2F%2Fgithub.com":                 `"git+https://github.com" is not`,
		"pkg:generic/rover?vcs_url=git%2Bhttps:%2F%2Fgithub.com%2F":              `"git+https://github.com/" is not`,
		"pkg:generic/rover?vcs_url=git%2Bhttps:%2F%2F%2Facme%2Frover":            `"git+https:///acme/rover" is not`,
		"pkg:generic/rover?vcs_url=1git%2Bhttps:%2F%2Fgithub.com%2Facme%2Frover": `"1git+https://github.com/acme/rover" is not`,
		"pkg:generic/rover?vcs_url=git%20https:%2F%2Fgithub.com%2Facme%2Frover":  `"git https://github.com/acme/rover" is not`,
	} {
		u := Usage{Resource: resource}
		_, err := u.location()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("location(%s) = %v, want an error containing %q", resource, err, want)
		}
		if got := resourceSegments(resource); got != nil {
			t.Errorf("resourceSegments(%s) = %v, want none", resource, got)
		}
		if (&Interchange{Publisher: "acme", Name: "rover"}).Uses(u) {
			t.Errorf("acme/rover answers %s", resource)
		}
	}
	for _, resource := range []string{canonicalSysmod, "pkg:generic/rover?vcs_url=git%2Bfile:%2F%2F%2Fsrv%2Fgit%2Frover.git", "pkg:generic/acme/gears", "pkg:generic/gears?checksum=sha256:abc", "pkg:git/github.com/MBSE4U/sysmod-sysmlv2"} {
		if _, err := (Usage{Resource: resource}).location(); err != nil {
			t.Errorf("location(%s): %v", resource, err)
		}
	}
	if got, _ := (Usage{Resource: canonicalSysmod}).location(); got != "git+https://github.com/MBSE4U/sysmod-sysmlv2@5.1.6" {
		t.Errorf("location(%s) = %q", canonicalSysmod, got)
	}
}

// The canonical spelling links the installed project by elimination exactly as
// the pkg:git spelling does, with the same warning but for the resource text.
func TestUsedLinksTheCanonicalSpellingAsThePkgGitSpelling(t *testing.T) {
	for _, resource := range []string{canonicalSysmod, "pkg:git/github.com/MBSE4U/sysmod-sysmlv2@5.1.6", "git+https://github.com/MBSE4U/sysmod-sysmlv2@5.1.6"} {
		t.Run(resource, func(t *testing.T) {
			proj := writeProject(t, filepath.Join(t.TempDir(), "proj"), fmt.Sprintf(`{"name": "app", "version": "0.1.0", "usage": [{"resource": %q}]}`, resource))
			writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.5"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.5"}`)
			sysmod516 := writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.6"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.6"}`)
			found := Discovery{}.Around([]string{filepath.Join(proj, "App.sysml")})
			if got := projectDirs(found.Used); !reflect.DeepEqual(got, []string{sysmod516}) {
				t.Fatalf("Used = %v, want %v", got, sysmod516)
			}
			want := []Warning{{
				Path:    filepath.Join(proj, ProjectFile),
				Message: fmt.Sprintf("usage %s names no installed project by publisher and name; linked by elimination to mbse4u/sysmod 5.1.6 (%s), the one unclaimed installed project that satisfies it; end the resource with the project's publisher and name (…/mbse4u/sysmod) to name it", resource, sysmod516),
			}}
			if !reflect.DeepEqual(found.Linking, want) {
				t.Errorf("Linking = %q, want %q", found.Linking, want)
			}
		})
	}
}

// A canonical resource whose repository path does spell the project's
// publisher and name, or whose #subpath names a sub-package, links by name.
func TestUsedLinksTheCanonicalSpellingByNameAndBySubpath(t *testing.T) {
	proj := writeProject(t, filepath.Join(t.TempDir(), "proj"), `{"name": "app", "version": "0.1.0", "usage": [
  {"resource": "pkg:generic/widgets?vcs_url=git%2Bhttps:%2F%2Fgit.example.org%2Facme%2Fwidgets%401.0.0"},
  {"resource": "pkg:generic/widgets?vcs_url=git%2Bhttps:%2F%2Fgit.example.org%2Facme%2Fwidgets%23packages%2Fgears", "versionConstraint": "^1.0.0"}]}`)
	widgets := writeProject(t, filepath.Join(proj, "lib", "acme-widgets_1.0.0"), `{"name": "widgets", "publisher": "acme", "version": "1.0.0"}`)
	writeProject(t, filepath.Join(proj, "lib", "acme-widgets_2.0.0"), `{"name": "widgets", "publisher": "acme", "version": "2.0.0"}`)
	gears := writeProject(t, filepath.Join(proj, "lib", "acme-gears_1.2.0"), `{"name": "gears", "publisher": "acme", "version": "1.2.0"}`)
	writeProject(t, filepath.Join(proj, "lib", "acme-gears_2.0.0"), `{"name": "gears", "publisher": "acme", "version": "2.0.0"}`)
	found := Discovery{}.Around([]string{filepath.Join(proj, "App.sysml")})
	if got := projectDirs(found.Used); !reflect.DeepEqual(got, []string{widgets, gears}) {
		t.Errorf("Used = %v, want %v", got, []string{widgets, gears})
	}
	if len(found.Linking) != 0 {
		t.Errorf("Linking = %q, want none: both usages name their project", found.Linking)
	}
}

// A usage whose vcs_url does not decode is a warning on the consumer's
// .project.json and selects nothing, leaving the installed project unclaimed
// for no one: it is not linked by elimination either.
func TestUsedReportsAMalformedVCSURLAndLinksNothing(t *testing.T) {
	resource := "pkg:generic/sysmod-sysmlv2?vcs_url=git%2Bhttps:%2F%2Fgithub.com"
	proj := writeProject(t, filepath.Join(t.TempDir(), "proj"), fmt.Sprintf(`{"name": "app", "version": "0.1.0", "usage": [{"resource": %q}]}`, resource))
	writeProject(t, filepath.Join(proj, "lib", "mbse4u-sysmod_5.1.6"), `{"name": "sysmod", "publisher": "mbse4u", "version": "5.1.6"}`)
	found := Discovery{}.Around([]string{filepath.Join(proj, "App.sysml")})
	if len(found.Used) != 0 {
		t.Errorf("Used = %v, want none", projectDirs(found.Used))
	}
	want := []Warning{{
		Path:    filepath.Join(proj, ProjectFile),
		Message: "usage " + resource + `: the vcs_url qualifier "git+https://github.com" is not a <vcs>+<transport>://<host>/<path> URL (a package URL of type generic names a repository as pkg:generic/<name>?vcs_url=<vcs>%2B<transport>:%2F%2F<host>%2F<path>[%40<version>][%23<subpath>], the qualifier percent-encoded)`,
	}}
	if !reflect.DeepEqual(found.Linking, want) {
		t.Errorf("Linking = %q, want %q", found.Linking, want)
	}
}
