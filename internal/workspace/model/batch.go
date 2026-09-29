package model

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Input is one document a batch opens: the name it is indexed under, its text
// and its version. A Transient input is a buffer no file holds, the REPL's
// transcript: it is never read from or written to the record cache.
type Input struct {
	Name      string
	Content   []byte
	Version   int
	Transient bool
}

// DefaultWorkers is the worker count a workspace starts with: one per CPU the
// process may run on.
func DefaultWorkers() int { return runtime.GOMAXPROCS(0) }

// ErrWorkers is the error for a worker count below one.
var ErrWorkers = errors.New("workspace: the worker count must be a positive integer")

// Workers reports how many documents a batch parses and analyzes at once.
func (w *Workspace) Workers() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.workers
}

// SetWorkers sets how many documents a batch parses and analyzes at once. The
// result of a batch is the same at any count; a count below one is an error.
func (w *Workspace) SetWorkers(n int) error {
	if n < 1 {
		return fmt.Errorf("%w: %d", ErrWorkers, n)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.workers = n
	return nil
}

// OpenAll opens the inputs as one batch: parsed on the workers, added to the index
// in order, wildcard imports expanded once. Same result as opening them one by one
// as the batch starts: a document changed by another caller meanwhile keeps that change.
//
// Over a record cache, an input whose content the cache holds a record for is
// held as that record, a closed file, where the record's provenance holds among
// the documents opened (see libs.Provenance); the others, and the records that
// do not hold, are parsed.
func (w *Workspace) OpenAll(inputs []Input) {
	was := w.reserveBatch(inputs)
	recs := w.cachedRecords(inputs)
	docs := make([]batchDoc, len(inputs))
	ParallelFor(w.Workers(), len(inputs), func(i int) {
		in := inputs[i]
		if rec := recs[i]; rec != nil {
			if scope, err := symbols.BuildRecorded(rec.Scope, rec.Name); err == nil {
				docs[i] = batchDoc{rec: rec, scope: scope, content: bytes.Clone(in.Content), version: in.Version}
				return
			}
		}
		docs[i] = batchDoc{doc: newDocument(in.Name, bytes.Clone(in.Content), in.Version)}
	})
	w.commitBatch(was, docs)
}

// batchDoc is one document a batch installs: parsed, or built from its record.
type batchDoc struct {
	doc     *Document
	rec     *libs.InterfaceRecord
	scope   *symbols.Scope
	content []byte
	version int
}

func (d batchDoc) name() string {
	if d.doc != nil {
		return d.doc.Name
	}
	return d.rec.Name
}

// cachedRecords is the record cache's record of each input's content, nil where
// there is none, no cache, or the record answers another question.
func (w *Workspace) cachedRecords(inputs []Input) []*libs.InterfaceRecord {
	recs := make([]*libs.InterfaceRecord, len(inputs))
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.records == nil {
		return recs
	}
	keys := make([]string, len(inputs))
	for i, in := range inputs {
		if !in.Transient {
			keys[i], _ = w.recordKeyLocked(in.Name, in.Content)
		}
	}
	ParallelFor(w.workers, len(inputs), func(i int) {
		if keys[i] == "" {
			return
		}
		if rec, ok := w.records.LoadInterface(keys[i]); ok && w.recordAcceptedLocked(rec) == nil {
			recs[i] = rec
		}
	})
	return recs
}

// reserveBatch is each input's name's change count as the batch starts, which is
// what commitBatch installs over: a name opened and removed meanwhile is absent
// again, but its count has moved.
func (w *Workspace) reserveBatch(inputs []Input) map[string]uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	was := make(map[string]uint64, len(inputs))
	for _, in := range inputs {
		was[in.Name] = w.changes[in.Name]
	}
	return was
}

// commitBatch installs the parsed and recorded documents whose name is as the
// batch reserved it; a name changed since keeps its newer state. A record whose
// provenance does not hold among the documents installed is parsed in its place.
func (w *Workspace) commitBatch(was map[string]uint64, docs []batchDoc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var installed, recorded []string
	for _, d := range docs {
		name := d.name()
		if w.changes[name] != was[name] {
			continue
		}
		if d.doc != nil {
			w.open[name] = true
			w.docs[name] = d.doc
			w.changes[name]++
			w.installLocked(d.doc)
		} else {
			w.installRecordedLocked(d.rec, d.scope, d.content, d.version)
			recorded = append(recorded, name)
		}
		was[name] = w.changes[name]
		installed = append(installed, name)
	}
	if len(installed) == 0 {
		return
	}
	w.index.ExpandWildcardImports()
	w.invalidateLocked(installed...)
	if len(recorded) == 0 {
		return
	}
	src := w.sourcesLocked()
	byName := make(map[string]batchDoc, len(docs))
	for _, d := range docs {
		byName[d.name()] = d
	}
	var stale []string
	for _, name := range recorded {
		if doc := w.docs[name]; doc.Recorded() && !byName[name].rec.Provenance.Valid(src) {
			stale = append(stale, name)
		}
	}
	if len(stale) == 0 {
		return
	}
	parsed := make([]*Document, len(stale))
	ParallelFor(w.workers, len(stale), func(i int) {
		held := w.docs[stale[i]]
		parsed[i] = newDocument(held.Name, held.Content, held.Version)
	})
	for i, name := range stale {
		w.docs[name] = parsed[i]
		w.changes[name]++
		w.installLocked(parsed[i])
	}
	w.index.ExpandWildcardImports()
	w.invalidateLocked(stale...)
}

// DiagnosticsAll returns the named documents' diagnostics in the order named (nil
// for an unknown name), analyzing the uncached ones on the workers, then caching.
// The workers share one gather of the workspace-wide audits, made on first use.
// Pending regathers settle first, so no cached entry they would drop is served.
func (w *Workspace) DiagnosticsAll(names []string) [][]diag.Diagnostic {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.settleGathersLocked()
	out := make([][]diag.Diagnostic, len(names))
	var pending []string
	queued := map[string]bool{}
	for i, name := range names {
		doc := w.docs[name]
		if doc == nil {
			continue
		}
		if doc.Recorded() {
			out[i] = doc.recorded.diagnostics
		} else if cached, ok := w.diagCache[name]; ok {
			out[i] = cached
		} else if !queued[name] {
			queued[name] = true
			pending = append(pending, name)
		}
	}
	batch := &passes.Batch{Documents: pending, Gathers: passes.NewGathers(), Source: w.sourceText(), Record: w.records != nil}
	passes.PrepareBatch(w.index, batch)
	analyzed := make([][]diag.Diagnostic, len(pending))
	reads := make([]*resolve.Reads, len(pending))
	ParallelFor(w.workers, len(pending), func(i int) {
		analyzed[i], reads[i] = w.analyze(pending[i], w.docs[pending[i]], batch)
	})
	for i, name := range pending {
		w.diagCache[name] = analyzed[i]
		w.batched[name] = reads[i]
	}
	w.writeRecordsLocked(pending, batch)
	for i, name := range names {
		if doc := w.docs[name]; out[i] == nil && doc != nil && !doc.Recorded() {
			out[i] = w.diagCache[name]
		}
		out[i] = passes.WithoutLints(out[i], w.disabledLints)
	}
	return out
}

// ParallelFor runs fn(i) for every i below n on up to workers goroutines, and
// returns once every call has; it is how a batch spreads its documents.
func ParallelFor(workers, n int, fn func(i int)) {
	workers = min(workers, n)
	if workers <= 1 {
		for i := 0; i < n; i++ {
			fn(i)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}
