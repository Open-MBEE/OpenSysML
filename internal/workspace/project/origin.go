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
	// OriginGit is a git repository on any host, released as a tag; its
	// identity is pkg:git/<host>/<path>[#<subpath>].
	OriginGit OriginKind = iota
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

// GitPurlType is the package URL type of a git repository: pkg:git/<host>/<path>.
const GitPurlType = "git"

// Origin is a usage's resource IRI read as the place to fetch its project
// from and the version wanted of it.
type Origin struct {
	// IRI is the resource's identity, less a version written after @: for a
	// git repository the canonical pkg:git/<host>/<path>[#<subpath>], which
	// a clone URL normalizes to; for anything else the resource as written.
	IRI string
	// Kind is how the project is fetched.
	Kind OriginKind
	// Host is the host name of a git repository or archive URL; empty for a
	// repository on this machine (pkg:git//<path>, file://).
	Host string
	// Path is the repository path on the host, less any .git, for a git
	// repository; the URL for an archive; the path for a local project.
	Path string
	// Remote is the URL a git repository is cloned from: the clone URL as
	// written, or https://<host>/<path>.git derived from the pkg:git form
	// (file://<path> when the host is empty).
	Remote string
	// Subpath is the directory within the repository holding the project
	// (written after #), empty for the root.
	Subpath string
	// Constraint is the version constraint: the version written after @ in
	// the IRI, else the usage's versionConstraint; empty for any version.
	Constraint string
}

// ErrNotAnOrigin is the error a resource that names no fetchable origin wraps.
var ErrNotAnOrigin = errors.New("not a source a project can be installed from")

// ParseOrigin reads a usage as the source to install its project from. A
// version written after @ in the IRI and a versionConstraint that disagree
// are an error, as are a package URL whose type is not git, a URL whose
// scheme is not one of https, http, ssh, git or file, and a resource that
// is none of these and no path either.
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
		src = &Origin{IRI: resource, Kind: OriginLocal, Path: filepath.Clean(resource)}
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

// gitOrigin builds the origin of a git repository from its host, path
// (less .git) and subpath, deriving the identity, and the remote when none
// was written.
func gitOrigin(host, repoPath, remote, subpath, version string) (*Origin, error) {
	host = strings.ToLower(host)
	repoPath = strings.TrimSuffix(strings.TrimRight(repoPath, "/"), ".git")
	if host == "" {
		repoPath = "/" + strings.TrimLeft(repoPath, "/")
	} else {
		repoPath = strings.TrimLeft(repoPath, "/")
	}
	if strings.Trim(repoPath, "/") == "" {
		return nil, errors.New("the repository has no path")
	}
	sub, err := cleanSubpath(subpath)
	if err != nil {
		return nil, err
	}
	iri := "pkg:" + GitPurlType + "/" + host + "/" + repoPath
	if host == "" {
		iri = "pkg:" + GitPurlType + "/" + repoPath
	}
	if sub != "" {
		iri += "#" + sub
	}
	if remote == "" {
		if host == "" {
			remote = "file://" + repoPath
		} else {
			remote = "https://" + host + "/" + repoPath + ".git"
		}
	}
	return &Origin{IRI: iri, Kind: OriginGit, Host: host, Path: repoPath, Remote: remote, Subpath: sub, Constraint: version}, nil
}

// parsePurl reads a package URL, pkg:git/<host>/<path>[@<version>][?qualifiers][#<subpath>];
// pkg:git//<path> is a repository on this machine. Any other type is refused.
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
	typ, rest, _ := strings.Cut(rest, "/")
	if typ == "" {
		return nil, errors.New("the package URL names no type")
	}
	if !strings.EqualFold(typ, GitPurlType) {
		return nil, fmt.Errorf("package URL type %q is not %s, the one projects are installed from (pkg:git/<host>/<path>): %w", typ, GitPurlType, ErrNotAnOrigin)
	}
	host, repoPath, _ := strings.Cut(rest, "/")
	if decoded, err := url.PathUnescape(repoPath); err == nil {
		repoPath = decoded
	}
	if host != "" && !strings.Contains(repoPath, "/") {
		return nil, errors.New("pkg:git/<host>/<path> needs a host and a path of at least two segments (an owner and a repository)")
	}
	if host == "" {
		repoPath = "/" + strings.TrimLeft(repoPath, "/")
	}
	return gitOrigin(host, repoPath, "", subpath, version)
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
	bare := *u
	bare.Path, bare.Fragment, bare.RawQuery, bare.RawPath = p, "", "", ""
	switch scheme := strings.ToLower(u.Scheme); {
	case scheme == "git" || scheme == "ssh" || scheme == "file" || strings.HasSuffix(strings.ToLower(p), ".git"):
		host := u.Hostname()
		if scheme == "file" {
			host = ""
			p = "/" + strings.TrimLeft(p, "/")
			bare.Host = ""
		} else if port := u.Port(); port != "" {
			host += ":" + port
		}
		return gitOrigin(host, p, bare.String(), u.Fragment, version)
	case (scheme == "https" || scheme == "http") && IsArchive(p):
		if u.Fragment != "" {
			return nil, errors.New("an archive URL takes no #subpath; the archive is the project")
		}
		bare.RawQuery = u.RawQuery
		return &Origin{IRI: bare.String(), Kind: OriginArchive, Host: u.Host, Path: bare.String(), Constraint: version}, nil
	case scheme == "https" || scheme == "http":
		return nil, fmt.Errorf("a URL is fetched as a git repository when it ends in .git or as an archive when it ends in %s: %w", ArchiveExt, ErrNotAnOrigin)
	default:
		return nil, fmt.Errorf("URL scheme %q is not one projects are installed over (https, http, ssh, git, file): %w", u.Scheme, ErrNotAnOrigin)
	}
}

// parseSCP reads git's scp-like form, git@host:path[.git][@version][#subpath].
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
	return gitOrigin(host, repoPath, "git@"+host+":"+repoPath, subpath, version)
}

// cleanSubpath checks a subpath: slash-separated, relative, no . or ..
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

// Identity is the resource IRI a usage is locked and matched under: the
// canonical pkg:git form of a git repository, the resource itself otherwise.
func Identity(u Usage) string {
	if o, err := ParseOrigin(u); err == nil {
		return o.IRI
	}
	return strings.TrimSpace(u.Resource)
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
