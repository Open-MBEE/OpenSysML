package resolve

import (
	"maps"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Reads is what a document's analysis read of the index: its frame's read set
// (see frame) closed over the frames it entered, since an entry it used may
// have been computed in one of them. A record carries it so a workspace can
// tell whether an index still answers those reads as the analysis saw them.
type Reads struct {
	// Names, Namespaces and Segments are what the index answered about, sorted.
	Names      []string
	Namespaces []string
	Segments   []string
	// All is set once a frame enumerated the whole name table.
	All bool
	// Docs are the documents whose roots or kinds were read or whose frames
	// were entered, sorted; the document itself is not among them.
	Docs []string
}

// ReadsOf reports the read set of doc's analysis: its frame's, with those of
// every frame reachable through its dependencies. ok is false when the resolver
// does not track or owns nothing for doc.
func (r *Resolver) ReadsOf(doc string) (reads Reads, ok bool) {
	if !r.Tracking() {
		return Reads{}, false
	}
	root := r.owners[doc]
	if root == nil {
		return Reads{}, false
	}
	names, namespaces, segments, docs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	seen := map[string]bool{doc: true}
	work := []*frame{root}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		maps.Copy(names, f.names)
		maps.Copy(namespaces, f.namespaces)
		maps.Copy(segments, f.segments)
		reads.All = reads.All || f.all
		for d := range f.docs {
			docs[d] = true
		}
		for d := range f.deps {
			docs[d] = true
			if !seen[d] {
				seen[d] = true
				if g := r.owners[d]; g != nil {
					work = append(work, g)
				}
			}
		}
	}
	delete(docs, doc)
	reads.Names = slices.Sorted(maps.Keys(names))
	reads.Namespaces = slices.Sorted(maps.Keys(namespaces))
	reads.Segments = slices.Sorted(maps.Keys(segments))
	reads.Docs = slices.Sorted(maps.Keys(docs))
	return reads, true
}

// Stale reports whether ch moved anything the reads saw: what frame.stale
// asks of a live frame, asked of a recorded one.
func (rd Reads) Stale(ch symbols.Changes) bool {
	if rd.All && ch.Registered() {
		return true
	}
	for _, n := range rd.Docs {
		if ch.Docs[n] {
			return true
		}
	}
	for _, n := range rd.Names {
		if ch.Names[n] {
			return true
		}
	}
	for _, n := range rd.Namespaces {
		if ch.Namespaces[n] {
			return true
		}
	}
	if len(rd.Segments) > 0 {
		for n := range ch.Names {
			if _, found := slices.BinarySearch(rd.Segments, symbols.LastSegment(n)); found {
				return true
			}
		}
	}
	return false
}

// Clone returns reads sharing nothing with rd.
func (rd Reads) Clone() Reads {
	return Reads{
		Names:      slices.Clone(rd.Names),
		Namespaces: slices.Clone(rd.Namespaces),
		Segments:   slices.Clone(rd.Segments),
		All:        rd.All,
		Docs:       slices.Clone(rd.Docs),
	}
}

// NewRecording is a tracking resolver over a view of idx whose reads report to
// it alone: for an analysis run beside others over one index, whose reads the
// record it writes carries (see ReadsOf).
func NewRecording(idx *symbols.Index) *Resolver {
	r := New(nil)
	r.idx = idx.Recording(r)
	r.Track()
	return r
}
