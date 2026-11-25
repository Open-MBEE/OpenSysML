package libs

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// modelFormatVersion is the on-disk format version of a model record. Bump it
// whenever ModelRecord or diag.Diagnostic changes shape or meaning.
const modelFormatVersion = 2

// maxModelRecordBytes bounds what a cache directory's model records take together: an
// edit to any document leaves the model's old record behind, so the least recently used go.
const maxModelRecordBytes = 64 << 20

// modelRecordSuffix ends a model record's key, which tells its file from the
// other records of the directory.
var modelRecordSuffix = "-m" + strconv.Itoa(modelFormatVersion)

// ModelRecord is what the analysis of a whole model reported, per document in the model's
// order; filed under every document's content, it holds wherever it is found.
type ModelRecord struct {
	Documents []ModelDocument
}

// ModelDocument is one document's part of a ModelRecord, with the reads its
// analysis made so a restored workspace needs no passes to prune on an edit.
type ModelDocument struct {
	Name        string
	Kind        source.Kind
	Diagnostics []diag.Diagnostic
	Reads       *resolve.Reads
}

// ModelKey derives a model record's key from the library's identity, the build, the format and
// modelDigest: a hex digest of every document's name, kind and content and the conformance mode.
func (c *Cache) ModelKey(modelDigest, libraryIdentity string) string {
	return modelDigest + "-l" + libraryIdentity + "-b" + buildID() + modelRecordSuffix
}

// LoadModel returns the model record stored under key, or (nil, false) on any
// miss.
func (c *Cache) LoadModel(key string) (*ModelRecord, bool) {
	var rec ModelRecord
	if !c.load(key, &rec) {
		return nil, false
	}
	return &rec, true
}

// StoreModel writes rec under key, atomically (see Store), and evicts the least recently
// used model records once enough was stored since the last eviction to pass their bound.
func (c *Cache) StoreModel(key string, rec *ModelRecord) error {
	if err := c.store(key, rec); err != nil {
		return err
	}
	// A process's first record prunes the directory: processes storing little each still bound it.
	pruned := false
	c.modelPruneOnce.Do(func() { pruned = true; c.Prune() })
	if pruned {
		return nil
	}
	// #nosec G703 -- the path is the one store just wrote.
	if info, err := os.Stat(c.path(key)); err == nil {
		if c.modelBytesStored.Add(info.Size()) >= maxModelRecordBytes/4 {
			c.pruneModels(maxModelRecordBytes)
		}
	}
	return nil
}

// pruneModels removes the least recently used model records until those left take no
// more than limit bytes; a load dates the record it hits.
func (c *Cache) pruneModels(limit int64) {
	c.modelBytesStored.Store(0)
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	var records []fs.FileInfo
	var total int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), modelRecordSuffix+".idx") {
			continue
		}
		info, err := e.Info()
		if err != nil {
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
		// #nosec G703 -- removal failure leaves a record over the bound, which the next eviction retries.
		if os.Remove(filepath.Join(c.dir, info.Name())) == nil {
			total -= info.Size()
		}
	}
}
