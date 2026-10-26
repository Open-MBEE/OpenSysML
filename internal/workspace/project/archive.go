package project

import (
	"archive/zip"
	"bytes"
	"crypto/md5"  // #nosec G501 -- an algorithm the interchange schema allows for a checksum, never chosen here.
	"crypto/sha1" // #nosec G505 -- as for md5.
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/adler32"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ArchiveExt is the extension of a KerML project interchange file: a project
// directory archived with ZIP (KerML 1.0 §10.3).
const ArchiveExt = ".kpar"

// MetaFile is the metadata file an archive carries beside its interchange file.
const MetaFile = ".meta.json"

// maxArchiveSize bounds the archive read into memory, so a file misnamed .kpar
// cannot exhaust it.
const maxArchiveSize = 512 << 20

// Meta is the content of an archive's .meta.json (KerML 1.0 §10.3, Table 13).
type Meta struct {
	// Index maps the names and short names of the top-level elements of the
	// project's root namespaces to the member file declaring them.
	Index map[string]string `json:"index"`
	// Created is when the archive was made, in ISO 8601 format.
	Created string `json:"created"`
	// Metamodel is the IRI of the language the members are written in; KerML
	// when absent.
	Metamodel string `json:"metamodel,omitempty"`
	// IncludesDerived and IncludesImplied qualify XMI and JSON members.
	IncludesDerived *bool `json:"includesDerived,omitempty"`
	IncludesImplied *bool `json:"includesImplied,omitempty"`
	// Checksum maps member paths to the checksums recorded for them.
	Checksum map[string]Checksums `json:"checksum,omitempty"`
}

// Checksum is one recorded digest of a member file.
type Checksum struct {
	Value     string `json:"value"`
	Algorithm string `json:"algorithm"`
}

// Unrecorded reports a checksum that records no digest, by the algorithm NONE
// (sysand's placeholder for a member it did not digest) or an empty value.
func (c Checksum) Unrecorded() bool {
	return strings.EqualFold(c.Algorithm, "NONE") || c.Value == ""
}

// Checksums are the checksums recorded for one member: one object per path in
// the normative schema (KerML-Model-Interchange.json), a list in Table 13's text.
type Checksums []Checksum

// UnmarshalJSON reads one checksum object or a non-empty list of them.
func (c *Checksums) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	switch {
	case bytes.HasPrefix(data, []byte("[")):
		var list []Checksum
		if err := json.Unmarshal(data, &list); err != nil {
			return err
		}
		if len(list) == 0 {
			return errors.New("is an empty list")
		}
		*c = list
	case bytes.HasPrefix(data, []byte("{")):
		var one Checksum
		if err := json.Unmarshal(data, &one); err != nil {
			return err
		}
		*c = Checksums{one}
	default:
		return errors.New("is neither a checksum object nor a list of them")
	}
	return nil
}

// MarshalJSON writes one checksum as the schema's object, several as a list.
func (c Checksums) MarshalJSON() ([]byte, error) {
	if len(c) == 1 {
		return json.Marshal(c[0])
	}
	return json.Marshal([]Checksum(c))
}

// Archive is a project interchange file read into memory: its project
// information, metadata and members, none of which touch the file system.
type Archive struct {
	// Path is the archive as it was named.
	Path string
	// Project is the interchange information of the archived project; its Dir
	// is the archive's path.
	Project *Interchange
	// Meta is the archive's metadata.
	Meta Meta
	// Notes are findings about the archive that did not prevent reading it.
	Notes []string
	// members maps each member's archive path to its bytes.
	members map[string][]byte
	// names lists the member paths, sorted.
	names []string
}

// IsArchive reports whether path is named as a project interchange file.
func IsArchive(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ArchiveExt)
}

// IsArchiveFile reports whether path names an existing archive: a regular file
// with the archive extension.
func IsArchiveFile(path string) bool {
	if !IsArchive(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// ArchiveError is an archive that could not be read as a project interchange
// file, reported under the archive's path.
type ArchiveError struct {
	Path string
	Err  error
}

func (e *ArchiveError) Error() string { return fmt.Sprintf("%s: %v", e.Path, e.Err) }

func (e *ArchiveError) Unwrap() error { return e.Err }

// OpenArchive reads the archive at path into memory and validates it. A file
// that is not a ZIP archive, or one without the interchange and metadata files
// the specification requires at its top level, is an *ArchiveError.
func OpenArchive(path string) (*Archive, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, &ArchiveError{Path: path, Err: err}
	}
	if info.Size() > maxArchiveSize {
		return nil, &ArchiveError{Path: path, Err: fmt.Errorf("archive is %d bytes; at most %d are read", info.Size(), maxArchiveSize)}
	}
	// #nosec G304 -- the archive is one the user named or found under a library root.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &ArchiveError{Path: path, Err: err}
	}
	return ParseArchive(path, data)
}

// ParseArchive reads an archive from its bytes, reported under path.
func ParseArchive(path string, data []byte) (*Archive, error) {
	a, err := parseArchive(path, data)
	if err != nil {
		return nil, &ArchiveError{Path: path, Err: err}
	}
	return a, nil
}

func parseArchive(archivePath string, data []byte) (*Archive, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a ZIP archive: %w", err)
	}
	a := &Archive{Path: archivePath, members: map[string][]byte{}}
	var total uint64
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		name, err := normalizeMemberPath(f.Name)
		if err != nil {
			return nil, err
		}
		if _, dup := a.members[name]; dup {
			return nil, fmt.Errorf("member %q appears twice", name)
		}
		if f.UncompressedSize64 > maxArchiveSize || total+f.UncompressedSize64 > maxArchiveSize {
			return nil, fmt.Errorf("members unpack to more than %d bytes", maxArchiveSize)
		}
		total += f.UncompressedSize64
		content, err := readMember(f)
		if err != nil {
			return nil, fmt.Errorf("member %q: %w", name, err)
		}
		a.members[name] = content
		a.names = append(a.names, name)
	}
	sort.Strings(a.names)
	if err := a.readProject(); err != nil {
		return nil, err
	}
	if err := a.readMeta(); err != nil {
		return nil, err
	}
	return a, nil
}

// readMember decompresses one member, bounding what is read to the size its
// header declares so a lying header cannot exhaust memory.
func readMember(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	if f.UncompressedSize64 > maxArchiveSize {
		return nil, fmt.Errorf("declares %d bytes; at most %d are read", f.UncompressedSize64, maxArchiveSize)
	}
	content, err := io.ReadAll(io.LimitReader(rc, int64(maxArchiveSize)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(content)) != f.UncompressedSize64 {
		return nil, fmt.Errorf("declares %d bytes but holds %d", f.UncompressedSize64, len(content))
	}
	return content, nil
}

// normalizeMemberPath checks a member path is relative, slash-separated and
// free of . and .. segments (§10.3, Table 13 note 2), so it can be a key and,
// when materialized, stays under the directory written.
func normalizeMemberPath(name string) (string, error) {
	switch {
	case name == "":
		return "", errors.New("a member has an empty path")
	case strings.ContainsRune(name, '\\'):
		return "", fmt.Errorf("member %q: path segments are separated by /, not \\", name)
	case strings.HasPrefix(name, "/"):
		return "", fmt.Errorf("member %q: path is absolute", name)
	case strings.ContainsRune(name, 0):
		return "", fmt.Errorf("member %q: path holds a NUL byte", name)
	}
	if vol := filepath.VolumeName(name); vol != "" || strings.ContainsRune(name, ':') {
		return "", fmt.Errorf("member %q: path holds a drive or scheme prefix", name)
	}
	for _, seg := range strings.Split(name, "/") {
		switch seg {
		case "":
			return "", fmt.Errorf("member %q: path has an empty segment", name)
		case ".", "..":
			return "", fmt.Errorf("member %q: path has a %s segment", name, seg)
		}
	}
	return name, nil
}

// readProject reads the archive's .project.json, which the specification puts
// at the top level, exactly once.
func (a *Archive) readProject() error {
	content, ok := a.members[ProjectFile]
	if !ok {
		return a.missingTopLevel(ProjectFile)
	}
	project, err := parseInterchange(a.Path, a.Path+"/"+ProjectFile, content)
	if err != nil {
		return err
	}
	a.Project = project
	return nil
}

// missingTopLevel reports a required file absent from the top level, naming a
// copy nested in a directory, as an archive made of a folder's parent holds.
func (a *Archive) missingTopLevel(file string) error {
	for _, name := range a.names {
		if path.Base(name) == file {
			return fmt.Errorf("no %s at the top level of the archive; %s is nested in a directory, so the archive holds a folder rather than a project", file, name)
		}
	}
	return fmt.Errorf("no %s at the top level of the archive", file)
}

// readMeta reads and checks .meta.json: its index and checksums must name
// members, and every checksum this build can compute must match.
func (a *Archive) readMeta() error {
	content, ok := a.members[MetaFile]
	if !ok {
		return a.missingTopLevel(MetaFile)
	}
	meta, err := ParseMeta(content)
	if err != nil {
		return fmt.Errorf("%s: %w", MetaFile, err)
	}
	for _, name := range sortedKeys(meta.Index) {
		member := meta.Index[name]
		if _, ok := a.members[member]; !ok {
			return fmt.Errorf("%s: index entry %q names %q, which is not a member of the archive", MetaFile, name, member)
		}
	}
	for _, member := range sortedKeys(meta.Checksum) {
		content, ok := a.members[member]
		if !ok {
			return fmt.Errorf("%s: a checksum is recorded for %q, which is not a member of the archive", MetaFile, member)
		}
		for _, sum := range meta.Checksum[member] {
			if err := a.verify(member, content, sum); err != nil {
				return fmt.Errorf("%s: %w", MetaFile, err)
			}
		}
	}
	a.Meta = meta
	return nil
}

// verify checks one recorded checksum. An algorithm the schema allows but
// this build cannot compute is noted rather than refused; a checksum that
// records no digest (algorithm NONE, or an empty value) verifies nothing.
func (a *Archive) verify(member string, content []byte, sum Checksum) error {
	if sum.Algorithm == "" {
		return fmt.Errorf("checksum of %q lacks an algorithm", member)
	}
	if strings.EqualFold(sum.Algorithm, "NONE") {
		return nil
	}
	h, known := newHash(sum.Algorithm)
	if !known {
		if !schemaAlgorithm(sum.Algorithm) {
			return fmt.Errorf("checksum of %q uses %q, which is not an algorithm the interchange schema allows", member, sum.Algorithm)
		}
		a.Notes = append(a.Notes, fmt.Sprintf("the %s checksum of %s was not verified: this build does not compute %s", sum.Algorithm, member, sum.Algorithm))
		return nil
	}
	if sum.Unrecorded() {
		return nil
	}
	h.Write(content)
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, sum.Value) {
		return fmt.Errorf("checksum of %q does not match: %s is %s, recorded as %s", member, sum.Algorithm, got, sum.Value)
	}
	return nil
}

// newHash returns the hash for an algorithm name of Table 13, note 6.
func newHash(algorithm string) (hash.Hash, bool) {
	switch strings.ToUpper(algorithm) {
	case "SHA1", "SHA-1":
		return sha1.New(), true // #nosec G401 -- verifying a digest another tool recorded.
	case "SHA224", "SHA-224":
		return sha256.New224(), true
	case "SHA256", "SHA-256":
		return sha256.New(), true
	case "SHA384", "SHA-384":
		return sha512.New384(), true
	case "SHA512", "SHA-512":
		return sha512.New(), true
	case "SHA3-256":
		return sha3.New256(), true
	case "SHA3-384":
		return sha3.New384(), true
	case "SHA3-512":
		return sha3.New512(), true
	case "MD5":
		return md5.New(), true // #nosec G401 -- as for SHA1.
	case "ADLER32":
		return adler32.New(), true
	}
	return nil, false
}

// schemaAlgorithm reports whether the name is one the interchange schema
// allows, including those this build cannot compute.
func schemaAlgorithm(algorithm string) bool {
	switch strings.ToUpper(algorithm) {
	case "BLAKE2B-256", "BLAKE2B-384", "BLAKE2B-512", "BLAKE3", "MD2", "MD4", "MD6":
		return true
	}
	_, known := newHash(algorithm)
	return known
}

// ParseMeta reads the metadata of Table 13 from the bytes of a .meta.json. A
// checksum may be recorded as one object, as the schema has it, or as a list
// of them, as the table's text has it.
func ParseMeta(content []byte) (Meta, error) {
	var raw struct {
		Index           map[string]string          `json:"index"`
		Created         string                     `json:"created"`
		Metamodel       string                     `json:"metamodel"`
		IncludesDerived *bool                      `json:"includesDerived"`
		IncludesImplied *bool                      `json:"includesImplied"`
		Checksum        map[string]json.RawMessage `json:"checksum"`
	}
	if err := decodeObject(content, &raw); err != nil {
		return Meta{}, err
	}
	if raw.Index == nil {
		return Meta{}, errors.New(`the "index" object is missing`)
	}
	if raw.Created == "" {
		return Meta{}, errors.New(`the "created" time is missing`)
	}
	if raw.Metamodel != "" {
		if u, err := url.Parse(raw.Metamodel); err != nil || u.Scheme == "" {
			return Meta{}, fmt.Errorf("the metamodel %q is not an IRI", raw.Metamodel)
		}
	}
	meta := Meta{
		Index:           raw.Index,
		Created:         raw.Created,
		Metamodel:       raw.Metamodel,
		IncludesDerived: raw.IncludesDerived,
		IncludesImplied: raw.IncludesImplied,
	}
	if len(raw.Checksum) > 0 {
		meta.Checksum = make(map[string]Checksums, len(raw.Checksum))
	}
	for member, value := range raw.Checksum {
		var sums Checksums
		if err := json.Unmarshal(value, &sums); err != nil {
			return Meta{}, fmt.Errorf("checksum of %q %w", member, err)
		}
		meta.Checksum[member] = sums
	}
	return meta, nil
}

// decodeObject unmarshals a JSON object, refusing any other value and
// reporting a field of the wrong type by name.
func decodeObject(content []byte, into any) error {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return errors.New("the file is empty")
	}
	if trimmed[0] != '{' {
		return errors.New("the file does not hold a JSON object")
	}
	if err := json.Unmarshal(trimmed, into); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return fmt.Errorf("%q should be %s, not %s", typeErr.Field, typeErr.Type, typeErr.Value)
		}
		return err
	}
	return nil
}

// Names lists every member of the archive, sorted.
func (a *Archive) Names() []string { return append([]string(nil), a.names...) }

// Read returns the bytes of a member.
func (a *Archive) Read(member string) ([]byte, error) {
	content, ok := a.members[member]
	if !ok {
		return nil, fmt.Errorf("%s: no member %q", a.Path, member)
	}
	return content, nil
}

// Files lists the model files of the archive as the paths they load under:
// the archive's path with the member's path appended.
func (a *Archive) Files() []string {
	var out []string
	for _, name := range a.names {
		if IsModelFile(name) {
			out = append(out, MemberPath(a.Path, name))
		}
	}
	return out
}

// MemberPath is the path a member of an archive is named by: the archive's
// path, then the member's path within it.
func MemberPath(archive, member string) string {
	return archive + string(filepath.Separator) + filepath.FromSlash(member)
}

// SplitMemberPath splits a path naming a member of an archive into the archive
// and the member's path within it, reporting whether path is such a path: one
// whose leading directories include an existing .kpar file.
func SplitMemberPath(p string) (archive, member string, ok bool) {
	rest := ""
	for dir := p; ; {
		parent, base := filepath.Split(dir)
		parent = strings.TrimRight(parent, string(filepath.Separator))
		if base == "" || parent == "" || parent == dir {
			return "", "", false
		}
		rest = path.Join(base, rest)
		if IsArchiveFile(parent) {
			return parent, rest, true
		}
		dir = parent
	}
}

// archives caches the archives read, so each is parsed once however many
// members are loaded from it; a rewritten archive is read again.
var archives struct {
	mu   sync.Mutex
	open map[string]cachedArchive
}

type cachedArchive struct {
	archive *Archive
	size    int64
	modTime time.Time
}

// openCached returns the archive at path, read once until it changes on disk.
func openCached(path string) (*Archive, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, &ArchiveError{Path: path, Err: err}
	}
	archives.mu.Lock()
	defer archives.mu.Unlock()
	if c, ok := archives.open[path]; ok && c.size == info.Size() && c.modTime.Equal(info.ModTime()) {
		return c.archive, nil
	}
	a, err := OpenArchive(path)
	if err != nil {
		return nil, err
	}
	if archives.open == nil {
		archives.open = map[string]cachedArchive{}
	}
	archives.open[path] = cachedArchive{archive: a, size: info.Size(), modTime: info.ModTime()}
	return a, nil
}

// readMemberPath reads a member named by its path, as SplitMemberPath splits it.
func readMemberPath(archive, member string) ([]byte, error) {
	a, err := openCached(archive)
	if err != nil {
		return nil, err
	}
	return a.Read(member)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ArchiveWarnings reports what the archives a load touched fell short of: an
// archive under a library root that is not a project interchange file, and a
// checksum of a loaded archive that could not be verified.
func (d Discovery) ArchiveWarnings(paths []string) []string {
	var out []string
	for _, ae := range d.Around(paths).Malformed {
		out = append(out, fmt.Sprintf("%s: not a project interchange archive: %v", ae.Path, ae.Err))
	}
	seen := map[string]bool{}
	for _, p := range paths {
		archive, _, ok := SplitMemberPath(p)
		if !ok {
			if !IsArchiveFile(p) {
				continue
			}
			archive = p
		}
		if seen[archive] {
			continue
		}
		seen[archive] = true
		if a, err := openCached(archive); err == nil {
			for _, note := range a.Notes {
				out = append(out, archive+": "+note)
			}
		}
	}
	return out
}
