package install

import (
	"context"
	"errors"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
	"io"
	"os"
	"path/filepath"
)

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
func (in *Installer) Sync(ctx context.Context) (*project.Lock, error) {
	prior, err := project.ReadLock(in.Dir)
	if err != nil {
		return nil, err
	}
	return in.run(ctx, prior, false)
}

// Relock resolves every usage afresh, installs and rewrites the lock. This
// is -lock.
func (in *Installer) Relock(ctx context.Context) (*project.Lock, error) {
	return in.run(ctx, &project.Lock{Dir: in.Dir}, true)
}

// Add appends a usage to .project.json, then syncs so it is resolved,
// installed and locked. A usage of the same resource is an error. This is
// -add.
func (in *Installer) Add(ctx context.Context, u project.Usage) (*project.Lock, error) {
	o, err := project.ParseOrigin(u)
	if err != nil {
		return nil, installErr(u.Resource, "resolve", err)
	}
	if o.Kind == project.OriginStandard {
		return nil, installErr(u.Resource, "resolve", errors.New("a standard library IRI is answered by the bundled library and is never installed"))
	}
	own, err := project.ReadInterchange(in.Dir)
	if err != nil {
		return nil, err
	}
	for _, have := range own.Usage {
		if have.Resource == u.Resource {
			return nil, installErr(u.Resource, "resolve", fmt.Errorf("%s already uses it (versionConstraint %q); edit the constraint in %s instead", own.Name, have.VersionConstraint, project.ProjectFile))
		}
	}
	prior, err := project.ReadLock(in.Dir)
	if err != nil {
		return nil, err
	}
	// The usage is resolved and installed before .project.json records it,
	// so a refused IRI leaves the project as it was.
	trial := *own
	trial.Usage = append(append([]project.Usage(nil), own.Usage...), u)
	lock, err := in.resolve(ctx, &trial, prior, false)
	if err != nil {
		return nil, err
	}
	if err := in.install(lock); err != nil {
		return nil, err
	}
	if err := project.AppendUsage(in.Dir, u); err != nil {
		return nil, installErr(u.Resource, "lock", err)
	}
	if err := lock.Write(); err != nil {
		return nil, installErr(u.Resource, "lock", err)
	}
	return lock, nil
}

func (in *Installer) run(ctx context.Context, prior *project.Lock, rewrite bool) (*project.Lock, error) {
	own, err := project.ReadInterchange(in.Dir)
	if err != nil {
		return nil, err
	}
	lock, err := in.resolve(ctx, own, prior, rewrite)
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
	if rewrite || string(before) != string(after) || !exists(filepath.Join(in.Dir, project.LockFile)) {
		if err := lock.Write(); err != nil {
			return nil, err
		}
	}
	return lock, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// resolution is a resolved entry with the release behind it, installed later.
type resolution struct {
	entry   project.LockEntry
	release *Release
}

// resolve walks the closure of the project's usages, pinning each to the
// release the prior lock names (unless rewriting) or to the latest release
// its constraint admits, sysand's lock hinting the version when there is no
// prior lock. Standard library IRIs are skipped.
func (in *Installer) resolve(ctx context.Context, own *project.Interchange, prior *project.Lock, rewrite bool) (*project.Lock, error) {
	hints := project.SysandHints{}
	if !rewrite && len(prior.Entries) == 0 {
		if h, err := project.ReadSysandHints(in.Dir); err == nil && h != nil {
			hints = h
		}
	}
	lock := &project.Lock{Dir: in.Dir}
	in.pending = nil
	resolved := map[string]bool{}
	queue := append([]project.Usage(nil), own.Usage...)
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		if resolved[u.Resource] {
			continue
		}
		resolved[u.Resource] = true
		o, err := project.ParseOrigin(u)
		if err != nil {
			return nil, installErr(u.Resource, "resolve", err)
		}
		if o.Kind == project.OriginStandard {
			continue
		}
		if o.Kind == project.OriginLocal && !filepath.IsAbs(o.Repo) {
			o.Repo = filepath.Join(in.Dir, o.Repo)
		}
		constraint, err := project.ParseConstraint(o.Constraint)
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
			want, _ = project.ParseConstraint(pinned.Version)
		case hints[u.Resource] != "" && (constraint.Any() || constraint.Matches(hints[u.Resource])):
			want, _ = project.ParseConstraint(hints[u.Resource])
		}
		rel, err := in.fetcher().Fetch(ctx, o, want)
		if err != nil {
			return nil, err
		}
		if pinned != nil && pinned.Digest != "" && rel.Digest != "" && pinned.Digest != rel.Digest {
			return nil, installErr(u.Resource, "verify", fmt.Errorf("the lock records %s for version %s but the fetch is %s; the release changed under its tag, or the lock is stale (run -lock to re-resolve)", pinned.Digest, pinned.Version, rel.Digest))
		}
		entry := project.LockEntry{
			Resource: u.Resource, IRI: o.IRI, Kind: o.Kind.String(), Constraint: o.Constraint,
			Publisher: rel.Project.Publisher, Name: rel.Project.Name, Version: rel.Version,
			Source: rel.SourceURL, Tag: rel.Tag, Commit: rel.Commit, Digest: rel.Digest,
		}
		if rel.Installable() {
			name := project.InstallName(rel.Project)
			if o.Kind == project.OriginArchive {
				name += project.ArchiveExt
			}
			entry.Path = project.LibraryDir + "/" + name
		} else {
			entry.Path = relPath(in.Dir, rel.Path)
		}
		for _, dep := range rel.Project.Usage {
			if !project.IsStandardLibraryResource(dep.Resource) {
				entry.Usage = append(entry.Usage, dep.Resource)
				queue = append(queue, dep)
			}
		}
		lock.Entries = append(lock.Entries, entry)
		in.pending = append(in.pending, resolution{entry: entry, release: rel})
	}
	lock.Sort()
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
// project.LibraryDir; one already there at the locked version is kept.
func (in *Installer) install(lock *project.Lock) error {
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
		if project.IsArchive(target) {
			if err := project.WriteNew(target, r.release.Archive); err != nil {
				return installErr(e.Resource, "install", err)
			}
		} else {
			a, err := project.ParseArchive(e.Source, r.release.Archive)
			if err != nil {
				return installErr(e.Resource, "verify", err)
			}
			if err := project.Unbundle(a, target); err != nil {
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
	if project.IsArchive(path) {
		a, err := project.OpenArchive(path)
		if err != nil {
			return "", err
		}
		return a.Project.Version, nil
	}
	p, err := project.ReadInterchange(path)
	if err != nil {
		return "", err
	}
	return p.Version, nil
}
