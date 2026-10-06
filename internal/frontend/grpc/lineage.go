package grpc

import (
	"container/list"
	"sort"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// A lineage is the incremental workspace behind the models one client parses
// over and over: the same documents, by name, at one conformance mode, with
// some of their content changed between calls (an editor's model, a build
// that recompiles after an edit). ParseSources is answered from it when it
// holds the request's documents, so a call re-parses only the documents whose
// content changed and re-analyzes only those and the documents that depend on
// them; the workspace drops exactly that set (#316, #637).
//
// A cached model must not change once its hash is handed out, and the
// workspace is edited by the next call. So the model a lineage answers with
// is a detached copy: the documents' trees and the diagnostics as they stood,
// on an index of the model's own (model.Workspace.Detach).
//
// Incremental equals fresh: a model answered from a lineage has the documents,
// diagnostics and index a fresh parse of the same documents would have
// (lineage_test.go). A model whose documents do not all parse clean is never
// answered from a lineage: the fresh path does not analyze it at all.
type lineage struct {
	mu sync.Mutex
	// fresh is set for a document set the fresh path must answer: one holding a
	// version of a bundled library file, which the workspace marks with the
	// library's tier and a fresh parse does not (see parseFromLineage).
	// standIns holds those documents' contents: once one changes, the set is
	// noted afresh, so it can be answered from a workspace again.
	fresh    bool
	standIns map[string]string
	ws       *model.Workspace  // nil until the document set is parsed a second time
	library  libs.Source       // the files the workspace's library was built from
	held     map[string]string // document name to the content the workspace holds
	versions int               // the version the next update is given
}

// lineages holds the most recently used lineages, a few at most: a lineage
// holds a workspace's trees and caches, and a service serves few clients.
type lineages struct {
	mu    sync.Mutex
	limit int
	order *list.List // most recently used first; values are *lineageEntry
	byKey map[string]*list.Element
}

type lineageEntry struct {
	key string
	l   *lineage
}

// maxLineages bounds the lineages a service keeps.
const maxLineages = 4

func newLineages(limit int) *lineages {
	return &lineages{limit: limit, order: list.New(), byKey: map[string]*list.Element{}}
}

// lineageKey names the lineage of a document set: the conformance mode and the
// documents' names and languages, in any order.
func lineageKey(inputs []sourceInput, mode diag.ConformanceMode) string {
	parts := make([]string, len(inputs))
	for i, input := range inputs {
		parts[i] = input.name + "\x00" + input.language
	}
	sort.Strings(parts)
	return mode.String() + "\x01" + strings.Join(parts, "\x01")
}

// get is the lineage for key, created by create when none is held; the least
// recently used one goes when the limit is reached.
func (ls *lineages) get(key string, create func() *lineage) (*lineage, bool) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if el, ok := ls.byKey[key]; ok {
		ls.order.MoveToFront(el)
		return el.Value.(*lineageEntry).l, true
	}
	l := create()
	ls.byKey[key] = ls.order.PushFront(&lineageEntry{key: key, l: l})
	for ls.order.Len() > ls.limit {
		oldest := ls.order.Back()
		ls.order.Remove(oldest)
		delete(ls.byKey, oldest.Value.(*lineageEntry).key)
	}
	return l, false
}

// drop forgets the lineage for key, so the next request of its documents
// starts a fresh one.
func (ls *lineages) drop(key string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if el, ok := ls.byKey[key]; ok {
		ls.order.Remove(el)
		delete(ls.byKey, key)
	}
}

// parseFromLineage answers a parse of inputs from the lineage of their
// documents, or reports false when the fresh path should answer it: the first
// parse of a document set only notes the set, so a one-off parse never pays
// for a workspace it will not reuse, and a document that does not parse clean
// leaves the model to the fresh path, which does not analyze it.
func (s *Service) parseFromLineage(inputs []sourceInput, mode diag.ConformanceMode) (*CachedModel, bool) {
	key := lineageKey(inputs, mode)
	l, _ := s.lineages.get(key, func() *lineage { return &lineage{held: map[string]string{}} })
	l.mu.Lock()
	defer l.mu.Unlock()

	switch {
	case l.fresh:
		for _, input := range inputs {
			if content, stood := l.standIns[input.name]; stood && content != input.content {
				// A library version changed: noted as if first seen, so the
				// next parse builds a workspace and checks it again.
				l.fresh, l.standIns, l.held = false, nil, map[string]string{}
				for _, input := range inputs {
					l.held[input.name] = input.content
				}
				break
			}
		}
		return nil, false
	case l.ws == nil && len(l.held) == 0:
		// First seen: noted, answered fresh.
		for _, input := range inputs {
			l.held[input.name] = input.content
		}
		return nil, false
	case l.ws == nil:
		// Seen again: the workspace is built over every document now.
		idx, library := s.libIndexes.get()
		l.ws = model.NewWorkspaceWithIndex(idx, model.WithConformanceMode(mode), model.WithLibrarySource(library))
		l.library = library
		batch := make([]model.Input, len(inputs))
		for i, input := range inputs {
			batch[i] = model.Input{Name: input.name, Content: []byte(input.content), Kind: input.kind}
			l.held[input.name] = input.content
		}
		l.ws.OpenAll(batch)
	default:
		for _, input := range inputs {
			if l.held[input.name] == input.content {
				continue
			}
			l.versions++
			l.ws.Update(input.name, []byte(input.content), l.versions)
			l.held[input.name] = input.content
		}
	}

	// A document that versions a bundled library file is indexed by the
	// workspace under the library's tier, which changes what the passes report
	// about it; a fresh parse indexes it as the request's own. The set is
	// answered fresh while those documents stay as they are, so the two never
	// disagree.
	standIns := map[string]string{}
	for _, input := range inputs {
		if l.ws.StandsIn(input.name) {
			standIns[input.name] = input.content
		}
	}
	if len(standIns) > 0 {
		l.fresh, l.standIns, l.ws, l.held = true, standIns, nil, nil
		return nil, false
	}
	names := make([]string, len(inputs))
	for i, input := range inputs {
		names[i] = input.name
		if doc := l.ws.Document(input.name); doc == nil || len(doc.ParseDiagnostics) > 0 {
			return nil, false
		}
	}
	// One document at a time, in the workspace's shared context, which records
	// what each analysis read: an edit then drops only the documents whose reads
	// it changed. DiagnosticsAll analyzes in contexts of their own, which any
	// edit drops entirely, so it is not incremental.
	diagnostics := make([][]diag.Diagnostic, len(names))
	for i, name := range names {
		diagnostics[i] = l.ws.Diagnostics(name)
	}
	detached, err := l.ws.Detach()
	if err != nil {
		// A workspace that cannot be detached is not trusted for the next call.
		s.lineages.drop(key)
		return nil, false
	}

	documents := make([]*CachedDocument, len(inputs))
	for i, input := range inputs {
		doc := l.ws.Document(input.name)
		documents[i] = &CachedDocument{
			Root:        doc.AST,
			Source:      detached.Source(input.name),
			ParseDiags:  doc.ParseDiagnostics,
			Diagnostics: diagnostics[i],
			Warnings:    append([]string(nil), input.warnings...),
		}
	}
	stamps := make(map[string]uint64, len(names))
	for _, name := range names {
		stamps[name] = l.ws.AnalysisStamp(name)
	}
	return &CachedModel{
		Documents: documents, Index: detached.Index(), Library: l.library, Mode: mode,
		analysis: &analysisSnapshot{workspace: l.ws, stamps: stamps, scoped: declaresIdentityScope(inputs)},
	}, true
}

// analysisSnapshot is which analysis of each document a model was answered
// with, as the lineage's workspace numbers them (model.Workspace.AnalysisStamp).
// A stamp changes exactly when the document is analyzed afresh, which happens
// only after an edit dropped something its analysis read; so a document with
// one stamp in two models of one workspace has one analysis in both, over the
// same text and the same resolution.
//
// scoped records that a document may declare an identity scope: whether the
// model's ids are qualified depends on how many scopes all of its documents
// declare (identity.ProjectRefFQN), so an edit to one can change what every
// document converts to without changing its analysis.
type analysisSnapshot struct {
	workspace *model.Workspace
	stamps    map[string]uint64
	scoped    bool
}

// declaresIdentityScope reports whether a document may declare an identity
// scope: one does so only through the ProjectRef metadata, which a document
// can apply, or alias, only by writing its name.
func declaresIdentityScope(inputs []sourceInput) bool {
	_, name := splitLastSegment(identity.ProjectRefFQN)
	for _, input := range inputs {
		if strings.Contains(input.content, name) {
			return true
		}
	}
	return false
}

// splitLastSegment splits a qualified name at its last `::`.
func splitLastSegment(fqn string) (string, string) {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[:i], fqn[i+2:]
	}
	return "", fqn
}

// affectedDocuments names, in model's document order, the documents whose
// analysis may differ between base and model: every one, unless both were
// answered from one lineage's workspace and neither may declare an identity
// scope, and then those whose stamp differs, is zero, or base did not hold.
func affectedDocuments(base, model *CachedModel) []string {
	names := make([]string, len(model.Documents))
	for i, doc := range model.Documents {
		names[i] = doc.Source.Name()
	}
	if base == nil || base.analysis == nil || model.analysis == nil || base.analysis.workspace != model.analysis.workspace ||
		base.analysis.scoped || model.analysis.scoped {
		return names
	}
	affected := make([]string, 0, len(names))
	for _, name := range names {
		was, held := base.analysis.stamps[name]
		now := model.analysis.stamps[name]
		if !held || was == 0 || now == 0 || was != now {
			affected = append(affected, name)
		}
	}
	return affected
}
