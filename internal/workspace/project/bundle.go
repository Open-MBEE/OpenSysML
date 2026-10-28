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
// (KerML 1.0 §10.3) and returns its bytes with the project they hold. A
// .meta.json the directory has is kept and decides the members: the two
// manifests and exactly the files it records (checksum keys and index
// targets), each of which must exist. Without one, the members are what §10.3
// says an archive holds — the model files under dir, outside hidden entries,
// library directories and nested projects — and a .meta.json written from
// them. The archive is deterministic: members sorted, each with the same
// fixed time.
func Bundle(dir string) ([]byte, *Interchange, error) {
	project, err := ReadInterchange(dir)
	if err != nil {
		return nil, nil, err
	}
	members, err := bundleMembers(dir)
	if err != nil {
		return nil, nil, err
	}
	if _, ok := members[MetaFile]; !ok {
		meta, err := generateMeta(members)
		if err != nil {
			return nil, nil, err
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
			return nil, nil, err
		}
		if _, err := f.Write(members[name]); err != nil {
			return nil, nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, nil, err
	}
	// The archive is read back as a consumer would, so a .meta.json the
	// directory kept is checked against the files it describes.
	if _, err := ParseArchive(dir, buf.Bytes()); err != nil {
		var ae *ArchiveError
		if errors.As(err, &ae) {
			err = ae.Err
		}
		return nil, nil, err
	}
	return buf.Bytes(), project, nil
}

// bundleMembers reads the files of dir that an archive of it holds, keyed by
// their slash-separated path under dir: the files its .meta.json records, or
// its model files when it has none.
func bundleMembers(dir string) (map[string][]byte, error) {
	meta, err := os.ReadFile(filepath.Join(dir, MetaFile)) // #nosec G304 -- the manifest of the directory being archived.
	if errors.Is(err, fs.ErrNotExist) {
		return modelMembers(dir)
	}
	if err != nil {
		return nil, err
	}
	return recordedMembers(dir, meta)
}

// recordedMembers reads the files the directory's .meta.json content records, with the two manifests;
// a recorded file missing, not regular, or reached through a link leading outside is an error naming it.
func recordedMembers(dir string, content []byte) (map[string][]byte, error) {
	meta, err := ParseMeta(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", MetaFile, err)
	}
	recorded := append([]string{ProjectFile}, sortedKeys(meta.Checksum)...)
	for _, name := range sortedKeys(meta.Index) {
		recorded = append(recorded, meta.Index[name])
	}
	members := map[string][]byte{MetaFile: content}
	for _, name := range recorded {
		member, err := normalizeMemberPath(name)
		if err != nil {
			return nil, fmt.Errorf("%s records %q: %w", MetaFile, name, err)
		}
		if _, done := members[member]; done {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(member))
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s records %s, which is not in the directory", MetaFile, member)
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s records %s, which is not a regular file", MetaFile, member)
		}
		if err := confinedTo(dir, filepath.Dir(path)); err != nil {
			return nil, fmt.Errorf("%s records %s, which %w", MetaFile, member, err)
		}
		content, err := os.ReadFile(path) // #nosec G304 -- a file the manifest of the directory being archived records, confined to it.
		if err != nil {
			return nil, err
		}
		members[member] = content
	}
	return members, nil
}

// modelMembers reads the project file and model files of a directory without a .meta.json (what §10.3
// says an archive holds), leaving out hidden entries, library directories and nested projects' trees.
func modelMembers(dir string) (map[string][]byte, error) {
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
			// A project nested in the directory is a project of its own,
			// bundled on its own.
			if hidden || IsLibraryDir(filepath.Dir(path), d.Name()) || (rel != "." && IsProjectDir(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || (rel != ProjectFile && (hidden || !IsModelFile(rel))) {
			return nil
		}
		content, err := os.ReadFile(path) // #nosec G304 G122 -- a model file under the directory being archived.
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
	meta := Meta{Index: map[string]string{}, Checksum: map[string]Checksums{}, Metamodel: MetamodelKerML}
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
		meta.Checksum[name] = Checksums{{Value: hex.EncodeToString(sum[:]), Algorithm: "SHA256"}}
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
	data, err := json.MarshalIndent(meta, "", "  ")
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
