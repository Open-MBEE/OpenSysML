package symbols

import "testing"

func TestShortNamedIncludesExplicitSameNameAndShortOnly(t *testing.T) {
	docs := map[string]string{"names.sysml": "attribute <rootCode>; package P { attribute <same> same; attribute <onlyCode>; }"}
	for _, idx := range []*Index{frozenBase(t, docs), roundTrip(t, frozenBase(t, docs))} {
		for _, name := range []string{"rootCode", "same", "onlyCode"} {
			if !idx.ShortNamed(name) {
				t.Errorf("ShortNamed(%q) = false", name)
			}
			docs := idx.SegmentAnswerers(name)
			if len(docs) != 1 || docs[0] != "names.sysml" {
				t.Errorf("answerers of %q: %v", name, docs)
			}
		}
	}
}
