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
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Input is one document a batch opens: the name it is indexed under, its text
// and its version. Kind is optional; when unknown the name determines it. A
// Transient input is a buffer no file holds, the REPL's transcript: it is never
// read from or written to the record cache.
type Input struct {
	Name      string
	Content   []byte
	Version   int
	Kind      source.Kind
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
	w.installBatch(inputs, true)
}

// SetOnDiskAll is SetOnDisk over the inputs as one batch: the files' content is
// recorded for them all, and those without an open buffer are parsed on the
// workers, added to the index in order and wildcard imports expanded once, as
// OpenAll does; an open buffer stays authoritative. Version is each file's
// document version, zero as SetOnDisk gives it.
func (w *Workspace) SetOnDiskAll(inputs []Input) {
	w.installBatch(w.recordOnDisk(inputs), false)
}

// recordOnDisk records each input's content as its file's, and returns the
// inputs whose name has no open buffer, each of the kind of the document it replaces.
func (w *Workspace) recordOnDisk(inputs []Input) []Input {
	w.mu.Lock()
	defer w.mu.Unlock()
	closed := make([]Input, 0, len(inputs))
	for _, in := range inputs {
		w.onDisk[in.Name] = bytes.Clone(in.Content)
		if w.open[in.Name] {
			continue
		}
		if held := w.docs[in.Name]; held != nil && in.Kind == source.KindUnknown {
			in.Kind = held.Kind()
		}
		closed = append(closed, in)
	}
	return closed
}

// installBatch parses the inputs on the workers and installs them as one batch,
// marking them open when open says so.
func (w *Workspace) installBatch(inputs []Input, open bool) {
	if len(inputs) == 0 {
		return
	}
	was := w.reserveBatch(inputs)
	recs, keys := w.cachedRecords(inputs)
	docs := make([]batchDoc, len(inputs))
	ParallelFor(w.Workers(), len(inputs), func(i int) {
		in := inputs[i]
		if rec := recs[i]; rec != nil {
			if scope, err := symbols.BuildRecorded(rec.Scope, rec.Name); err == nil {
				docs[i] = batchDoc{rec: rec, key: keys[i], scope: scope, content: bytes.Clone(in.Content), version: in.Version}
				return
			}
		}
		docs[i] = batchDoc{doc: newDocument(in.Name, bytes.Clone(in.Content), in.Version, in.Kind)}
	})
	w.commitBatch(was, docs, open)
}

// batchDoc is one document a batch installs: parsed, or built from its record,
// with the key the record was found under.
type batchDoc struct {
	doc     *Document
	rec     *libs.InterfaceRecord
	key     string
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

// cachedRecords is the record cache's record of each input's content and the
// key it was found under; nil where there is none or the record answers another question.
func (w *Workspace) cachedRecords(inputs []Input) ([]*libs.InterfaceRecord, []string) {
	recs := make([]*libs.InterfaceRecord, len(inputs))
	keys := make([]string, len(inputs))
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.records == nil {
		return recs, keys
	}
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
			if rec.Kind == inputKind(inputs[i]) {
				recs[i] = rec
			}
		}
	})
	return recs, keys
}

func inputKind(in Input) source.Kind {
	if in.Kind != source.KindUnknown {
		return in.Kind
	}
	return source.KindOf(in.Name)
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
// provenance or key does not hold once the batch is in is parsed in its place.
func (w *Workspace) commitBatch(was map[string]uint64, docs []batchDoc, open bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var installed, recorded []string
	for _, d := range docs {
		name := d.name()
		if w.changes[name] != was[name] {
			continue
		}
		if d.doc != nil {
			if open {
				w.open[name] = true
			}
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
		doc := w.docs[name]
		if !doc.Recorded() {
			continue
		}
		// A library version among the documents moves the identity the key names.
		if key, ok := w.recordKeyLocked(name, doc.Content); !ok || key != byName[name].key || !byName[name].rec.Provenance.Valid(src) {
			stale = append(stale, name)
		}
	}
	if len(stale) == 0 {
		return
	}
	parsed := make([]*Document, len(stale))
	ParallelFor(w.workers, len(stale), func(i int) {
		held := w.docs[stale[i]]
		parsed[i] = newDocument(held.Name, held.Content, held.Version, held.Kind())
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
		w.stampLocked(name)
	}
	w.writeRecordsLocked(pending, batch)
	for i, name := range names {
		if doc := w.docs[name]; out[i] == nil && doc != nil && !doc.Recorded() {
			out[i] = w.diagCache[name]
		}
		out[i] = passes.WithoutLints(out[i], w.disabledLints, w.enabledLints)
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
