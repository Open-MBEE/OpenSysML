package flexo

import (
	"fmt"
	"sort"
	"strings"
)

// The report is the deliverable of a run: a per-element, per-property inventory
// of what survived a round trip through the stack, in a form a human adjudicates
// as a diff. It records nothing a second run on another machine would render
// differently — no commit ids, no timestamps, no wall-clock, no map order.

// PropertyStat is one predicate's fate across a whole side of the measurement:
// how many elements carried it, and on how many it came back.
type PropertyStat struct {
	Property       string // "sysml:declaredName" written, "declaredName" delivered
	Written        int    // elements that carried it
	Delivered      int    // elements that returned it
	MultiValued    int    // elements that carried more than one value for it
	MultiDelivered int    // of those, the elements that returned it
}

// ElementStat is one element's fate: whether the paged listing saw it, whether
// it can be read directly, and which of its properties came back.
type ElementStat struct {
	ID        string
	Type      string   // metaclass as written
	Listed    bool     // the paged element listing returned it
	Direct    string   // "ok", or "rejected(400)" / "absent(404)" for a direct read
	Written   int      // properties written for it, rdf:type excluded
	Delivered int      // properties returned for it, @type excluded
	Lost      []string // written properties the read path did not return
	Shape     []string // "sysml:type: reference written, string delivered"
}

// SideReport is one direction of the comparison: the graph this project loads,
// or the same model committed through the service's own JSON path.
type SideReport struct {
	Name          string // "graph-load" or "json-commit"
	Accepted      bool   // the service took the payload
	Commits       int    // commits the project has after the write
	Written       int    // elements in the payload
	Listed        int    // written elements the listing returned
	Extra         int    // elements the listing returned that were not written
	Pages         int    // responses the listing took, at the harness page size
	IgnoredPaging bool   // the listing repeated or overran what the page size asked for
	Direct        int    // elements a direct read returned
	Roots         int    // elements the roots endpoint returned
	RootsInModel  int    // elements the payload itself gives no owner
	Elements      []ElementStat
	Properties    []PropertyStat
}

// GraphStats describes the Turtle this project produced, before any service
// saw it: the denominator of the measurement.
type GraphStats struct {
	Subjects    int
	Triples     int
	Bytes       int
	ByNamespace []PropertyStat // one entry per prefix, Written holding its triple count
}

// Report is a whole run.
type Report struct {
	Fixture   string
	Graph     GraphStats
	Load      SideReport
	Reference SideReport
	Findings  []string
}

// Text renders the report as the expectation file: sections of tab-separated
// records, every list sorted, so a diff between two runs is a diff between two
// measurements.
func (r *Report) Text(header string) string {
	var b strings.Builder
	b.WriteString(header)
	fmt.Fprintf(&b, "\n[graph]\nfixture\t%s\nsubjects\t%d\ntriples\t%d\nbytes\t%d\n",
		r.Fixture, r.Graph.Subjects, r.Graph.Triples, r.Graph.Bytes)
	for _, ns := range r.Graph.ByNamespace {
		fmt.Fprintf(&b, "triples.%s\t%d\n", ns.Property, ns.Written)
	}
	r.Load.write(&b)
	r.Reference.write(&b)

	b.WriteString("\n[findings]\n")
	for _, finding := range r.Findings {
		fmt.Fprintf(&b, "%s\n", finding)
	}
	return b.String()
}

// write renders one side: its totals, then a line per property and a line per
// element.
func (s *SideReport) write(b *strings.Builder) {
	fmt.Fprintf(b, "\n[%s]\naccepted\t%s\ncommits\t%d\nelements.written\t%d\nelements.listed\t%d\n"+
		"elements.unexpected\t%d\nlisting.responses\t%d\nlisting.paging-ignored\t%s\n"+
		"elements.readable-directly\t%d\nroots.reported\t%d\nroots.in-model\t%d\n",
		s.Name, yesNo(s.Accepted), s.Commits, s.Written, s.Listed, s.Extra, s.Pages,
		yesNo(s.IgnoredPaging), s.Direct, s.Roots, s.RootsInModel)

	fmt.Fprintf(b, "\n[%s.properties]\n", s.Name)
	properties := append([]PropertyStat(nil), s.Properties...)
	sort.Slice(properties, func(i, j int) bool { return properties[i].Property < properties[j].Property })
	for _, p := range properties {
		fmt.Fprintf(b, "%s\twritten=%d\tdelivered=%d\tmulti-valued=%d/%d\n",
			p.Property, p.Written, p.Delivered, p.MultiDelivered, p.MultiValued)
	}

	fmt.Fprintf(b, "\n[%s.elements]\n", s.Name)
	elements := append([]ElementStat(nil), s.Elements...)
	sort.Slice(elements, func(i, j int) bool { return elements[i].ID < elements[j].ID })
	for _, e := range elements {
		fmt.Fprintf(b, "%s\ttype=%s\tlisted=%s\tdirect=%s\tproperties=%d/%d",
			e.ID, orNone(e.Type), yesNo(e.Listed), e.Direct, e.Delivered, e.Written)
		if len(e.Lost) > 0 {
			lost := append([]string(nil), e.Lost...)
			sort.Strings(lost)
			fmt.Fprintf(b, "\tlost=%s", strings.Join(lost, ","))
		}
		if len(e.Shape) > 0 {
			shape := append([]string(nil), e.Shape...)
			sort.Strings(shape)
			fmt.Fprintf(b, "\tshape=%s", strings.Join(shape, ","))
		}
		b.WriteString("\n")
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func orNone(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
