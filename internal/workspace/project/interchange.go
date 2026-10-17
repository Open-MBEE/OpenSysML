package project

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ProjectFile is the file every OMG project interchange directory holds beside
// its model files (KerML 1.0 §10.3): a directory holding one is a project.
const ProjectFile = ".project.json"

// archiveExt is the extension of a project interchange archive, which a
// resource IRI may end in; it is no part of the project's name.
const archiveExt = ".kpar"

// Interchange is the InterchangeProject information a project's .project.json
// carries (KerML 1.0 Table 12), with the directory it was read from.
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
	var raw struct {
		Name        string  `json:"name"`
		Publisher   string  `json:"publisher"`
		Description string  `json:"description"`
		Version     string  `json:"version"`
		License     string  `json:"license"`
		Usage       []Usage `json:"usage"`
	}
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("%s: the project has no name", file)
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

// EnclosingProjects lists the directories at or above path that are projects,
// the nearest first; path names a file or a directory, and need not exist.
func EnclosingProjects(path string) []string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	dir := abs
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		dir = filepath.Dir(abs)
	}
	var out []string
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
// an archive are not part of the name.
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

// resourceSegments splits a resource IRI into the path segments that name the
// project, without its scheme, query, fragment, package-URL version or
// archive extension.
func resourceSegments(resource string) []string {
	if i := strings.IndexAny(resource, "?#"); i >= 0 {
		resource = resource[:i]
	}
	if i := strings.Index(resource, "://"); i >= 0 {
		resource = resource[i+3:]
	} else if i := strings.IndexByte(resource, ':'); i >= 0 {
		resource = resource[i+1:]
	}
	resource = strings.Trim(resource, "/")
	if resource == "" {
		return nil
	}
	var out []string
	for _, seg := range strings.Split(resource, "/") {
		if decoded, err := url.PathUnescape(seg); err == nil {
			seg = decoded
		}
		if seg != "" {
			out = append(out, seg)
		}
	}
	if n := len(out); n > 0 {
		last := out[n-1]
		if i := strings.IndexByte(last, '@'); i > 0 {
			last = last[:i]
		}
		if strings.EqualFold(path.Ext(last), archiveExt) {
			last = last[:len(last)-len(archiveExt)]
		}
		out[n-1] = last
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

// satisfies reports whether the project's version is the one the constraint
// names exactly; only an exact version (optionally written =, == or v first)
// is understood, any other constraint is not decided here.
func (p *Interchange) satisfies(constraint string) bool {
	want := strings.TrimSpace(constraint)
	want = strings.TrimPrefix(strings.TrimPrefix(want, "=="), "=")
	want = strings.TrimPrefix(strings.TrimSpace(want), "v")
	return want != "" && want == strings.TrimPrefix(p.Version, "v")
}

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
