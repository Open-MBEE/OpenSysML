package project

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// OriginKind is where a usage's project is fetched from.
type OriginKind int

const (
	// OriginHost is a package URL naming a repository on a git host
	// (pkg:github/<org>/<repo>, pkg:gitlab/<group>/<repo>), released as a tag.
	OriginHost OriginKind = iota
	// OriginGit is a git URL (https://…/.git, git@…, ssh://…), released as a tag.
	OriginGit
	// OriginArchive is a URL of a project interchange archive.
	OriginArchive
	// OriginLocal is a path on this machine: a project directory or an
	// archive, read in place ("editable"), never copied.
	OriginLocal
	// OriginStandard is a standard library IRI the bundled library answers;
	// nothing is installed for it.
	OriginStandard
)

// String names the kind as the lock and the diagnostics spell it.
func (k OriginKind) String() string {
	switch k {
	case OriginHost:
		return "host"
	case OriginGit:
		return "git"
	case OriginArchive:
		return "archive"
	case OriginLocal:
		return "local"
	case OriginStandard:
		return "standard"
	}
	return fmt.Sprintf("OriginKind(%d)", int(k))
}

// Origin is a usage's resource IRI read as the place to fetch its project
// from and the version wanted of it.
type Origin struct {
	// IRI is the resource as written, less a version written after @.
	IRI string
	// Kind is how the project is fetched.
	Kind OriginKind
	// Host is the host type of a OriginHost (github, gitlab) and the host
	// name of a OriginGit or OriginArchive.
	Host string
	// Repo is the repository path on the host, as org/repo, for OriginHost
	// and OriginGit; the URL for OriginArchive; the path for OriginLocal.
	Repo string
	// Subpath is the directory within the repository holding the project
	// (a purl subpath, written after #), empty for the root.
	Subpath string
	// Constraint is the version constraint: the version written after @ in
	// the IRI, else the usage's versionConstraint; empty for any version.
	Constraint string
}

// hostTypes maps the purl types of the git hosts a release is fetched from
// to the host they live on. Any other purl type is refused by name.
var hostTypes = map[string]string{
	"github": "github.com",
	"gitlab": "gitlab.com",
}

// ErrNotAnOrigin is the error a resource that names no fetchable origin wraps.
var ErrNotAnOrigin = errors.New("not a source a project can be installed from")

// ParseOrigin reads a usage as the source to install its project from. A
// version written after @ in the IRI and a versionConstraint that disagree
// are an error, as are a purl type that is not a git host, a URL whose
// scheme is not one of https, http, ssh or git, and a resource that is none
// of these and no path either.
func ParseOrigin(u Usage) (*Origin, error) {
	resource := strings.TrimSpace(u.Resource)
	if resource == "" {
		return nil, fmt.Errorf("usage names no resource: %w", ErrNotAnOrigin)
	}
	if IsStandardLibraryResource(resource) {
		return &Origin{IRI: resource, Kind: OriginStandard, Constraint: strings.TrimSpace(u.VersionConstraint)}, nil
	}
	var src *Origin
	var err error
	switch {
	case strings.HasPrefix(resource, "pkg:"):
		src, err = parsePurl(resource)
	case strings.Contains(resource, "://"):
		src, err = parseURL(resource)
	case strings.HasPrefix(resource, "git@") && strings.Contains(resource, ":"):
		src, err = parseSCP(resource)
	default:
		src = &Origin{IRI: resource, Kind: OriginLocal, Repo: filepath.Clean(resource)}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", resource, err)
	}
	constraint := strings.TrimSpace(u.VersionConstraint)
	switch {
	case src.Constraint == "":
		src.Constraint = constraint
	case constraint != "" && !sameConstraint(src.Constraint, constraint):
		return nil, fmt.Errorf("%s: the IRI names version %s and versionConstraint %q another; state the version once", resource, src.Constraint, constraint)
	}
	return src, nil
}

// sameConstraint reports whether two constraints spell the same exact version.
func sameConstraint(a, b string) bool {
	return exactVersion(a) != "" && exactVersion(a) == exactVersion(b)
}

// parsePurl reads a package URL: pkg:<type>/<namespace>/<name>[@<version>][?qualifiers][#<subpath>].
func parsePurl(resource string) (*Origin, error) {
	rest := strings.TrimPrefix(resource, "pkg:")
	rest = strings.TrimLeft(rest, "/")
	subpath := ""
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest, subpath = rest[:i], rest[i+1:]
	}
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		rest = rest[:i]
	}
	version := ""
	if i := strings.LastIndexByte(rest, '@'); i >= 0 {
		rest, version = rest[:i], rest[i+1:]
		if version == "" {
			return nil, errors.New("the @ names no version")
		}
		if decoded, err := url.PathUnescape(version); err == nil {
			version = decoded
		}
	}
	segments := strings.Split(rest, "/")
	if len(segments) < 1 || segments[0] == "" {
		return nil, errors.New("the package URL names no type")
	}
	host, ok := hostTypes[strings.ToLower(segments[0])]
	if !ok {
		return nil, fmt.Errorf("package URL type %q is not a git host projects are installed from (%s): %w", segments[0], strings.Join(sortedKeys(hostTypes), ", "), ErrNotAnOrigin)
	}
	var parts []string
	for _, seg := range segments[1:] {
		if decoded, err := url.PathUnescape(seg); err == nil {
			seg = decoded
		}
		if seg != "" {
			parts = append(parts, seg)
		}
	}
	if len(parts) < 2 {
		return nil, fmt.Errorf("pkg:%s/<namespace>/<name> needs both a namespace and a name", segments[0])
	}
	sub, err := cleanSubpath(subpath)
	if err != nil {
		return nil, err
	}
	iri := "pkg:" + strings.ToLower(segments[0]) + "/" + strings.Join(parts, "/")
	return &Origin{IRI: iri, Kind: OriginHost, Host: host, Repo: strings.Join(parts, "/"), Subpath: sub, Constraint: version}, nil
}

// parseURL reads a URL as a git repository, when it ends in .git or its
// scheme is git, ssh or file, else as an archive when it ends in .kpar; a
// version written after @ in the last path segment selects the tag.
func parseURL(resource string) (*Origin, error) {
	u, err := url.Parse(resource)
	if err != nil {
		return nil, err
	}
	if u.Host == "" && u.Scheme != "file" {
		return nil, errors.New("the URL names no host")
	}
	version := ""
	p := u.Path
	if i := strings.LastIndexByte(path.Base(p), '@'); i > 0 {
		base := path.Base(p)
		version = base[i+1:]
		p = path.Join(path.Dir(p), base[:i])
	}
	subpath, err := cleanSubpath(u.Fragment)
	if err != nil {
		return nil, err
	}
	bare := *u
	bare.Path, bare.Fragment, bare.RawQuery, bare.RawPath = p, "", "", ""
	switch scheme := strings.ToLower(u.Scheme); {
	case scheme == "git" || scheme == "ssh" || scheme == "file" || strings.HasSuffix(strings.ToLower(p), ".git"):
		return &Origin{IRI: bare.String(), Kind: OriginGit, Host: u.Host, Repo: bare.String(), Subpath: subpath, Constraint: version}, nil
	case (scheme == "https" || scheme == "http") && IsArchive(p):
		if subpath != "" {
			return nil, errors.New("an archive URL takes no #subpath; the archive is the project")
		}
		bare.RawQuery = u.RawQuery
		return &Origin{IRI: bare.String(), Kind: OriginArchive, Host: u.Host, Repo: bare.String(), Constraint: version}, nil
	case scheme == "https" || scheme == "http":
		return nil, fmt.Errorf("a URL is fetched as a git repository when it ends in .git or as an archive when it ends in %s: %w", ArchiveExt, ErrNotAnOrigin)
	default:
		return nil, fmt.Errorf("URL scheme %q is not one projects are installed over (https, http, ssh, git, file): %w", u.Scheme, ErrNotAnOrigin)
	}
}

// parseSCP reads git's scp-like form, git@host:org/repo.git[@version][#subpath].
func parseSCP(resource string) (*Origin, error) {
	rest, subpath := resource, ""
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest, subpath = rest[:i], rest[i+1:]
	}
	colon := strings.IndexByte(rest, ':')
	host := strings.TrimPrefix(rest[:colon], "git@")
	repoPath := rest[colon+1:]
	version := ""
	if i := strings.LastIndexByte(repoPath, '@'); i > 0 {
		repoPath, version = repoPath[:i], repoPath[i+1:]
	}
	if host == "" || repoPath == "" {
		return nil, errors.New("git@<host>:<path> needs both a host and a path")
	}
	sub, err := cleanSubpath(subpath)
	if err != nil {
		return nil, err
	}
	iri := "git@" + host + ":" + repoPath
	return &Origin{IRI: iri, Kind: OriginGit, Host: host, Repo: iri, Subpath: sub, Constraint: version}, nil
}

// cleanSubpath checks a purl subpath: slash-separated, relative, no . or ..
// segments, so it names a directory inside the repository.
func cleanSubpath(sub string) (string, error) {
	if decoded, err := url.PathUnescape(sub); err == nil {
		sub = decoded
	}
	sub = strings.Trim(sub, "/")
	if sub == "" {
		return "", nil
	}
	if _, err := NormalizeMemberPath(sub); err != nil {
		return "", fmt.Errorf("subpath: %w", err)
	}
	return sub, nil
}

// IsStandardLibraryResource reports whether the resource is an OMG standard
// library archive IRI, which the bundled library answers and no installer
// fetches.
func IsStandardLibraryResource(resource string) bool {
	lower := strings.ToLower(strings.TrimSpace(resource))
	return (strings.HasPrefix(lower, "https://www.omg.org/spec/kerml/") || strings.HasPrefix(lower, "https://www.omg.org/spec/sysml/")) &&
		strings.HasSuffix(lower, ArchiveExt)
}

// InstallName is the directory (or archive, with ArchiveExt) an installed
// project takes under the library directory: <slug(publisher)>-<slug(name)>_<version>,
// or <slug(name)>_<version> for a project with no publisher.
func InstallName(p *Interchange) string {
	name := slug(p.Name)
	if p.Publisher != "" {
		name = slug(p.Publisher) + "-" + name
	}
	return name + "_" + strings.TrimPrefix(p.Version, "v")
}
