package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"slices"
	"sort"
	"strings"
	"time"
)

// InstallError is the error every step of installing a usage reports: which
// IRI, which step (resolve, fetch, verify, install, lock) and why.
type InstallError struct {
	IRI  string
	Step string
	Err  error
}

func (e *InstallError) Error() string {
	return fmt.Sprintf("cannot install %s: %s: %v", e.IRI, e.Step, e.Err)
}

func (e *InstallError) Unwrap() error { return e.Err }

func installErr(iri, step string, err error) error {
	var ie *InstallError
	if errors.As(err, &ie) {
		return err
	}
	return &InstallError{IRI: iri, Step: step, Err: err}
}

// ErrRedirectOffHost is the error a fetch wraps when the server redirects to
// another host, which is never followed.
var ErrRedirectOffHost = errors.New("redirect leaves the host")

// hostType is a git host releases are fetched from: the URL of the archive
// of a tag, relative to the host's base URL.
type hostType struct {
	archiveURL func(base, repo, tag string) string
}

// hostArchives is the host table; every host-specific path lives here.
var hostArchives = map[string]hostType{
	"github.com": {archiveURL: func(base, repo, tag string) string {
		return base + "/" + repo + "/archive/refs/tags/" + url.PathEscape(tag) + ".tar.gz"
	}},
	"gitlab.com": {archiveURL: func(base, repo, tag string) string {
		return base + "/" + repo + "/-/archive/" + url.PathEscape(tag) + "/" + url.PathEscape(path.Base(repo)+"-"+tag) + ".tar.gz"
	}},
}

// Fetcher fetches the releases of usages. The zero value fetches over HTTPS
// honouring HTTPS_PROXY and shells out to git; the fields redirect it for tests.
type Fetcher struct {
	// Client makes the HTTP requests; nil uses a client that honours the
	// proxy environment and refuses redirects off the host.
	Client *http.Client
	// Git is the git executable; "" is git on PATH.
	Git string
	// HostURLs overrides the base URL (https://<host>) archives are fetched
	// from, keyed by host.
	HostURLs map[string]string
	// GitURLs overrides the base URL (https://<host>) a host's repositories
	// are cloned from, keyed by host.
	GitURLs map[string]string
	// MaxBytes bounds every download and extracted tree; 0 is project.MaxArchiveSize.
	MaxBytes int64
	// Timeout bounds every request and git command; 0 is five minutes.
	Timeout time.Duration
}

// Release is one resolved and verified version of a usage's project.
type Release struct {
	Origin *project.Origin
	// Version is the project's version, as .project.json spells it.
	Version string
	// Tag and Commit name the release in its repository; empty for an
	// archive or a local path.
	Tag, Commit string
	// SourceURL is where the release was fetched from.
	SourceURL string
	// Digest is sha256:<hex> of the fetched tree (sorted members) or archive.
	Digest string
	// Archive is the verified archive of the release; nil for a local path,
	// which is read in place at Path.
	Archive []byte
	Path    string
	Project *project.Interchange
}

// Installable reports whether anything is copied for the release: a local
// path is read in place.
func (r *Release) Installable() bool { return r.Archive != nil }

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return &http.Client{
		Transport:     &http.Transport{Proxy: http.ProxyFromEnvironment},
		CheckRedirect: sameHostRedirects,
	}
}

// sameHostRedirects follows at most ten redirects, none to another host.
func sameHostRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("more than ten redirects")
	}
	if from := via[0].URL.Host; !strings.EqualFold(req.URL.Host, from) {
		return fmt.Errorf("%w: %s redirects to %s", ErrRedirectOffHost, via[0].URL, req.URL)
	}
	return nil
}

func (f *Fetcher) maxBytes() int64 {
	if f.MaxBytes > 0 {
		return f.MaxBytes
	}
	return project.MaxArchiveSize
}

func (f *Fetcher) timeout() time.Duration {
	if f.Timeout > 0 {
		return f.Timeout
	}
	return 5 * time.Minute
}

func (f *Fetcher) baseURL(table map[string]string, host string) string {
	if base, ok := table[host]; ok {
		return strings.TrimRight(base, "/")
	}
	return "https://" + host
}

// cloneURL is the URL a host origin's repository is cloned from.
func (f *Fetcher) cloneURL(o *project.Origin) string {
	if o.Kind == project.OriginGit {
		return o.Repo
	}
	return f.baseURL(f.GitURLs, o.Host) + "/" + o.Repo + ".git"
}

// Fetch resolves the version of the usage the constraint admits and fetches
// and verifies its project. A local path is read in place.
func (f *Fetcher) Fetch(ctx context.Context, o *project.Origin, c project.Constraint) (*Release, error) {
	var rel *Release
	var err error
	switch o.Kind {
	case project.OriginStandard:
		return nil, installErr(o.IRI, "resolve", errors.New("a standard library IRI is answered by the bundled library and is never installed"))
	case project.OriginLocal:
		rel, err = f.fetchLocal(o)
	case project.OriginArchive:
		rel, err = f.fetchArchive(ctx, o)
	case project.OriginHost, project.OriginGit:
		rel, err = f.fetchTagged(ctx, o, c)
	default:
		return nil, installErr(o.IRI, "resolve", fmt.Errorf("origin kind %s is not fetched", o.Kind))
	}
	if err != nil {
		return nil, err
	}
	if !c.Any() && !c.Matches(rel.Project.Version) {
		return nil, installErr(o.IRI, "resolve", fmt.Errorf("%s is version %s, which does not satisfy %s", o.IRI, rel.Project.Version, c))
	}
	rel.Version = rel.Project.Version
	return rel, nil
}

// fetchLocal reads a project directory or archive on this machine in place.
func (f *Fetcher) fetchLocal(o *project.Origin) (*Release, error) {
	var p *project.Interchange
	var err error
	if project.IsArchive(o.Repo) {
		var a *project.Archive
		a, err = project.OpenArchive(o.Repo)
		if err == nil {
			p = a.Project
		}
	} else {
		p, err = project.ReadInterchange(o.Repo)
		if errors.Is(err, os.ErrNotExist) {
			err = fmt.Errorf("not a project interchange: %s has no %s", o.Repo, project.ProjectFile)
		}
	}
	if err != nil {
		return nil, installErr(o.IRI, "verify", err)
	}
	return &Release{Origin: o, SourceURL: o.Repo, Path: o.Repo, Project: p}, nil
}

// fetchArchive downloads an archive and verifies it as the reader does.
func (f *Fetcher) fetchArchive(ctx context.Context, o *project.Origin) (*Release, error) {
	data, err := f.get(ctx, o.Repo)
	if err != nil {
		return nil, installErr(o.IRI, "fetch", err)
	}
	a, err := project.ParseArchive(o.Repo, data)
	if err != nil {
		return nil, installErr(o.IRI, "verify", err)
	}
	return &Release{Origin: o, SourceURL: o.Repo, Digest: digest(data), Archive: data, Project: a.Project}, nil
}

// get downloads a URL, bounded to MaxBytes.
func (f *Fetcher) get(ctx context.Context, rawURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, f.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	limit := f.maxBytes()
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("GET %s: more than %d bytes; the download is bounded", rawURL, limit)
	}
	return data, nil
}

// fetchTagged resolves the tags of a repository to the version the
// constraint admits and fetches that tag's tree.
func (f *Fetcher) fetchTagged(ctx context.Context, o *project.Origin, c project.Constraint) (*Release, error) {
	tags, err := f.tags(ctx, o)
	if err != nil {
		return nil, installErr(o.IRI, "resolve", err)
	}
	versions := make([]string, 0, len(tags))
	for v := range tags {
		versions = append(versions, v)
	}
	version := c.Latest(versions)
	if version == "" {
		sort.Strings(versions)
		return nil, installErr(o.IRI, "resolve", fmt.Errorf("no release tag of %s satisfies %q; the tags name versions %s", o.IRI, c, strings.Join(versions, ", ")))
	}
	tag := tags[version]
	rel := &Release{Origin: o, Tag: tag.name, Commit: tag.commit}
	members, err := f.tree(ctx, o, tag.name, rel)
	if err != nil {
		return nil, installErr(o.IRI, "fetch", err)
	}
	if o.Subpath != "" {
		members = subtree(members, o.Subpath)
	}
	at := o.IRI + "@" + tag.name
	if o.Subpath != "" {
		at += "#" + o.Subpath
	}
	content, ok := members[project.ProjectFile]
	if !ok {
		return nil, installErr(o.IRI, "verify", fmt.Errorf("not a project interchange: %s has no %s", at, project.ProjectFile))
	}
	p, err := project.ParseInterchange(at, at+"/"+project.ProjectFile, content)
	if err != nil {
		return nil, installErr(o.IRI, "verify", err)
	}
	if got, _ := project.CanonicalVersion(p.Version); got != version {
		return nil, installErr(o.IRI, "verify", fmt.Errorf("%s is tagged %s but its %s says version %q; the tag and the file must agree", at, tag.name, project.ProjectFile, p.Version))
	}
	rel.Digest = treeDigest(members)
	rel.Archive, err = project.ArchiveOf(at, members)
	if err != nil {
		return nil, installErr(o.IRI, "verify", err)
	}
	rel.Project = p
	return rel, nil
}

type tagRef struct{ name, commit string }

// tags lists the release tags of a repository (v<semver> or <semver>) with
// the commit each names, keyed by canonical version.
func (f *Fetcher) tags(ctx context.Context, o *project.Origin) (map[string]tagRef, error) {
	out, err := f.git(ctx, "", "ls-remote", "--tags", "--", f.cloneURL(o))
	if err != nil {
		return nil, err
	}
	tags := map[string]tagRef{}
	peeled := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "refs/tags/") {
			continue
		}
		name := strings.TrimPrefix(fields[1], "refs/tags/")
		if strings.HasSuffix(name, "^{}") {
			peeled[strings.TrimSuffix(name, "^{}")] = fields[0]
			continue
		}
		version, err := project.CanonicalVersion(name)
		if err != nil || strings.TrimPrefix(name, "v") != version {
			continue
		}
		if other, dup := tags[version]; dup {
			return nil, fmt.Errorf("tags %s and %s of %s both name version %s", other.name, name, o.IRI, version)
		}
		tags[version] = tagRef{name: name, commit: fields[0]}
	}
	for version, tag := range tags {
		if commit, ok := peeled[tag.name]; ok {
			tags[version] = tagRef{name: tag.name, commit: commit}
		}
	}
	return tags, nil
}

// tree fetches the files of a tag: the host's archive over HTTPS when the
// origin names a host in the table, else (or when that fails) a shallow
// clone of the tag.
func (f *Fetcher) tree(ctx context.Context, o *project.Origin, tag string, rel *Release) (map[string][]byte, error) {
	var archiveErr error
	if host, ok := hostArchives[o.Host]; ok && o.Kind == project.OriginHost {
		archiveURL := host.archiveURL(f.baseURL(f.HostURLs, o.Host), o.Repo, tag)
		data, err := f.get(ctx, archiveURL)
		if err == nil {
			members, err := extractTarGz(data, f.maxBytes())
			if err != nil {
				return nil, fmt.Errorf("%s: %w", archiveURL, err)
			}
			rel.SourceURL = archiveURL
			return members, nil
		}
		if errors.Is(err, ErrRedirectOffHost) {
			return nil, err
		}
		archiveErr = err
	}
	members, err := f.clone(ctx, o, tag)
	if err != nil {
		if archiveErr != nil {
			return nil, fmt.Errorf("%w; the clone after it also failed: %v", archiveErr, err)
		}
		return nil, err
	}
	rel.SourceURL = f.cloneURL(o)
	return members, nil
}

// clone makes a shallow clone of a tag and reads its files.
func (f *Fetcher) clone(ctx context.Context, o *project.Origin, tag string) (map[string][]byte, error) {
	dir, err := os.MkdirTemp("", "sysml-install-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if _, err := f.git(ctx, "", "clone", "--quiet", "--depth", "1", "--branch", tag, "--", f.cloneURL(o), dir); err != nil {
		return nil, err
	}
	members, err := project.BundleMembers(dir)
	if err != nil {
		return nil, err
	}
	var total int64
	for _, content := range members {
		if total += int64(len(content)); total > f.maxBytes() {
			return nil, fmt.Errorf("the tree of %s holds more than %d bytes; the install is bounded", tag, f.maxBytes())
		}
	}
	return members, nil
}

// git runs a git command without prompting, its stderr in the error.
func (f *Fetcher) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, f.timeout())
	defer cancel()
	exe := f.Git
	if exe == "" {
		exe = "git"
	}
	cmd := exec.CommandContext(ctx, exe, args...) // #nosec G204 -- the arguments are git's, the URL the usage names.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", args[0], msg)
	}
	return out, nil
}

// extractTarGz reads a host's tag archive: the files under its single
// top-level directory, hidden entries skipped as an archive of a directory
// skips them, every path checked, links and specials left out, the whole
// bounded.
func extractTarGz(data []byte, limit int64) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("not a gzip archive: %w", err)
	}
	tr := tar.NewReader(gz)
	members := map[string][]byte{}
	var total int64
	top := ""
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("not a tar archive: %w", err)
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "" || name == "pax_global_header" {
			continue
		}
		first, rest, _ := strings.Cut(strings.Trim(name, "/"), "/")
		if top == "" {
			top = first
		}
		if first != top {
			return nil, fmt.Errorf("the archive holds %s beside %s; a tag archive holds one directory", first, top)
		}
		if hdr.Typeflag != tar.TypeReg || rest == "" {
			continue
		}
		rel, err := project.NormalizeMemberPath(rest)
		if err != nil {
			return nil, err
		}
		if !keepMember(rel) {
			continue
		}
		if total += hdr.Size; total > limit {
			return nil, fmt.Errorf("the archive extracts to more than %d bytes; the install is bounded", limit)
		}
		content, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
		if err != nil {
			return nil, err
		}
		members[rel] = content
	}
	return members, nil
}

// keepMember reports whether a tree member goes into the install: hidden
// files and directories stay out, except the project files themselves.
func keepMember(rel string) bool {
	for _, seg := range strings.Split(path.Dir(rel), "/") {
		if strings.HasPrefix(seg, ".") && seg != "." {
			return false
		}
	}
	base := path.Base(rel)
	return !strings.HasPrefix(base, ".") || base == project.ProjectFile || base == project.MetaFile
}

// subtree is the members under a subpath, re-rooted at it.
func subtree(members map[string][]byte, sub string) map[string][]byte {
	prefix := strings.Trim(sub, "/") + "/"
	out := map[string][]byte{}
	for name, content := range members {
		if strings.HasPrefix(name, prefix) {
			out[strings.TrimPrefix(name, prefix)] = content
		}
	}
	return out
}

// treeDigest is sha256:<hex> over the members in sorted order, each as its
// path, a NUL, its size, a NUL and its content, so two fetches of one tree
// agree whatever the archive format.
func treeDigest(members map[string][]byte) string {
	h := sha256.New()
	for _, name := range slices.Sorted(maps.Keys(members)) {
		fmt.Fprintf(h, "%s\x00%d\x00", name, len(members[name]))
		h.Write(members[name])
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// digest is sha256:<hex> of an archive's bytes.
func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
