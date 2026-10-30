package project

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
)

// ProjectFile is the file every OMG project interchange directory holds beside
// its model files (KerML 1.0 §10.3): a directory holding one is a project.
const ProjectFile = ".project.json"

// Interchange is the InterchangeProject information a project's .project.json
// carries (KerML 1.0 Table 12), with the directory it was read from — or, for
// an archived project, the archive's path.
type Interchange struct {
	Dir         string
	Name        string
	Publisher   string
	Description string
	Version     string
	License     string
	Usage       []Usage
}

// Usage is one InterchangeProjectUsage: the IRI of a project the project uses
// and, optionally, a constraint on the versions of it that may be used.
type Usage struct {
	Resource          string `json:"resource"`
	VersionConstraint string `json:"versionConstraint"`
}

// IsProjectDir reports whether dir holds a project interchange file.
func IsProjectDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ProjectFile))
	return err == nil && !info.IsDir()
}

// ReadInterchange reads the project interchange file in dir. A file that is
// missing, unreadable or not the JSON object the specification describes is an
// error; properties the specification does not name are passed over.
func ReadInterchange(dir string) (*Interchange, error) {
	file := filepath.Join(dir, ProjectFile)
	// #nosec G304 -- the file is the project interchange file of a directory the user named or that holds one.
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parseInterchange(dir, file, content)
}

// parseInterchange reads the InterchangeProject object of Table 12 from the
// bytes of a .project.json, reported as file; the project's Dir is dir. The
// name and version are mandatory, as is the resource of every usage.
func parseInterchange(dir, file string, content []byte) (*Interchange, error) {
	var raw struct {
		Name        string  `json:"name"`
		Publisher   string  `json:"publisher"`
		Description string  `json:"description"`
		Version     string  `json:"version"`
		License     string  `json:"license"`
		Usage       []Usage `json:"usage"`
	}
	if err := decodeObject(content, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("%s: the project has no name", file)
	}
	if raw.Version == "" {
		return nil, fmt.Errorf("%s: the project has no version", file)
	}
	for i, u := range raw.Usage {
		if u.Resource == "" {
			return nil, fmt.Errorf("%s: usage %d names no resource", file, i+1)
		}
	}
	return &Interchange{
		Dir:         dir,
		Name:        raw.Name,
		Publisher:   raw.Publisher,
		Description: raw.Description,
		Version:     raw.Version,
		License:     raw.License,
		Usage:       raw.Usage,
	}, nil
}

// IsArchived reports whether the project was read from an archive rather
// than a directory.
func (p *Interchange) IsArchived() bool { return IsArchive(p.Dir) }

// Files lists the model files the project contributes, sorted: the model
// members of an archive, or the loadable files under a directory, at most
// maxDependencyDirs directories deep. A directory below it that holds a project
// interchange file of its own is a project of its own, read through its own
// installation, and contributes nothing here; nor do its members in an archive.
func (p *Interchange) Files() []string {
	if p.IsArchived() {
		a, err := openCached(p.Dir)
		if err != nil {
			return nil
		}
		return a.Files()
	}
	var files []string
	budget := maxDependencyDirs
	walkTolerant(p.Dir, map[string]bool{}, &budget, IsLoadable, IsProjectDir, &files)
	sort.Strings(files)
	return files
}

// EnclosingProjects lists the directories at or above path that are projects,
// the nearest first; path names a file or a directory, and need not exist.
func EnclosingProjects(path string) []string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	dir := abs
	var out []string
	// An archive, or a member of one, belongs to the archive's project first.
	if archive, _, ok := SplitMemberPath(abs); ok {
		out, dir = append(out, archive), archive
	} else if IsArchiveFile(abs) {
		out = append(out, abs)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if IsProjectDir(dir) {
			out = append(out, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return out
		}
		dir = parent
	}
}

// Uses reports whether the usage's resource IRI names the project: its last two
// path segments are the project's publisher and name, or its last one the name
// of a project with no publisher, each compared as a slug (lower case, runs of
// anything but letters and digits as one hyphen), so the names a registry
// spells in a path match the ones the project spells for people. A version
// written after @ in the last segment, as package URLs do, and the extension of
// an archive are not part of the name. The path read is the one of the
// resource's location (Usage.location): for a pkg:generic package URL, the
// repository its vcs_url qualifier names.
func (p *Interchange) Uses(u Usage) bool {
	segments := resourceSegments(u.Resource)
	if len(segments) == 0 {
		return false
	}
	last := segments[len(segments)-1]
	if p.Publisher == "" {
		return slug(last) == slug(p.Name)
	}
	if len(segments) < 2 {
		return false
	}
	return slug(segments[len(segments)-2]) == slug(p.Publisher) && slug(last) == slug(p.Name)
}

// identity is the project as a usage's resource spells it: its publisher and
// name as slugs, publisher/name, or the name alone for a project with none.
func (p *Interchange) identity() string {
	if p.Publisher == "" {
		return slug(p.Name)
	}
	return slug(p.Publisher) + "/" + slug(p.Name)
}

// standardLibrary reports whether the usage names a package of the standard
// library, under the IRIs the OMG publishes it at: the bundled library answers it.
func (u Usage) standardLibrary() bool {
	return strings.HasPrefix(u.Resource, identity.KerML.Prefix()) || strings.HasPrefix(u.Resource, identity.SysML.Prefix())
}

// spellsLocation reports whether the usage's resource spells where a project is
// kept rather than the project: a version-control location — a pkg:generic
// package URL carrying one in its vcs_url qualifier, the pkg:git/, pkg:svn/ and
// pkg:hg/ spellings, a bare <vcs>+<transport>:// URL — or a URL of any scheme but pkg.
func (u Usage) spellsLocation() bool {
	location, err := u.location()
	return err != nil || spellsLocation(u.Resource, location)
}

// spellsLocation reports whether a resource whose location is location spells
// where a project is kept: a location unwrapped from a package URL, a pkg:svn/
// or pkg:hg/ spelling, or a URL of any scheme but pkg.
func spellsLocation(resource, location string) bool {
	if location != resource {
		return true
	}
	lower := strings.ToLower(location)
	for _, vcs := range []string{"pkg:svn/", "pkg:hg/"} {
		if strings.HasPrefix(lower, vcs) {
			return true
		}
	}
	return !strings.HasPrefix(lower, "pkg:") && (strings.Contains(lower, "://") || strings.HasPrefix(lower, "git@"))
}

// key is the resource as a path of slugs, without version, query or fragment:
// what two usages of one repository at two versions share.
func (u Usage) key() string {
	segments := resourceSegments(u.Resource)
	for i, seg := range segments {
		segments[i] = slug(seg)
	}
	return strings.Join(segments, "/")
}

// vcsURLQualifier is the ECMA-427 qualifier (Annex B.3.5) that carries the
// location of a version-controlled package in a pkg:generic package URL.
const vcsURLQualifier = "vcs_url"

// location is where the usage's resource says the project is kept, as a
// version-control URL: <vcs>+<transport>://<host>/<path>[@<version>][#<subpath>]
// (ECMA-427 Annex B.3.5). A package URL of type generic carries one in its
// vcs_url qualifier, percent-encoded — pkg:generic/<name>?vcs_url=… is how
// package managers write a repository — and that value, decoded, is the
// location; pkg:git/<host>/<path> is the same repository over https. Any other
// resource is its own location. A vcs_url that does not decode to such a URL is
// an error: the usage spells a location, but not one that names a project.
func (u Usage) location() (string, error) {
	return resourceLocation(u.Resource)
}

// resourceLocation is Usage.location over a resource IRI.
func resourceLocation(resource string) (string, error) {
	if rest, ok := cutPrefixFold(resource, "pkg:git/"); ok {
		return "git+https://" + rest, nil
	}
	rest, ok := cutPrefixFold(resource, "pkg:generic/")
	if !ok {
		return resource, nil
	}
	// A package URL's subpath follows the whole query, after every qualifier.
	rest, subpath, _ := strings.Cut(rest, "#")
	_, query, ok := strings.Cut(rest, "?")
	if !ok {
		return resource, nil
	}
	// A + in a purl qualifier is a literal, not the space a form encodes.
	values, err := url.ParseQuery(strings.ReplaceAll(query, "+", "%2B"))
	if err != nil {
		return "", fmt.Errorf("the qualifiers do not decode: %w", err)
	}
	vcsURLs, ok := values[vcsURLQualifier]
	switch {
	case !ok:
		return resource, nil
	case len(vcsURLs) > 1:
		return "", fmt.Errorf("%d vcs_url qualifiers, where one is expected", len(vcsURLs))
	case vcsURLs[0] == "":
		return "", errors.New("an empty vcs_url qualifier")
	}
	location := vcsURLs[0]
	if !spellsVCSURL(location) {
		return "", fmt.Errorf("the vcs_url qualifier %q is not a <vcs>+<transport>://<host>/<path> URL (vcs: git, svn or hg)", location)
	}
	switch {
	case subpath == "":
	case strings.Contains(location, "#"):
		return "", fmt.Errorf("a subpath both in the vcs_url qualifier %q and after # (%q)", location, subpath)
	default:
		location += "#" + subpath
	}
	return location, nil
}

// versionControl names the version-control systems a vcs_url may begin with:
// the ones the pkg:git/, pkg:svn/ and pkg:hg/ spellings name.
var versionControl = map[string]bool{"git": true, "svn": true, "hg": true}

// spellsVCSURL reports whether s is a version-control URL as ECMA-427 Annex
// B.3.5 writes one: a <vcs>[+<transport>] scheme naming a known version-control
// system, then a host and a path; a repository on this machine
// (<vcs>+file:///path) has a path and no host.
func spellsVCSURL(s string) bool {
	scheme, rest, ok := strings.Cut(s, "://")
	if !ok {
		return false
	}
	vcs, transport, _ := strings.Cut(strings.ToLower(scheme), "+")
	if !versionControl[vcs] || (transport != "" && !isScheme(transport)) {
		return false
	}
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	host, path, ok := strings.Cut(rest, "/")
	if host == "" && transport != "file" {
		return false
	}
	return ok && strings.Trim(path, "/") != ""
}

// isScheme reports whether s is a URI scheme: a letter, then letters, digits, + - or .
func isScheme(s string) bool {
	for i, r := range strings.ToLower(s) {
		letter := r >= 'a' && r <= 'z'
		if !letter && (i == 0 || !(r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.')) {
			return false
		}
	}
	return s != ""
}

// cutPrefixFold is strings.CutPrefix comparing the prefix without regard to case.
func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

// resourceSegments splits a resource IRI into the path segments that name the
// project, without its scheme, query, package-URL version or archive extension,
// nor the .git suffix of a resource spelling a repository; a resource whose
// location does not decode names nothing. A fragment names a project directory
// within the repository the path names (pkg:git/host/acme/widgets#packages/gears):
// that project is named by the fragment's last segment, under the repository's
// owner. An archive holds no directory a fragment could name, so one on its IRI is passed over.
func resourceSegments(resource string) []string {
	location, err := resourceLocation(resource)
	if err != nil {
		return nil
	}
	repository := spellsLocation(resource, location)
	resource = location
	fragment := ""
	if i := strings.IndexByte(resource, '#'); i >= 0 {
		resource, fragment = resource[:i], resource[i+1:]
	}
	if i := strings.IndexByte(resource, '?'); i >= 0 {
		resource = resource[:i]
	}
	if i := strings.Index(resource, "://"); i >= 0 {
		resource = resource[i+3:]
	} else if i := strings.IndexByte(resource, ':'); i >= 0 {
		resource = resource[i+1:]
	}
	out := pathSegments(resource)
	if n := len(out); n > 0 {
		last := out[n-1]
		if i := strings.IndexByte(last, '@'); i > 0 {
			last = last[:i]
		}
		archive := strings.EqualFold(path.Ext(last), ArchiveExt)
		if archive {
			last = last[:len(last)-len(ArchiveExt)]
		}
		if repository {
			last = strings.TrimSuffix(last, ".git")
		}
		if sub := pathSegments(fragment); len(sub) > 0 && !archive {
			last = sub[len(sub)-1]
		}
		out[n-1] = last
	}
	return out
}

// pathSegments splits a slash-separated path into its non-empty segments,
// each percent-decoded where it decodes.
func pathSegments(p string) []string {
	var out []string
	for _, seg := range strings.Split(strings.Trim(p, "/"), "/") {
		if decoded, err := url.PathUnescape(seg); err == nil {
			seg = decoded
		}
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// slug folds a name to what a registry spells in a path: lower case, with every
// run of characters other than letters and digits written as one hyphen.
func slug(name string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			hyphen = false
		case b.Len() > 0 && !hyphen:
			b.WriteByte('-')
			hyphen = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// constraint is the version a usage asks for: its versionConstraint, else the one
// written after @ in its resource IRI (pkg:…/widgets@1.2.0); empty for any version.
func (u Usage) constraint() string {
	if c := strings.TrimSpace(u.VersionConstraint); c != "" {
		return c
	}
	return resourceVersion(u.Resource)
}

// resourceVersion is the version written after @ in the last path segment of a
// resource IRI (before any query or fragment, after a trailing slash); empty
// when none is. For a pkg:generic resource naming a repository the version is
// the one in its vcs_url, else the one after @ in the purl's name.
func resourceVersion(resource string) string {
	location, err := resourceLocation(resource)
	if err != nil {
		return ""
	}
	if location != resource {
		if v := pathVersion(location); v != "" {
			return v
		}
	}
	return pathVersion(resource)
}

// pathVersion is the version written after @ in the last path segment of an
// IRI, before any query or fragment and after a trailing slash.
func pathVersion(resource string) string {
	if i := strings.IndexAny(resource, "?#"); i >= 0 {
		resource = resource[:i]
	}
	resource = strings.TrimRight(resource, "/")
	last := resource[strings.LastIndexByte(resource, '/')+1:]
	if i := strings.LastIndexByte(last, '@'); i > 0 {
		if decoded, err := url.PathUnescape(last[i+1:]); err == nil {
			return decoded
		}
		return last[i+1:]
	}
	return ""
}

// versionRange is the usage's constraint parsed; an error wrapping errBadConstraint
// when it does not follow the grammar.
func (u Usage) versionRange() (versionRange, error) { return parseVersionRange(u.constraint()) }

// satisfies reports whether the project's version lies in the range.
func (p *Interchange) satisfies(r versionRange) bool { return r.matches(p.Version) }

// newerVersion reports whether a is a later version than b, in semantic
// versioning's order: the dot-separated numeric core first, then a release
// over a pre-release, then the pre-release identifiers (numeric ones
// numerically, before alphanumeric ones); build metadata counts for nothing.
// A segment that is not a number compares as text, so any string has a place.
func newerVersion(a, b string) bool {
	coreA, preA := splitVersion(a)
	coreB, preB := splitVersion(b)
	if c := compareIdentifiers(coreA, coreB, true); c != 0 {
		return c > 0
	}
	switch {
	case len(preA) == 0 && len(preB) == 0:
		return false
	case len(preA) == 0 || len(preB) == 0:
		return len(preA) == 0
	}
	return compareIdentifiers(preA, preB, false) > 0
}

// splitVersion separates a version into its dotted core and its dotted
// pre-release identifiers, dropping build metadata and a leading v.
func splitVersion(v string) (core, pre []string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	rest := ""
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, rest = v[:i], v[i+1:]
	}
	core = strings.Split(v, ".")
	if rest != "" {
		pre = strings.Split(rest, ".")
	}
	return core, pre
}

// compareIdentifiers orders two identifier lists segment by segment: numbers
// numerically, a number before text, text as text; a missing segment counts as
// zero in a core (1.0 is 1.0.0) and as earlier in a pre-release (1.0.0-rc is
// before 1.0.0-rc.1).
func compareIdentifiers(as, bs []string, core bool) int {
	for i := 0; i < len(as) || i < len(bs); i++ {
		a, b := "", ""
		if i < len(as) {
			a = as[i]
		}
		if i < len(bs) {
			b = bs[i]
		}
		if a == b {
			continue
		}
		if a == "" || b == "" {
			if core {
				a, b = zeroIfEmpty(a), zeroIfEmpty(b)
				if a == b {
					continue
				}
			} else if a == "" {
				return -1
			} else {
				return 1
			}
		}
		aNum, bNum := isDigits(a), isDigits(b)
		switch {
		case aNum && bNum:
			if c := compareNumbers(a, b); c != 0 {
				return c
			}
		case aNum:
			return -1
		case bNum:
			return 1
		default:
			return strings.Compare(a, b)
		}
	}
	return 0
}

// isDigits reports whether s is a non-empty run of decimal digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// compareNumbers orders two digit strings by value, however long: leading
// zeros dropped, the longer is the larger, equal lengths compare as text.
func compareNumbers(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

func zeroIfEmpty(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
