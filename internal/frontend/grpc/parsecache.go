package grpc

import (
	"context"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	wsmodel "github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// maxRecordedParseTreeBytes caps the AST stream read back from a parse record;
// a record larger than this is a miss rather than a decode.
const maxRecordedParseTreeBytes = 64 << 20

// astBytesPerNode is the minimum density a decoded stream is allowed: the
// densest real trees measured 14.6 bytes per node across the examples.
const astBytesPerNode = 8

// maxDecodedNodes caps the absolute allocation a stream can demand: the
// largest real tree measured 3905 nodes, so this leaves a wide margin.
const maxDecodedNodes = 1 << 18

// minDecodeNodes keeps the density bound from refusing a small tree whose
// stream is shorter than one node's worth of denominator.
const minDecodeNodes = 1024

// parsedSource is what parsing one document yields; parseSource is the service's
// single seam to the parser, so a test can count parses.
type parsedSource struct {
	root        *ast.RootNamespace
	diagnostics []parser.Diagnostic
	warnings    []parser.Diagnostic
}

func parseSourceFile(sf *source.SourceFile) parsedSource {
	p := parser.New(sf)
	return parsedSource{root: p.ParseFile(), diagnostics: p.Diagnostics, warnings: p.Warnings}
}

// encodeParseTree writes one tree as an astcodec stream and returns its root's
// table id, so a decode can find the root among the nodes grouped by type.
func encodeParseTree(root *ast.RootNamespace) ([]byte, uint64, error) {
	w := pack.NewWriter()
	e := astcodec.NewEncoder(w)
	if err := e.Encode([]ast.Node{root}); err != nil {
		return nil, 0, err
	}
	id, ok := e.ID(root)
	if !ok || id == 0 {
		return nil, 0, fmt.Errorf("the encoded tree has no root node")
	}
	return w.Bytes(), id, nil
}

// decodeParseTree is encodeParseTree's inverse. Any malformation is an error,
// which the caller treats as a miss and answers by parsing the source afresh.
func decodeParseTree(data []byte, rootID uint64) (*ast.RootNamespace, error) {
	if len(data) == 0 || len(data) > maxRecordedParseTreeBytes || rootID == 0 {
		return nil, fmt.Errorf("parse record holds no usable tree")
	}
	r, err := pack.NewReader(data)
	if err != nil {
		return nil, err
	}
	// A decoded node writes at least a span and trivia counts; real trees use
	// far more than astBytesPerNode bytes each, so a denser claim is hostile.
	limit := min(max(len(data)/astBytesPerNode, minDecodeNodes), maxDecodedNodes)
	d := astcodec.NewDecoder(r)
	d.LimitNodes(limit)
	d.Decode()
	if err := d.Err(); err != nil {
		return nil, err
	}
	n, ok := d.Node(rootID)
	if !ok || n == nil {
		return nil, fmt.Errorf("parse record's root id is not in its node table")
	}
	root, ok := n.(*ast.RootNamespace)
	if !ok {
		return nil, fmt.Errorf("parse record's root is a %T, not a root namespace", n)
	}
	return root, nil
}

// parseRecordKey is the key of the parse record holding input's tree. Without a
// record cache there is no key and nothing to restore or store.
func (s *Service) parseRecordKey(input sourceInput) (string, bool) {
	if s.records == nil {
		return "", false
	}
	return s.records.ParseKey(input.name, input.language, input.kind, []byte(input.content)), true
}

// loadParsedDocument rebuilds input's document from the record another process
// stored for these bytes; the tree is decoded, not parsed.
func (s *Service) loadParsedDocument(input sourceInput) (*CachedDocument, bool) {
	key, ok := s.parseRecordKey(input)
	if !ok {
		return nil, false
	}
	rec, ok := s.records.LoadParse(key)
	if !ok || rec == nil {
		return nil, false
	}
	// The key names these fields, so a record that disagrees with them is a
	// damaged record rather than one of this document.
	if rec.Name != input.name || rec.Kind != input.kind || rec.Language != input.language {
		return nil, false
	}
	root, err := decodeParseTree(rec.AST, rec.RootID)
	if err != nil {
		return nil, false
	}
	sf := source.NewWithKind(input.name, []byte(input.content), input.kind)
	doc := newCachedDocument(root, sf, rec.ParseDiags,
		parser.AsDiagnostics(rec.ParseDiags, rec.ParseWarnings), input.warnings)
	doc.parseWarnings = rec.ParseWarnings
	return doc, true
}

// storeParsedDocument files doc's tree under input's content for the next process;
// a record that cannot be written is a benign miss.
func (s *Service) storeParsedDocument(input sourceInput, doc *CachedDocument) {
	key, ok := s.parseRecordKey(input)
	if !ok {
		return
	}
	tree, rootID, err := encodeParseTree(doc.Root)
	if err != nil {
		return
	}
	rec := &libs.ParseRecord{
		Name: input.name, Kind: input.kind, Language: input.language,
		AST: tree, RootID: rootID,
		ParseDiags: doc.ParseDiags, ParseWarnings: doc.parseWarnings, Warnings: doc.Warnings,
	}
	_ = s.records.StoreParse(key, rec)
}

// parseDocument reads input's tree from the cache or parses and records it;
// restored reports which, so a test can prove a model was not parsed.
func (s *Service) parseDocument(input sourceInput) (*CachedDocument, bool) {
	if doc, ok := s.loadParsedDocument(input); ok {
		return doc, true
	}
	sf := source.NewWithKind(input.name, []byte(input.content), input.kind)
	parse := s.parseSource
	if parse == nil {
		parse = parseSourceFile
	}
	parsed := parse(sf)
	doc := newCachedDocument(parsed.root, sf, parsed.diagnostics,
		parser.AsDiagnostics(parsed.diagnostics, parsed.warnings), input.warnings)
	doc.parseWarnings = parsed.warnings
	s.storeParsedDocument(input, doc)
	return doc, false
}

// restoreFromDisk assembles a model from another process's records: trees are
// decoded, the index rebuilt, and the analysis read, with no parse.
func (s *Service) restoreFromDisk(ctx context.Context, inputs []sourceInput, mode diag.ConformanceMode, modelHash string) (*CachedModel, bool, error) {
	if s.records == nil {
		return nil, false, nil
	}
	documents := make([]*CachedDocument, len(inputs))
	clean := true
	for i, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		doc, ok := s.loadParsedDocument(input)
		if !ok {
			return nil, false, nil
		}
		documents[i] = doc
		if len(doc.ParseDiags) > 0 {
			clean = false
		}
	}

	// The index is derived data: rebuilding it over the restored trees gives
	// exactly the index a fresh parse builds, without re-reading source text.
	idx, library := s.libIndexes.get()
	for i, input := range inputs {
		idx.AddDocumentWithKind(input.name, documents[i].Root, input.kind)
	}
	idx.ExpandWildcardImports()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	var analysis *analysisSnapshot
	if clean {
		// An unclean set publishes no semantic diagnostics: stamps on it would
		// hide its siblings from a later edit's affected set (lineage.go).
		reads, ok := s.analysisFromRecord(modelHash, idx, documents)
		var pendingRecord func()
		if !ok {
			names := make([]string, len(inputs))
			for i, input := range inputs {
				names[i] = input.name
			}
			batch := &passes.Batch{Documents: names, Gathers: passes.NewGathers(), Record: s.records != nil}
			passes.PrepareBatch(idx, batch)
			reads = make(map[string]*resolve.Reads, len(documents))
			for i, doc := range documents {
				if err := ctx.Err(); err != nil {
					return nil, false, err
				}
				var runReads *resolve.Reads
				doc.Unfiltered, runReads = passes.AnalyzeInBatch(inputs[i].name, inputs[i].kind, doc.Root,
					doc.Diagnostics, idx, passes.Options{Conformance: mode, Context: ctx}, batch)
				reads[inputs[i].name] = runReads
			}
			if err := ctx.Err(); err != nil {
				return nil, false, err
			}
			// Record only once the lineage is primed: a cancelled prime writes
			// no partial analysis, as on the fresh path.
			pendingRecord = func() { s.recordAnalysis(modelHash, idx, documents, reads) }
		}
		for _, doc := range documents {
			doc.Diagnostics = passes.WithoutLints(doc.Unfiltered, nil, nil)
		}
		analysis = s.restoredAnalysis(ctx, inputs, mode, documents, reads)
		if analysis == nil && ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		if pendingRecord != nil {
			pendingRecord()
		}
	}
	// A document that did not parse clean is never analyzed, exactly as on the
	// fresh path; its diagnostics remain the parse's.

	model := &CachedModel{
		Documents: documents, Index: idx, Library: library, Mode: mode,
		Findings: checkws.DuplicateLibraries(idx), analysis: analysis,
	}
	return model, true, nil
}

// restoredAnalysis primes the lineage workspace from the analysis record's
// diagnostics and reads, but never stamps a set a library stand-in sends fresh.
func (s *Service) restoredAnalysis(ctx context.Context, inputs []sourceInput, mode diag.ConformanceMode, documents []*CachedDocument, reads map[string]*resolve.Reads) *analysisSnapshot {
	key := lineageKey(inputs, mode)
	l, _ := s.lineages.get(key, func() *lineage { return &lineage{held: map[string]string{}} })
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil
	}
	if l.fresh {
		// parseFromLineage already gave up this workspace and its held map.
		return nil
	}
	if l.ws == nil {
		idx, library := s.libIndexes.get()
		l.ws = wsmodel.NewWorkspaceWithIndex(idx, wsmodel.WithConformanceMode(mode), wsmodel.WithLibrarySource(library))
		l.library = library
	}
	if l.held == nil {
		l.held = map[string]string{}
	}
	standIns := map[string]string{}
	for i, input := range inputs {
		doc := documents[i]
		l.ws.OpenParsed(input.name, []byte(input.content), l.versions, input.kind,
			doc.Root, doc.ParseDiags, doc.parseWarnings)
		l.held[input.name] = input.content
		if l.ws.StandsIn(input.name) {
			standIns[input.name] = input.content
		}
	}
	if len(standIns) > 0 {
		l.fresh, l.standIns, l.ws, l.held, l.answered = true, standIns, nil, nil, nil
		return nil
	}
	// Rebuild the shared facts the recorded verdicts read, so a later edit can
	// tell a reader of a moving union from one that did not read it.
	facts := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if rd := reads[input.name]; rd != nil {
			facts = append(facts, rd.Names...)
		}
	}
	l.ws.RestoreSharedFacts(facts)
	stamps := make(map[string]uint64, len(inputs))
	for i, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if !l.ws.RestoreAnalyzed(input.name, documents[i].Unfiltered, reads[input.name]) {
			_ = l.ws.Diagnostics(input.name)
		}
		stamps[input.name] = l.ws.AnalysisStamp(input.name)
	}
	if err := ctx.Err(); err != nil {
		return nil
	}
	return &analysisSnapshot{workspace: l.ws, stamps: stamps, scoped: declaresIdentityScope(inputs)}
}
