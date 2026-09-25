package model

import (
	"bytes"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Input is one document a batch opens: the name it is indexed under, its text
// and its version.
type Input struct {
	Name    string
	Content []byte
	Version int
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
func (w *Workspace) OpenAll(inputs []Input) {
	was := w.reserveBatch(inputs)
	docs := make([]*Document, len(inputs))
	ParallelFor(w.Workers(), len(inputs), func(i int) {
		in := inputs[i]
		docs[i] = newDocument(in.Name, bytes.Clone(in.Content), in.Version)
	})
	w.commitBatch(was, docs)
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

// commitBatch installs the parsed documents whose name is as the batch reserved
// it; a name changed since keeps its newer state.
func (w *Workspace) commitBatch(was map[string]uint64, docs []*Document) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var installed []string
	for _, doc := range docs {
		if w.changes[doc.Name] != was[doc.Name] {
			continue
		}
		w.open[doc.Name] = true
		w.docs[doc.Name] = doc
		w.changes[doc.Name]++
		was[doc.Name] = w.changes[doc.Name]
		w.installLocked(doc)
		installed = append(installed, doc.Name)
	}
	if len(installed) > 0 {
		w.index.ExpandWildcardImports()
		w.invalidateLocked(installed...)
	}
}

// DiagnosticsAll returns the named documents' diagnostics in the order named (nil
// for an unknown name), analyzing the uncached ones on the workers, then caching.
// The workers share one gather of the workspace-wide audits, made on first use.
func (w *Workspace) DiagnosticsAll(names []string) [][]diag.Diagnostic {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([][]diag.Diagnostic, len(names))
	var pending []string
	queued := map[string]bool{}
	for i, name := range names {
		if w.docs[name] == nil {
			continue
		}
		if cached, ok := w.diagCache[name]; ok {
			out[i] = cached
		} else if !queued[name] {
			queued[name] = true
			pending = append(pending, name)
		}
	}
	batch := &passes.Batch{Documents: pending, Gathers: passes.NewGathers(), Source: w.sourceText()}
	passes.PrepareBatch(w.index, batch)
	analyzed := make([][]diag.Diagnostic, len(pending))
	ParallelFor(w.workers, len(pending), func(i int) {
		analyzed[i] = w.analyze(pending[i], w.docs[pending[i]], batch)
	})
	for i, name := range pending {
		w.diagCache[name] = analyzed[i]
		w.batched[name] = true
	}
	for i, name := range names {
		if out[i] == nil && w.docs[name] != nil {
			out[i] = w.diagCache[name]
		}
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
