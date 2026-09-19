package libs

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// maxIdleAge is how long a record no load has looked up is kept. A key describes
// the content it was built from, so a record whose key stops being produced —
// after a format bump, or an edit to any file of its library — is never found
// again and only takes up space.
const maxIdleAge = 30 * 24 * time.Hour

// Cache persists reduced library IndexRecords to disk keyed by content hash and
// format version. A hit lets the loader skip lexing/parsing entirely. The same
// mechanism serves git dependencies (Plan 5c) since it keys purely on content.
type Cache struct {
	dir string
}

// NewCache resolves the cache directory ($XDG_CACHE_HOME or os.UserCacheDir),
// appends "sysml-ls/libs", creates it, and returns a ready Cache.
func NewCache() (*Cache, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		d, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		base = d
	}
	dir := filepath.Join(base, "sysml-ls", "libs")
	// #nosec G703 -- the base directory is the user's own XDG_CACHE_HOME or OS
	// cache directory, and the joined suffix is a constant.
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Cache{dir: dir}, nil
}

// keyFor derives a cache key from the file content, the digest of the library
// set it belongs to, the build that produced the record, and the format
// version. Any change yields a distinct key, so stale entries are simply never
// found (miss) rather than requiring explicit invalidation. The set digest is
// part of the key because a record holds values derived from sibling files — a
// unit reduction follows a reference unit or prefix declared elsewhere — which
// the file's own content does not cover. The build ID is part of it because a
// record also holds values the code computes, such as a symbol kind, which no
// input covers.
func (c *Cache) keyFor(content []byte, setDigest string) string {
	return c.keyForSum(sha256.Sum256(content), setDigest)
}

// keyForSum is keyFor given the file content's sha256 sum.
func (c *Cache) keyForSum(sum [sha256.Size]byte, setDigest string) string {
	return hex.EncodeToString(sum[:]) + "-s" + setDigest + "-b" + buildID() + "-v" + strconv.Itoa(formatVersion)
}

func (c *Cache) path(key string) string {
	return filepath.Join(c.dir, key+".idx")
}

// Load returns the cached record for key, or (nil, false) on any miss
// (absent file, read error, or decode error — all treated as a benign miss).
func (c *Cache) Load(key string) (*IndexRecord, bool) {
	// #nosec G304 G703 -- the path is <cache dir>/<content hash>.idx; the key is
	// hex-encoded SHA-256 computed by keyFor, never caller-supplied text.
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return nil, false
	}
	var rec IndexRecord
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&rec); err != nil {
		return nil, false
	}
	// Date the hit, so Prune reads the file's age as the age of its last use.
	now := time.Now()
	_ = os.Chtimes(c.path(key), now, now)
	return &rec, true
}

// Prune removes records that no load has hit for maxIdleAge, and the temp files
// a crashed store left behind. Cache entries are
// otherwise never invalidated explicitly — a key that no longer matches is
// simply missed — so without this the directory keeps every record any library
// version ever produced.
func (c *Cache) Prune() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxIdleAge)
	for _, e := range entries {
		if e.IsDir() || !(filepath.Ext(e.Name()) == ".idx" || strings.Contains(e.Name(), ".idx.tmp")) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		// #nosec G703 -- removal failure leaves a stale record, which is benign.
		_ = os.Remove(filepath.Join(c.dir, e.Name()))
	}
}

// Store gob-encodes rec and writes it to <dir>/<key>.idx atomically: it writes a
// temp file of its own then renames it into place, so a concurrent Load or a
// crash never observes a partially written file, and two writers of one key
// cannot truncate or publish each other's temp. A failed encode/write removes
// the temp and leaves any existing final file untouched.
func (c *Cache) Store(key string, rec *IndexRecord) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rec); err != nil {
		return err
	}
	final := c.path(key)
	f, err := os.CreateTemp(c.dir, key+".idx.tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := writeTemp(f, buf.Bytes()); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	if err := os.Rename(tmp, final); err != nil {
		if rmErr := os.Remove(tmp); rmErr != nil {
			return errors.Join(err, rmErr)
		}
		return err
	}
	return nil
}

// writeTemp writes data to f with the permissions a record is kept at, and
// closes f whatever happens.
func writeTemp(f *os.File, data []byte) error {
	if err := f.Chmod(0o600); err != nil {
		return errors.Join(err, f.Close())
	}
	if _, err := f.Write(data); err != nil {
		return errors.Join(err, f.Close())
	}
	return f.Close()
}
