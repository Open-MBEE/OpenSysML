package project

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// archiveEpoch is the modification time every member of a written archive
// carries — the earliest ZIP can hold, as the OMG's archives do — so the bytes
// written depend on the content alone.
var archiveEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// Metamodel IRIs the metadata file names (§10.3, Table 13), the KerML one
// being what an archive without the field is read as.
const (
	MetamodelKerML = "https://www.omg.org/spec/KerML/20250201"
	MetamodelSysML = "https://www.omg.org/spec/SysML/20250201"
)

// Bundle archives the project directory dir as a project interchange archive
// (KerML 1.0 §10.3) and returns its bytes with the project they hold. Every
// regular file under dir is a member except those in hidden directories and
// hidden files other than the two manifests; a .meta.json the directory has is
// kept and checked against the files, one it lacks is written from them. The
// archive is deterministic: members sorted, each with the same fixed time.
func Bundle(dir string) ([]byte, *Interchange, error) {
	project, err := ReadInterchange(dir)
	if err != nil {
		return nil, nil, err
	}
	members, err := bundleMembers(dir)
	if err != nil {
		return nil, nil, err
	}
	data, err := archiveMembers(dir, members)
	if err != nil {
		return nil, nil, err
	}
	return data, project, nil
}

// archiveMembers writes the members as an archive, generating the .meta.json
// they lack, and reads it back as a consumer would, so a .meta.json they
// carry is checked against the files it describes; name is the tree's for
// the diagnostics.
func archiveMembers(name string, members map[string][]byte) ([]byte, error) {
	if _, ok := members[MetaFile]; !ok {
		meta, err := generateMeta(members)
		if err != nil {
			return nil, err
		}
		members[MetaFile] = meta
	}
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range names {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: archiveEpoch}
		h.SetMode(0o644)
		f, err := w.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(members[name]); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if _, err := ParseArchive(name, buf.Bytes()); err != nil {
		var ae *ArchiveError
		if errors.As(err, &ae) {
			err = ae.Err
		}
		return nil, err
	}
	return buf.Bytes(), nil
}

// bundleMembers reads the files of dir that an archive of it holds, keyed by
// their slash-separated path under dir.
func bundleMembers(dir string) (map[string][]byte, error) {
	members := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hidden := rel != "." && strings.HasPrefix(d.Name(), ".")
		if d.IsDir() {
			if hidden {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || (hidden && !(rel == ProjectFile || rel == MetaFile)) {
			return nil
		}
		content, err := os.ReadFile(path) // #nosec G304 -- a file under the directory being archived.
		if err != nil {
			return err
		}
		members[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return members, nil
}

// generateMeta writes the .meta.json of a project directory that has none:
// the index of the top-level namespaces each model file declares, a SHA256
// checksum of every member, and the metamodel of the files' notation.
func generateMeta(members map[string][]byte) ([]byte, error) {
	meta := Meta{Index: map[string]string{}, Checksum: map[string][]Checksum{}, Metamodel: MetamodelKerML}
	for name, content := range members {
		if strings.HasSuffix(name, ".sysml") {
			meta.Metamodel = MetamodelSysML
		}
		if IsModelFile(name) {
			root := parser.New(source.New(name, content)).ParseFile()
			for _, declared := range declaredNames(root.Members) {
				if other, dup := meta.Index[declared]; dup && other != name {
					return nil, fmt.Errorf("%s and %s both declare %s; a project's namespace is flat (KerML 1.0 §10.3)", other, name, declared)
				}
				meta.Index[declared] = name
			}
		}
		sum := sha256.Sum256(content)
		meta.Checksum[name] = []Checksum{{Value: hex.EncodeToString(sum[:]), Algorithm: "SHA256"}}
	}
	meta.Created = createdTime().UTC().Format(time.RFC3339)
	return marshalMeta(meta)
}

// createdTime is the creation time a generated .meta.json records: the
// SOURCE_DATE_EPOCH of a reproducible build when set, else the archive epoch,
// since the clock would make two runs of the same directory differ.
func createdTime() time.Time {
	if seconds, err := strconv.ParseInt(os.Getenv("SOURCE_DATE_EPOCH"), 10, 64); err == nil && seconds >= 0 {
		return time.Unix(seconds, 0)
	}
	return archiveEpoch
}

// marshalMeta writes the metadata as the schema spells it, a checksum
// recorded once as one object.
func marshalMeta(meta Meta) ([]byte, error) {
	out := struct {
		Index           map[string]string `json:"index"`
		Created         string            `json:"created"`
		Metamodel       string            `json:"metamodel,omitempty"`
		IncludesDerived *bool             `json:"includesDerived,omitempty"`
		IncludesImplied *bool             `json:"includesImplied,omitempty"`
		Checksum        map[string]any    `json:"checksum,omitempty"`
	}{Index: meta.Index, Created: meta.Created, Metamodel: meta.Metamodel, IncludesDerived: meta.IncludesDerived, IncludesImplied: meta.IncludesImplied}
	if len(meta.Checksum) > 0 {
		out.Checksum = map[string]any{}
		for name, sums := range meta.Checksum {
			if len(sums) == 1 {
				out.Checksum[name] = sums[0]
			} else {
				out.Checksum[name] = sums
			}
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Unbundle materializes the archive as a project directory at dir, which must
// not exist or be empty, so an archive never overwrites a file that was there.
// Members are written under their archive paths, which OpenArchive has already
// confined to the directory.
func Unbundle(a *Archive, dir string) error {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s exists and is not empty; unbundle into a new or empty directory", dir)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	for _, name := range a.Names() {
		content, err := a.Read(name)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := confinedTo(dir, filepath.Dir(path)); err != nil {
			return err
		}
		if err := writeNew(path, content); err != nil {
			return err
		}
	}
	return nil
}

// confinedTo fails unless parent, links resolved, still lies under dir, so a
// link planted among the directories being written cannot lead outside.
func confinedTo(dir, parent string) error {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s resolves outside %s", parent, dir)
	}
	return nil
}

// writeNew creates path, refusing one that exists in any form, a link included.
func writeNew(path string, content []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- a member path validated against the archive's directory.
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
