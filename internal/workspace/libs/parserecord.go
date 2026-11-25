package libs

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
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

// parseFormatVersion is the on-disk format of a parse record. Bump it whenever
// ParseRecord or the encoding of its AST stream changes shape or meaning.
const parseFormatVersion = 1

// maxParseRecordBytes bounds a cache directory's parse records together;
// the least recently used go, independently of the analysis records' bound.
const maxParseRecordBytes = 64 << 20

// parseRecordSuffix ends a parse record's key, telling its file from the other
// records of the directory.
var parseRecordSuffix = "-p" + strconv.Itoa(parseFormatVersion)

// parseRecordDir keeps parse records apart from the analysis records at the root;
// a reader of one kind must never see the other's files.
func (c *Cache) parseRecordDir() string { return filepath.Join(c.dir, "parses") }

// ErrDamagedParseRecord is the typed miss for a record whose payload digest
// disagrees with its contents, including one gob could decode into wrong data.
var ErrDamagedParseRecord = errors.New("damaged parse record")

// ParseRecord is one document's parse, filed by content; AST is rooted at RootID
// and Digest covers every other field, so damage is a miss, not an answer.
type ParseRecord struct {
	Name          string
	Kind          source.Kind
	Language      string
	AST           []byte
	RootID        uint64
	ParseDiags    []parser.Diagnostic // parse errors
	ParseWarnings []parser.Diagnostic // parser warnings, errors of their own kind to the reporter
	Warnings      []string            // warnings from reading the file, not from parsing it
	Digest        [sha256.Size]byte   // digest of the fields above
}

// payloadDigest hashes every field but Digest itself. A copy is hashed so a
// store leaves its caller's record untouched.
func (r *ParseRecord) payloadDigest() [sha256.Size]byte {
	copy := *r
	copy.Digest = [sha256.Size]byte{}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&copy); err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(buf.Bytes())
}

// ParseKey keys a parse by name, language, kind, content and build; the fields
// are length-delimited. Conformance mode is absent: parsing does not depend on it.
func (c *Cache) ParseKey(name, language string, kind source.Kind, content []byte) string {
	h := sha256.New()
	var head [binary.MaxVarintLen64]byte
	for _, field := range []string{name, language, kind.String()} {
		n := binary.PutUvarint(head[:], uint64(len(field)))
		_, _ = h.Write(head[:n])
		_, _ = h.Write([]byte(field))
	}
	n := binary.PutUvarint(head[:], uint64(len(content)))
	_, _ = h.Write(head[:n])
	_, _ = h.Write(content)
	return hex.EncodeToString(h.Sum(nil)) + "-b" + buildID() + parseRecordSuffix
}

// LoadParse returns the parse record stored under key, or (nil, false) on any
// miss, including a damaged record. A miss is never an error: the caller parses.
func (c *Cache) LoadParse(key string) (*ParseRecord, bool) {
	rec, err := c.LoadParseChecked(key)
	return rec, err == nil && rec != nil
}

// LoadParseChecked returns the record under key, a typed miss for damage, and
// (nil, nil) when absent; a damaged file is removed so a store rewrites it.
func (c *Cache) LoadParseChecked(key string) (*ParseRecord, error) {
	// #nosec G703 -- key is hex SHA-256 the cache computed, not caller text.
	path := filepath.Join(c.parseRecordDir(), key+".idx")
	var rec ParseRecord
	if !loadAt(path, &rec) {
		if _, err := os.Stat(path); err == nil {
			_ = os.Remove(path)
			return nil, ErrDamagedParseRecord
		}
		return nil, nil
	}
	if rec.Digest != rec.payloadDigest() {
		_ = os.Remove(path)
		return nil, ErrDamagedParseRecord
	}
	return &rec, nil
}

// StoreParse writes rec under key atomically and evicts the least recently used
// parse records once their bound is passed.
func (c *Cache) StoreParse(key string, rec *ParseRecord) error {
	dir := c.parseRecordDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	rec.Digest = rec.payloadDigest()
	// #nosec G703 -- key is hex SHA-256 the cache computed, not caller text.
	final := filepath.Join(dir, key+".idx")
	if err := storeAt(dir, final, rec); err != nil {
		return err
	}
	pruned := false
	c.parsePruneOnce.Do(func() { pruned = true; c.pruneParses(maxParseRecordBytes) })
	if pruned {
		return nil
	}
	if info, err := os.Stat(final); err == nil {
		if c.parseBytesStored.Add(info.Size()) >= maxParseRecordBytes/4 {
			c.pruneParses(maxParseRecordBytes)
		}
	}
	return nil
}

// pruneParses removes idle temporary files from crashed stores and the least
// recently used parse records until those left take no more than limit bytes.
func (c *Cache) pruneParses(limit int64) {
	c.parseBytesStored.Store(0)
	entries, err := os.ReadDir(c.parseRecordDir())
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxIdleAge)
	var records []fs.FileInfo
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if strings.Contains(e.Name(), ".idx.tmp") {
			// A store that died mid-write left this; no load can find it.
			if info.ModTime().Before(cutoff) {
				// #nosec G703 -- a removal failure leaves a stale temp; the next prune retries.
				_ = os.Remove(filepath.Join(c.parseRecordDir(), e.Name()))
			}
			continue
		}
		if !strings.HasSuffix(e.Name(), parseRecordSuffix+".idx") {
			continue
		}
		records = append(records, info)
		total += info.Size()
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ModTime().Before(records[j].ModTime()) })
	for _, info := range records {
		if total <= limit {
			return
		}
		// #nosec G703 -- a removal failure leaves a record over the bound; the next eviction retries.
		if os.Remove(filepath.Join(c.parseRecordDir(), info.Name())) == nil {
			total -= info.Size()
		}
	}
}
