package flexo

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// The findings are the report's prose: each one names a mechanism in the service
// that a measured number is explained by, so a reader adjudicating a change to
// the expectation file does not have to re-derive it from the Kotlin.

// findings turns the measurements into named conclusions, in a fixed order.
func findings(report *Report, graphWritten map[string]*writtenElement) []string {
	var found []string
	add := func(format string, args ...any) {
		found = append(found, fmt.Sprintf(format, args...))
	}

	load, reference, apiJSON := &report.Load, &report.Reference, &report.APIJSON

	if !load.Accepted {
		add("graph-load: Layer 1 refused the Turtle, so nothing below was measured")
		return found
	}

	written, delivered := load.propertyTotals()
	add("graph-load: %d of %d elements listed, %d of %d properties delivered",
		load.Listed, load.Written, delivered, written)
	referenceWritten, referenceDelivered := reference.propertyTotals()
	add("json-commit: %d of %d elements listed, %d of %d properties delivered",
		reference.Listed, reference.Written, referenceDelivered, referenceWritten)
	apiWritten, apiDelivered := apiJSON.propertyTotals()
	add("api-json-commit: %d of %d elements listed, %d of %d properties delivered",
		apiJSON.Listed, apiJSON.Written, apiDelivered, apiWritten)
	if !apiJSON.Accepted {
		add("api-json-commit: the commit was refused (%s), so nothing below was measured on it",
			apiJSON.Refusal)
	}

	if count := load.propertyCount(func(p PropertyStat) bool {
		return strings.HasPrefix(p.Property, "sysx:")
	}); count > 0 {
		add("graph-load: %d properties are in the sysx: extension namespace and are dropped "+
			"unread — extractModelElementToJson ignores every predicate outside the sysml: "+
			"vocabulary and the JSON annotation namespace (%s)",
			count, strings.Join(load.propertyNames("sysx:"), ", "))
	}

	if _, ok := load.property("sysml:elementId"); !ok {
		add("graph-load: no element carries sysml:elementId, which the service's own paged " +
			"listing query selects on; the deployed listing route ignores that query and " +
			"returns every subject of the branch graph instead, so the loaded elements are " +
			"visible today only because paging is unimplemented")
	}

	if multi, delivered := load.multiValued(); multi > 0 {
		add("graph-load: %d of %d multi-valued properties are delivered (%s); a standard "+
			"predicate with several values is skipped entirely unless a JSON annotation "+
			"supplies the array",
			delivered, multi, strings.Join(load.multiValuedNames(), ", "))
	}

	if refused := load.refusedIDs(); len(refused) > 0 {
		add("graph-load: %d elements cannot be read directly because their ids are not "+
			"[a-zA-Z0-9_-]+, which requireValidId demands (%s)",
			len(refused), strings.Join(refused, ", "))
	}

	add("graph-load: the roots endpoint reports %d of %d elements as roots (%d have no owner "+
		"in the model); it filters on sysml:owner and sysml:owningRelatedElement",
		load.Roots, load.Written, load.RootsInModel)
	add("json-commit: the roots endpoint reports %d of %d elements as roots, %d in the payload",
		reference.Roots, reference.Written, reference.RootsInModel)
	add("api-json-commit: the roots endpoint reports %d of %d elements as roots, %d in the payload",
		apiJSON.Roots, apiJSON.Written, apiJSON.RootsInModel)

	if load.IgnoredPaging {
		add("both: the element listing answered every element it has in %d response(s) at "+
			"pageSize=%d — the service accepts pageAfter/pageBefore/pageSize and ignores them",
			load.Pages, PageSize)
	}

	if shapes := load.shapes(); len(shapes) > 0 {
		add("graph-load: %d properties came back in a different shape than they were written "+
			"(%s)", len(shapes), strings.Join(shapes, ", "))
	}
	if shapes := reference.shapes(); len(shapes) > 0 {
		add("json-commit: %d properties came back in a different shape than they were posted "+
			"(%s)", len(shapes), strings.Join(shapes, ", "))
	}
	if shapes := apiJSON.shapes(); len(shapes) > 0 {
		add("api-json-commit: %d properties came back in a different shape than they were posted "+
			"(%s)", len(shapes), strings.Join(shapes, ", "))
	}

	if multi, delivered := reference.multiValued(); multi > 0 {
		add("json-commit: %d of %d multi-valued properties are delivered (%s), because the "+
			"commit path stores each array whole as a JSON annotation literal alongside the "+
			"typed triples",
			delivered, multi, strings.Join(reference.multiValuedNames(), ", "))
	}
	if multi, delivered := apiJSON.multiValued(); multi > 0 {
		add("api-json-commit: %d of %d multi-valued properties are delivered (%s)",
			delivered, multi, strings.Join(apiJSON.multiValuedNames(), ", "))
	}

	if refused := apiJSON.refusedIDs(); len(refused) > 0 {
		add("api-json-commit: %d elements cannot be read directly because their ids are not "+
			"[a-zA-Z0-9_-]+, which requireValidId demands (%s)",
			len(refused), strings.Join(refused, ", "))
	}

	if apiJSON.Accepted {
		apiOnly, loadOnly := deliveredOnly(apiJSON, load)
		add("api-json-commit vs graph-load: %d properties delivered on api-json only (%s), "+
			"%d on graph-load only (%s)",
			len(apiOnly), strings.Join(apiOnly, ", "), len(loadOnly), strings.Join(loadOnly, ", "))
		if posted, delivered := apiJSON.sysxTotals(); posted > 0 {
			add("api-json-commit: %d sysx: properties posted, %d delivered (%s)",
				posted, delivered, strings.Join(apiJSON.propertyNames("sysx:"), ", "))
		}
	}

	add("graph-load: %d of %d subjects of the graph are expression nodes or other subjects "+
		"outside the element namespace", expressionNodes(graphWritten), len(graphWritten))
	return found
}

// deliveredOnly names the delivered properties of a that b did not deliver,
// and vice versa, normalized to the bare key both sides deliver them under.
func deliveredOnly(a, b *SideReport) ([]string, []string) {
	delivered := func(s *SideReport) map[string]bool {
		names := map[string]bool{}
		for _, p := range s.Properties {
			if p.Delivered > 0 {
				names[localName(p.Property)] = true
			}
		}
		return names
	}
	aDelivered, bDelivered := delivered(a), delivered(b)
	var aOnly, bOnly []string
	for name := range aDelivered {
		if !bDelivered[name] {
			aOnly = append(aOnly, name)
		}
	}
	for name := range bDelivered {
		if !aDelivered[name] {
			bOnly = append(bOnly, name)
		}
	}
	sort.Strings(aOnly)
	sort.Strings(bOnly)
	return aOnly, bOnly
}

// sysxTotals sums the posted and delivered counts of a side's sysx: properties.
func (s *SideReport) sysxTotals() (int, int) {
	posted, delivered := 0, 0
	for _, p := range s.Properties {
		if strings.HasPrefix(p.Property, "sysx:") {
			posted += p.Written
			delivered += p.Delivered
		}
	}
	return posted, delivered
}

// propertyTotals sums the per-property written and delivered counts of a side.
func (s *SideReport) propertyTotals() (int, int) {
	written, delivered := 0, 0
	for _, p := range s.Properties {
		written += p.Written
		delivered += p.Delivered
	}
	return written, delivered
}

// property returns one property's stats.
func (s *SideReport) property(name string) (PropertyStat, bool) {
	for _, p := range s.Properties {
		if p.Property == name {
			return p, true
		}
	}
	return PropertyStat{}, false
}

// propertyCount counts the properties matching a predicate.
func (s *SideReport) propertyCount(match func(PropertyStat) bool) int {
	count := 0
	for _, p := range s.Properties {
		if match(p) {
			count++
		}
	}
	return count
}

// multiValued sums the elements that wrote several values for some property, and
// how many of those the read path returned it for.
func (s *SideReport) multiValued() (int, int) {
	written, delivered := 0, 0
	for _, p := range s.Properties {
		written += p.MultiValued
		delivered += p.MultiDelivered
	}
	return written, delivered
}

// propertyNames lists the properties with a given prefix, sorted.
func (s *SideReport) propertyNames(prefix string) []string {
	var names []string
	for _, p := range s.Properties {
		if strings.HasPrefix(p.Property, prefix) {
			names = append(names, p.Property)
		}
	}
	sort.Strings(names)
	return names
}

// multiValuedNames lists the properties some element carries several values for,
// with how many of those elements returned them.
func (s *SideReport) multiValuedNames() []string {
	var names []string
	for _, p := range s.Properties {
		if p.MultiValued > 0 {
			names = append(names, fmt.Sprintf("%s on %d/%d", p.Property, p.MultiDelivered, p.MultiValued))
		}
	}
	sort.Strings(names)
	return names
}

// refusedIDs lists the elements a direct read refused, sorted.
func (s *SideReport) refusedIDs() []string {
	var ids []string
	for _, e := range s.Elements {
		if e.Direct != "ok" {
			ids = append(ids, e.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// shapes lists the distinct written-to-delivered shape changes of a side.
func (s *SideReport) shapes() []string {
	seen := map[string]int{}
	for _, e := range s.Elements {
		for _, shape := range e.Shape {
			seen[shape]++
		}
	}
	var shapes []string
	for _, shape := range slices.Sorted(maps.Keys(seen)) {
		shapes = append(shapes, fmt.Sprintf("%s on %d", shape, seen[shape]))
	}
	return shapes
}

// expressionNodes counts the subjects whose id is not a valid element id, which
// is what the encoder writes for expression graph nodes.
func expressionNodes(written map[string]*writtenElement) int {
	count := 0
	for id := range written {
		if !validElementID(id) {
			count++
		}
	}
	return count
}

// validElementID mirrors the service's requireValidId: ids outside
// [a-zA-Z0-9_-]+ are refused before any query runs.
func validElementID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
