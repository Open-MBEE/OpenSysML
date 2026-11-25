package libs

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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

// ParseRecord is one document's parse, filed under its content for another process.
// AST is an astcodec stream rooted at RootID; the index and analysis are derived.
type ParseRecord struct {
	Name          string
	Kind          source.Kind
	Language      string
	AST           []byte
	RootID        uint64
	ParseDiags    []parser.Diagnostic // parse errors
	ParseWarnings []parser.Diagnostic // parser warnings, errors of their own kind to the reporter
	Warnings      []string            // warnings from reading the file, not from parsing it
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
	var rec ParseRecord
	// #nosec G703 -- key is hex SHA-256 the cache computed, not caller text.
	if !loadAt(filepath.Join(c.parseRecordDir(), key+".idx"), &rec) {
		return nil, false
	}
	return &rec, true
}

// StoreParse writes rec under key atomically and evicts the least recently used
// parse records once their bound is passed.
func (c *Cache) StoreParse(key string, rec *ParseRecord) error {
	dir := c.parseRecordDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
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
