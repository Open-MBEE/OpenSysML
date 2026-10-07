package libs

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Provenance is where the analysis an interface record was written from got
// its answers: what it read of the index (Reads) and, per read, the workspace
// documents that took part in the answer, with their content digests. A record
// holds only where those documents are as they were; anywhere else its
// diagnostics and evaluated facts may not follow from the documents around it,
// and the document is parsed instead (see Valid). Library documents are left
// out of it: the record's key already names the library as a whole.
type Provenance struct {
	Reads resolve.Reads
	// Answerers maps each read that a workspace document took part in to those
	// documents, sorted; reads answered by the library alone are absent.
	Answerers map[string][]string
	// Digests are of every document named in Answerers.
	Digests map[string]string
}

// Sources is what a Provenance is attributed against or checked against: the
// index, the shared state its readers consulted, and the digest of each
// workspace document.
type Sources struct {
	Index *symbols.Index
	// Contributors names the workspace documents the shared state called name
	// is built from, false when no shared state answers for name (see
	// passes.Contributors).
	Contributors func(name string) ([]string, bool)
	// Digest reports the content digest of a workspace document, false when
	// the workspace holds none of the name.
	Digest func(doc string) (string, bool)
	memo   *sourcesMemo
}

// sourcesMemo keeps the index's answers about a read across the records one
// Sources checks, so the segment a hundred records read is scanned once.
type sourcesMemo struct {
	namespaces map[string][]string
	segments   map[string][]string
}

// NewSources is a Sources over index, contributors and digest as they stand
// now, memoizing the index's answers: use one for every record checked or
// attributed against the same state, and a fresh one once the index changes.
func NewSources(index *symbols.Index, contributors func(string) ([]string, bool), digest func(string) (string, bool)) Sources {
	return Sources{
		Index:        index,
		Contributors: contributors,
		Digest:       digest,
		memo:         &sourcesMemo{namespaces: map[string][]string{}, segments: map[string][]string{}},
	}
}

func (s Sources) namespaceAnswerers(ns string) []string {
	if s.memo == nil {
		return s.Index.NamespaceAnswerers(ns)
	}
	docs, ok := s.memo.namespaces[ns]
	if !ok {
		docs = s.Index.NamespaceAnswerers(ns)
		s.memo.namespaces[ns] = docs
	}
	return docs
}

func (s Sources) segmentAnswerers(seg string) []string {
	if s.memo == nil {
		return s.Index.SegmentAnswerers(seg)
	}
	docs, ok := s.memo.segments[seg]
	if !ok {
		docs = s.Index.SegmentAnswerers(seg)
		s.memo.segments[seg] = docs
	}
	return docs
}

// Attribute records where the reads were answered from, as src stands now,
// which is as it stood for the analysis that made them. Documents src has no
// digest for — the library's — are not attributed: the record's key names the
// library as a whole. It fails when a read names shared state nothing of src's
// answers for: such a record could not tell where it holds.
func Attribute(reads resolve.Reads, src Sources) (*Provenance, error) {
	p := &Provenance{Reads: reads.Clone(), Answerers: map[string][]string{}, Digests: map[string]string{}}
	err := p.each(src, func(key string, docs []string) bool {
		if len(docs) > 0 {
			p.Answerers[key] = docs
		}
		for _, doc := range docs {
			p.Digests[doc], _ = src.Digest(doc)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Clone is a deep copy of p, sharing nothing with it; nil clones to nil.
func (p *Provenance) Clone() *Provenance {
	if p == nil {
		return nil
	}
	out := &Provenance{Reads: p.Reads.Clone()}
	if p.Answerers != nil {
		out.Answerers = make(map[string][]string, len(p.Answerers))
		for key, docs := range p.Answerers {
			out.Answerers[key] = slices.Clone(docs)
		}
	}
	if p.Digests != nil {
		out.Digests = maps.Clone(p.Digests)
	}
	return out
}

// Valid reports whether src answers every read as the analysis saw it: the
// same documents take part in each answer, and each has the content it had. A
// nil provenance holds nowhere.
func (p *Provenance) Valid(src Sources) bool {
	if p == nil {
		return false
	}
	err := p.each(src, func(key string, docs []string) bool {
		if !equalDocs(p.Answerers[key], docs) {
			return false
		}
		for _, doc := range docs {
			if digest, _ := src.Digest(doc); digest != p.Digests[doc] {
				return false
			}
		}
		return true
	})
	return err == nil
}

// each visits every read of p with the documents src holds a digest of that
// take part in its answer, until visit returns false, which it reports as the
// read that did not hold.
func (p *Provenance) each(src Sources, visit func(key string, docs []string) bool) error {
	held := func(docs []string) []string {
		out := docs[:0:0]
		for _, doc := range docs {
			if _, ok := src.Digest(doc); ok {
				out = append(out, doc)
			}
		}
		sort.Strings(out)
		return out
	}
	see := func(key string, docs []string) bool { return visit(key, held(docs)) }
	fail := func(what, name string) error {
		return fmt.Errorf("%w: %s %q", ErrUnrecordable, what, strings.ReplaceAll(name, "\x00", "@"))
	}
	if p.Reads.All {
		if !see("all", src.Index.WorkspaceDocuments()) {
			return fail("the name table read as a whole", "")
		}
	}
	// The names spelled are every document's to answer: a record whose
	// suggestions read them holds while no document changed.
	if p.Reads.Spellings {
		if !see("spellings", src.Index.WorkspaceDocuments()) {
			return fail("the names spelled", "")
		}
	}
	for _, name := range p.Reads.Names {
		docs, ok := sharedContributors(name, src)
		if !ok {
			if strings.HasPrefix(name, "\x00") {
				return fail("no shared state answers", name)
			}
			docs = src.Index.Answerers(name)
		}
		if !see("n:"+name, docs) {
			return fail("the answer about", name)
		}
	}
	for _, ns := range p.Reads.Namespaces {
		if !see("s:"+ns, src.namespaceAnswerers(ns)) {
			return fail("the members of", ns)
		}
	}
	for _, seg := range p.Reads.Segments {
		if !see("g:"+seg, src.segmentAnswerers(seg)) {
			return fail("the short names under", seg)
		}
	}
	for _, doc := range p.Reads.Docs {
		docs := []string{doc}
		if strings.HasPrefix(doc, "\x00") {
			contributors, ok := sharedContributors(doc, src)
			if !ok {
				return fail("no shared state answers", doc)
			}
			docs = contributors
		}
		if !see("d:"+doc, docs) {
			return fail("the document", doc)
		}
	}
	return nil
}

// sharedContributors names the documents the shared state called name is
// built from, false when name is no shared state's.
func sharedContributors(name string, src Sources) ([]string, bool) {
	if !strings.HasPrefix(name, "\x00") {
		return nil, false
	}
	return src.Contributors(name)
}

func equalDocs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
