package migrate

import "github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"

// populatedRegions drops the regions with no vertex: a state written for one
// would be a region nothing enters, which the runtime refuses to lower.
func (m *migration) populatedRegions(regions []*sysmlv1.Element) []*sysmlv1.Element {
	kept := regions[:0:0]
	for _, r := range regions {
		if len(r.Owned("subvertex")) == 0 {
			m.add(r, Approximated, "", "the region has no vertex and is not written: nothing would enter it")
			continue
		}
		kept = append(kept, r)
	}
	return kept
}
