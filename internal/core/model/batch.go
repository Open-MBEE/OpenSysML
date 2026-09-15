package model

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// Input is one document a batch opens: the name it is indexed under, its text
// and its version.
type Input struct {
	Name    string
	Content []byte
	Version int
}

// WorkersEnvVar names the environment variable that sets how many documents a
// batch parses and analyzes at once when no setting of the caller's does.
const WorkersEnvVar = "OPENSYSML_WORKERS"

// DefaultWorkers is the worker count a workspace starts with: one per CPU the
// process may run on.
func DefaultWorkers() int { return runtime.GOMAXPROCS(0) }

// WorkersError reports a worker count that is not a positive integer, naming
// the setting it came from.
type WorkersError struct {
	Source string
	Value  string
}

// Error names the source and the value, and what a usable one is.
func (e *WorkersError) Error() string {
	return fmt.Sprintf("%s=%q is not a positive integer: set it to how many documents may be parsed and analyzed at once (default %d, one per CPU)", e.Source, e.Value, DefaultWorkers())
}

// ParseWorkers reads a worker count written at source: a positive integer, else
// a WorkersError.
func ParseWorkers(source, text string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || n < 1 {
		return 0, &WorkersError{Source: source, Value: text}
	}
	return n, nil
}

// WorkersFromEnv is the worker count OPENSYSML_WORKERS asks for, DefaultWorkers
// when it is unset or empty; a value that is not a positive integer is an error.
func WorkersFromEnv() (int, error) {
	return workersFromLookup(os.Getenv)
}

// workersFromLookup is WorkersFromEnv over an explicit lookup, so the parsing is
// testable without the process environment.
func workersFromLookup(lookup func(string) string) (int, error) {
	raw := lookup(WorkersEnvVar)
	if strings.TrimSpace(raw) == "" {
		return DefaultWorkers(), nil
	}
	return ParseWorkers(WorkersEnvVar, raw)
}

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
		return &WorkersError{Source: "workers", Value: strconv.Itoa(n)}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.workers = n
	return nil
}

// OpenAll opens every input as an authoritative buffer in one batch: the
// documents are parsed and their scope trees built on the workers, then added
// to the index in the order given, and the wildcard imports of the whole batch
// are expanded once. It leaves the workspace as opening the inputs one by one
// would, at a cost that does not grow with the number already open.
func (w *Workspace) OpenAll(inputs []Input) {
	docs := make([]*Document, len(inputs))
	ParallelFor(w.Workers(), len(inputs), func(i int) {
		in := inputs[i]
		docs[i] = newDocument(in.Name, bytes.Clone(in.Content), in.Version)
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, doc := range docs {
		w.open[doc.Name] = true
		w.docs[doc.Name] = doc
		w.index.AddBuiltDocument(doc.Name, doc.AST, doc.Scope)
	}
	w.index.ExpandWildcardImports()
	w.invalidateLocked()
}

// DiagnosticsAll returns the diagnostics of the named documents, in the order
// named, with nil for a name the workspace does not hold. The documents not yet
// analyzed since the last change are analyzed on the workers, each with a
// context of its own over the index, which nothing writes meanwhile; the
// results are cached as Diagnostics caches them.
func (w *Workspace) DiagnosticsAll(names []string) [][]passes.Diagnostic {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([][]passes.Diagnostic, len(names))
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
	batch := &passes.Batch{Documents: pending}
	passes.PrepareBatch(w.index, batch)
	analyzed := make([][]passes.Diagnostic, len(pending))
	ParallelFor(w.workers, len(pending), func(i int) {
		analyzed[i] = w.analyze(pending[i], w.docs[pending[i]], batch)
	})
	for i, name := range pending {
		w.diagCache[name] = analyzed[i]
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
