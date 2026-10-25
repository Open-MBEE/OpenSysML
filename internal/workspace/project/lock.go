package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LockFile is the file beside .project.json that pins the resolved closure of
// a project's usages; -install reproduces it, -lock rewrites it.
const LockFile = ".project.lock.json"

// SysandLockFile is sysand's lock, read as a hint for the versions to pin
// when a project has no LockFile; it is never written.
const SysandLockFile = "sysand-lock.toml"

// lockVersion is the schema version LockFile carries.
const lockVersion = 1

// Lock is the resolved closure of a project's usages: one entry per usage,
// the project's own and those of the projects it installed.
type Lock struct {
	// Dir is the project directory the lock lies beside .project.json in.
	Dir string
	// Entries are sorted by Resource, each resource once.
	Entries []LockEntry
}

// LockEntry pins one usage to the release that satisfied it.
type LockEntry struct {
	// Resource is the usage's resource IRI as .project.json spells it.
	Resource string `json:"resource"`
	// IRI is the resource read as an origin, its version taken out.
	IRI string `json:"iri"`
	// Kind is the origin kind: host, git, archive or local.
	Kind string `json:"kind"`
	// Constraint is the versionConstraint the release satisfied.
	Constraint string `json:"versionConstraint,omitempty"`
	// Publisher, Name and Version are the release's .project.json says.
	Publisher string `json:"publisher,omitempty"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	// Source is the URL (or path) the release was fetched from.
	Source string `json:"source"`
	// Tag and Commit name the release in its repository, when it has one.
	Tag    string `json:"tag,omitempty"`
	Commit string `json:"commit,omitempty"`
	// Digest is sha256:<hex> of the fetched tree or archive, when fetched.
	Digest string `json:"digest,omitempty"`
	// Path is where the project is read from, relative to Dir with forward
	// slashes: under lib/ when installed, the usage's own path when local.
	Path string `json:"path"`
	// Usage lists the resources of the release's own usages; each has an
	// entry of its own, so the lock holds the closure.
	Usage []string `json:"usage,omitempty"`
}

// ReadLock reads the lock of a project directory; a missing file is a lock
// with no entries and no error.
func ReadLock(dir string) (*Lock, error) {
	lock := &Lock{Dir: dir}
	content, err := os.ReadFile(filepath.Join(dir, LockFile)) // #nosec G304 -- the lock beside the project file.
	if errors.Is(err, os.ErrNotExist) {
		return lock, nil
	}
	if err != nil {
		return nil, err
	}
	var raw struct {
		LockVersion int         `json:"lockVersion"`
		Projects    []LockEntry `json:"projects"`
	}
	if err := decodeObject(content, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, LockFile), err)
	}
	if raw.LockVersion != lockVersion {
		return nil, fmt.Errorf("%s: lockVersion %d is not the %d this build writes", filepath.Join(dir, LockFile), raw.LockVersion, lockVersion)
	}
	seen := map[string]bool{}
	for _, e := range raw.Projects {
		if e.Resource == "" || e.Path == "" || e.Version == "" || e.Name == "" {
			return nil, fmt.Errorf("%s: an entry lacks its resource, name, version or path", filepath.Join(dir, LockFile))
		}
		if seen[e.Resource] {
			return nil, fmt.Errorf("%s: %s is pinned twice", filepath.Join(dir, LockFile), e.Resource)
		}
		seen[e.Resource] = true
	}
	lock.Entries = raw.Projects
	lock.sort()
	return lock, nil
}

// Entry returns the entry pinning a resource, nil when none does.
func (l *Lock) Entry(resource string) *LockEntry {
	if l == nil {
		return nil
	}
	for i := range l.Entries {
		if l.Entries[i].Resource == resource {
			return &l.Entries[i]
		}
	}
	return nil
}

// AbsPath is where an entry's project is read from, under the lock's directory.
func (l *Lock) AbsPath(e *LockEntry) string {
	p := filepath.FromSlash(e.Path)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(l.Dir, p)
}

func (l *Lock) sort() {
	sort.Slice(l.Entries, func(i, j int) bool { return l.Entries[i].Resource < l.Entries[j].Resource })
	for i := range l.Entries {
		sort.Strings(l.Entries[i].Usage)
	}
}

// Marshal writes the lock as deterministic JSON: object keys sorted, entries
// sorted by resource, two-space indent, a trailing newline.
func (l *Lock) Marshal() ([]byte, error) {
	l.sort()
	projects := make([]map[string]any, 0, len(l.Entries))
	for _, e := range l.Entries {
		raw, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		projects = append(projects, m)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]any{"lockVersion": lockVersion, "projects": projects}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write writes the lock beside the project file, replacing the one there.
func (l *Lock) Write() error {
	data, err := l.Marshal()
	if err != nil {
		return err
	}
	return writeWhole(filepath.Join(l.Dir, LockFile), data)
}

// writeWhole writes a file in place, creating or truncating it.
func writeWhole(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

// SysandHints are the versions sysand's lock pins, keyed by each identifier
// it lists for a project; read when a project has no LockFile so the first
// resolution lands on the versions a sysand user already locked.
type SysandHints map[string]string

// ReadSysandHints reads SysandLockFile in dir; absent, it hints nothing.
// Only what the hint needs is read: each [[project]]'s version and identifiers.
func ReadSysandHints(dir string) (SysandHints, error) {
	content, err := os.ReadFile(filepath.Join(dir, SysandLockFile)) // #nosec G304 -- the sysand lock beside the project file.
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	hints := SysandHints{}
	var version string
	var identifiers []string
	flush := func() {
		for _, id := range identifiers {
			if version != "" {
				hints[id] = version
			}
		}
		version, identifiers = "", nil
	}
	lines := strings.Split(string(content), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == "[[project]]":
			flush()
		case strings.HasPrefix(line, "["):
			flush()
		default:
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			for strings.Count(value, "[") > strings.Count(value, "]") && i+1 < len(lines) {
				i++
				value += " " + strings.TrimSpace(lines[i])
			}
			switch strings.TrimSpace(key) {
			case "version":
				version = tomlString(value)
			case "identifiers":
				identifiers = tomlStrings(value)
			}
		}
	}
	flush()
	return hints, nil
}

// tomlString reads a basic or literal TOML string, "" for anything else.
func tomlString(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return ""
}

// tomlStrings reads a TOML array of strings.
func tomlStrings(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	var out []string
	for _, item := range strings.Split(value[1:len(value)-1], ",") {
		if s := tomlString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}
