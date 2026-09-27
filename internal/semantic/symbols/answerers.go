package symbols

import "sort"

// Answerers names the documents whose declarations or imports take part in
// the index's answer about fqn: those declaring a symbol under it and those
// whose wildcard imports surface one there, sorted.
func (idx *Index) Answerers(fqn string) []string {
	docs := map[string]bool{}
	idx.answerersOf(fqn, docs)
	return sortedDocs(docs)
}

// NamespaceAnswerers is Answerers over the direct members of the namespace
// named prefix: the documents that declare or surface one, sorted.
func (idx *Index) NamespaceAnswerers(prefix string) []string {
	docs := map[string]bool{}
	for _, fqn := range idx.children.at(prefix) {
		idx.answerersOf(fqn, docs)
	}
	return sortedDocs(docs)
}

// SegmentAnswerers names the documents whose declarations decide
// ShortNamed(name): those declaring a symbol registered under name that is
// not its own last segment, sorted.
func (idx *Index) SegmentAnswerers(name string) []string {
	docs := map[string]bool{}
	short := func(fqn string) {
		for _, sym := range idx.fqn.at(fqn) {
			if LastSegment(sym.Name) != name {
				docs[sym.DocName] = true
			}
		}
	}
	for _, fqn := range idx.segmentNames(name) {
		short(fqn)
	}
	short(name)
	return sortedDocs(docs)
}

func (idx *Index) answerersOf(fqn string, docs map[string]bool) {
	for _, sym := range idx.fqn.at(fqn) {
		docs[sym.DocName] = true
		for doc := range idx.reexportDocs.at(reexportKey{fqn: fqn, sym: sym}) {
			docs[doc] = true
		}
	}
}

func sortedDocs(docs map[string]bool) []string {
	if len(docs) == 0 {
		return nil
	}
	out := make([]string, 0, len(docs))
	for doc := range docs {
		out = append(out, doc)
	}
	sort.Strings(out)
	return out
}
