package migrate

import "github.com/Open-MBEE/OpenSysML/internal/core/xmi"

// populatedRegions drops the regions with no vertex: a state written for one
// would be a region nothing enters, which the runtime refuses to lower.
func (m *migration) populatedRegions(regions []*xmi.Element) []*xmi.Element {
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
