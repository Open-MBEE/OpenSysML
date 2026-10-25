package project

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LibraryDir is where the installer puts the projects a project uses,
// relative to the project's directory.
const LibraryDir = "lib"

// Installer resolves, fetches, installs and locks the usages of a project.
type Installer struct {
	// Dir is the project directory, holding .project.json.
	Dir string
	// Fetcher fetches releases; nil is the default Fetcher.
	Fetcher *Fetcher
	// Log, when set, is told what was installed and what was already there.
	Log io.Writer

	pending []resolution
}

func (in *Installer) fetcher() *Fetcher {
	if in.Fetcher != nil {
		return in.Fetcher
	}
	return &Fetcher{}
}

func (in *Installer) logf(format string, args ...any) {
	if in.Log != nil {
		fmt.Fprintf(in.Log, format+"\n", args...)
	}
}

// Sync installs what .project.json and the lock say: a usage the lock pins
// is reproduced exactly (its version, and its digest when it is fetched); a
// usage the lock lacks is resolved and added. The lock is written when it
// changed. This is -install / -sync.
func (in *Installer) Sync(ctx context.Context) (*Lock, error) {
	prior, err := ReadLock(in.Dir)
	if err != nil {
		return nil, err
	}
	return in.run(ctx, prior, false)
}

// Relock resolves every usage afresh, installs and rewrites the lock. This
// is -lock.
func (in *Installer) Relock(ctx context.Context) (*Lock, error) {
	return in.run(ctx, &Lock{Dir: in.Dir}, true)
}

// Add appends a usage to .project.json, then syncs so it is resolved,
// installed and locked. A usage of the same resource is an error. This is
// -add.
func (in *Installer) Add(ctx context.Context, u Usage) (*Lock, error) {
	o, err := ParseOrigin(u)
	if err != nil {
		return nil, installErr(u.Resource, "resolve", err)
	}
	if o.Kind == OriginStandard {
		return nil, installErr(u.Resource, "resolve", errors.New("a standard library IRI is answered by the bundled library and is never installed"))
	}
	project, err := ReadInterchange(in.Dir)
	if err != nil {
		return nil, err
	}
	for _, have := range project.Usage {
		if have.Resource == u.Resource {
			return nil, installErr(u.Resource, "resolve", fmt.Errorf("%s already uses it (versionConstraint %q); edit the constraint in %s instead", project.Name, have.VersionConstraint, ProjectFile))
		}
	}
	prior, err := ReadLock(in.Dir)
	if err != nil {
		return nil, err
	}
	// The usage is resolved and installed before .project.json records it,
	// so a refused IRI leaves the project as it was.
	trial := *project
	trial.Usage = append(append([]Usage(nil), project.Usage...), u)
	lock, err := in.resolve(ctx, &trial, prior, false)
	if err != nil {
		return nil, err
	}
	if err := in.install(lock); err != nil {
		return nil, err
	}
	if err := appendUsage(filepath.Join(in.Dir, ProjectFile), u); err != nil {
		return nil, installErr(u.Resource, "lock", err)
	}
	if err := lock.Write(); err != nil {
		return nil, installErr(u.Resource, "lock", err)
	}
	return lock, nil
}

func (in *Installer) run(ctx context.Context, prior *Lock, rewrite bool) (*Lock, error) {
	project, err := ReadInterchange(in.Dir)
	if err != nil {
		return nil, err
	}
	lock, err := in.resolve(ctx, project, prior, rewrite)
	if err != nil {
		return nil, err
	}
	if err := in.install(lock); err != nil {
		return nil, err
	}
	before, _ := prior.Marshal()
	after, err := lock.Marshal()
	if err != nil {
		return nil, err
	}
	if rewrite || string(before) != string(after) || !fileExists(filepath.Join(in.Dir, LockFile)) {
		if err := lock.Write(); err != nil {
			return nil, err
		}
	}
	return lock, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// resolution is a resolved entry with the release behind it, installed later.
type resolution struct {
	entry   LockEntry
	release *Release
}

// resolve walks the closure of the project's usages, pinning each to the
// release the prior lock names (unless rewriting) or to the latest release
// its constraint admits, sysand's lock hinting the version when there is no
// prior lock. Standard library IRIs are skipped.
func (in *Installer) resolve(ctx context.Context, project *Interchange, prior *Lock, rewrite bool) (*Lock, error) {
	hints := SysandHints{}
	if !rewrite && len(prior.Entries) == 0 {
		if h, err := ReadSysandHints(in.Dir); err == nil && h != nil {
			hints = h
		}
	}
	lock := &Lock{Dir: in.Dir}
	in.pending = nil
	resolved := map[string]bool{}
	queue := append([]Usage(nil), project.Usage...)
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		if resolved[u.Resource] {
			continue
		}
		resolved[u.Resource] = true
		o, err := ParseOrigin(u)
		if err != nil {
			return nil, installErr(u.Resource, "resolve", err)
		}
		if o.Kind == OriginStandard {
			continue
		}
		if o.Kind == OriginLocal && !filepath.IsAbs(o.Repo) {
			o.Repo = filepath.Join(in.Dir, o.Repo)
		}
		constraint, err := ParseConstraint(o.Constraint)
		if err != nil {
			return nil, installErr(u.Resource, "resolve", err)
		}
		pinned := prior.Entry(u.Resource)
		if rewrite {
			pinned = nil
		}
		want := constraint
		switch {
		case pinned != nil:
			if !constraint.Any() && !constraint.Matches(pinned.Version) {
				return nil, installErr(u.Resource, "resolve", fmt.Errorf("the lock pins version %s, which no longer satisfies %q; run -lock to re-resolve", pinned.Version, constraint))
			}
			want, _ = ParseConstraint(pinned.Version)
		case hints[u.Resource] != "" && (constraint.Any() || constraint.Matches(hints[u.Resource])):
			want, _ = ParseConstraint(hints[u.Resource])
		}
		rel, err := in.fetcher().Fetch(ctx, o, want)
		if err != nil {
			return nil, err
		}
		if pinned != nil && pinned.Digest != "" && rel.Digest != "" && pinned.Digest != rel.Digest {
			return nil, installErr(u.Resource, "verify", fmt.Errorf("the lock records %s for version %s but the fetch is %s; the release changed under its tag, or the lock is stale (run -lock to re-resolve)", pinned.Digest, pinned.Version, rel.Digest))
		}
		entry := LockEntry{
			Resource: u.Resource, IRI: o.IRI, Kind: o.Kind.String(), Constraint: o.Constraint,
			Publisher: rel.Project.Publisher, Name: rel.Project.Name, Version: rel.Version,
			Source: rel.SourceURL, Tag: rel.Tag, Commit: rel.Commit, Digest: rel.Digest,
		}
		if rel.Installable() {
			name := InstallName(rel.Project)
			if o.Kind == OriginArchive {
				name += ArchiveExt
			}
			entry.Path = LibraryDir + "/" + name
		} else {
			entry.Path = relPath(in.Dir, rel.Path)
		}
		for _, dep := range rel.Project.Usage {
			if !IsStandardLibraryResource(dep.Resource) {
				entry.Usage = append(entry.Usage, dep.Resource)
				queue = append(queue, dep)
			}
		}
		lock.Entries = append(lock.Entries, entry)
		in.pending = append(in.pending, resolution{entry: entry, release: rel})
	}
	lock.sort()
	return lock, nil
}

// relPath is path relative to dir with forward slashes when it lies under
// or beside it, else the absolute path.
func relPath(dir, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	base, err := filepath.Abs(dir)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// install writes every resolved release that is not installed yet under
// LibraryDir; one already there at the locked version is kept.
func (in *Installer) install(lock *Lock) error {
	for _, r := range in.pending {
		e := r.entry
		if !r.release.Installable() {
			in.logf("%s %s: read in place at %s", e.Name, e.Version, e.Path)
			continue
		}
		target := lock.AbsPath(&e)
		if have, err := installedVersion(target); err == nil {
			if have != e.Version {
				return installErr(e.Resource, "install", fmt.Errorf("%s holds version %s, not the locked %s; remove it to reinstall", e.Path, have, e.Version))
			}
			in.logf("%s %s: already installed at %s", e.Name, e.Version, e.Path)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return installErr(e.Resource, "install", err)
		}
		if IsArchive(target) {
			if err := writeNew(target, r.release.Archive); err != nil {
				return installErr(e.Resource, "install", err)
			}
		} else {
			a, err := ParseArchive(e.Source, r.release.Archive)
			if err != nil {
				return installErr(e.Resource, "verify", err)
			}
			if err := Unbundle(a, target); err != nil {
				_ = os.RemoveAll(target)
				return installErr(e.Resource, "install", err)
			}
		}
		in.logf("%s %s: installed at %s", e.Name, e.Version, e.Path)
	}
	in.pending = nil
	return nil
}

// installedVersion is the version of the project installed at path, which
// is a project directory or an archive.
func installedVersion(path string) (string, error) {
	if IsArchive(path) {
		a, err := OpenArchive(path)
		if err != nil {
			return "", err
		}
		return a.Project.Version, nil
	}
	p, err := ReadInterchange(path)
	if err != nil {
		return "", err
	}
	return p.Version, nil
}

// appendUsage adds a usage to a .project.json, keeping the file's other
// fields; the file is rewritten with sorted keys and a trailing newline, as
// -project-init writes it.
func appendUsage(file string, u Usage) error {
	content, err := os.ReadFile(file) // #nosec G304 -- the project file being edited.
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := decodeObject(content, &raw); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	usages, _ := raw["usage"].([]any)
	entry := map[string]any{"resource": u.Resource}
	if strings.TrimSpace(u.VersionConstraint) != "" {
		entry["versionConstraint"] = u.VersionConstraint
	}
	raw["usage"] = append(usages, entry)
	data, err := marshalSorted(raw)
	if err != nil {
		return err
	}
	return writeWhole(file, data)
}
