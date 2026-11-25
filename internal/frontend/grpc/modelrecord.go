package grpc

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A model's analysis is recorded on disk under everything it reads, so the next process to
// parse the same model reads its diagnostics instead of running the passes; any miss analyzes.

// modelRecordKey is the record cache's key for the model hashed modelHash over idx;
// there is none without a cache or while the index's library states no identity.
func (s *Service) modelRecordKey(modelHash string, idx *symbols.Index) (string, bool) {
	if s.records == nil {
		return "", false
	}
	library, ok := idx.LibraryIdentity()
	if !ok {
		return "", false
	}
	return s.records.ModelKey(modelHash, library), true
}

// analysisFromRecord gives documents the record's diagnostics and reads, and
// reports whether a record of exactly these documents was found.
func (s *Service) analysisFromRecord(modelHash string, idx *symbols.Index, documents []*CachedDocument) (map[string]*resolve.Reads, bool) {
	key, ok := s.modelRecordKey(modelHash, idx)
	if !ok {
		return nil, false
	}
	rec, ok := s.records.LoadModel(key)
	if !ok || len(rec.Documents) != len(documents) {
		return nil, false
	}
	for i, doc := range documents {
		if rec.Documents[i].Name != doc.Source.Name() || rec.Documents[i].Kind != doc.Source.Kind() ||
			rec.Documents[i].Reads == nil {
			return nil, false
		}
	}
	reads := make(map[string]*resolve.Reads, len(documents))
	for i, doc := range documents {
		doc.Unfiltered = rec.Documents[i].Diagnostics
		reads[doc.Source.Name()] = rec.Documents[i].Reads
	}
	return reads, true
}

// recordAnalysis writes the documents' analysis and reads to the model's record.
// A cache that cannot be written is no cache: the next process analyzes.
func (s *Service) recordAnalysis(modelHash string, idx *symbols.Index, documents []*CachedDocument, reads map[string]*resolve.Reads) {
	key, ok := s.modelRecordKey(modelHash, idx)
	if !ok {
		return
	}
	rec := &libs.ModelRecord{Documents: make([]libs.ModelDocument, len(documents))}
	for i, doc := range documents {
		rec.Documents[i] = libs.ModelDocument{
			Name: doc.Source.Name(), Kind: doc.Source.Kind(),
			Diagnostics: doc.Unfiltered, Reads: reads[doc.Source.Name()],
		}
	}
	_ = s.records.StoreModel(key, rec)
}
