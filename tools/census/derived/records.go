package derived

import (
	"bytes"
	"fmt"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"sort"
	"strconv"
	"strings"
)

// CommittedRecords holds every root's rule counts, one sorted line per rule and
// root, beside the header CommittedBaseline keeps; the totals are derived.
const CommittedRecords = "internal/semantic/metamodel/derived_census_records.tsv"

const recordsHeader = "# One line per served rule and corpus root; re-record with " + refreshCommand + "\n" +
	"# rule\troot\tours.carriers\tours.value\tours.empty\tours.notDerived\tpilot.carriers\tpilot.value\tpilot.empty\tpilot.notDerived\n"

// encodeRecords writes the roots' rules, by rule then root order.
func encodeRecords(r *Report) []byte {
	var b bytes.Buffer
	b.WriteString(recordsHeader)
	names := map[string]bool{}
	for _, root := range r.Roots {
		for name := range root.Rules {
			names[name] = true
		}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		for _, root := range r.Roots {
			counts, ok := root.Rules[name]
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", name, root.Name,
				counts.Ours.Carriers, counts.Ours.Value, counts.Ours.Empty, counts.Ours.NotDerived,
				counts.Pilot.Carriers, counts.Pilot.Value, counts.Pilot.Empty, counts.Pilot.NotDerived)
		}
	}
	return b.Bytes()
}

// decodeRecords fills the roots' rules from the records file.
func decodeRecords(data []byte, r *Report) error {
	roots := map[string]*RootReport{}
	for i := range r.Roots {
		r.Roots[i].Rules = map[string]RuleCounts{}
		roots[r.Roots[i].Name] = &r.Roots[i]
	}
	for number, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 10 {
			return fmt.Errorf("%s:%d: want 10 tab-separated fields, got %d", CommittedRecords, number+1, len(fields))
		}
		root, ok := roots[fields[1]]
		if !ok {
			return fmt.Errorf("%s:%d: root %q is not in %s", CommittedRecords, number+1, fields[1], CommittedBaseline)
		}
		var n [8]int
		for i := range n {
			v, err := strconv.Atoi(fields[2+i])
			if err != nil {
				return fmt.Errorf("%s:%d: field %d: %w", CommittedRecords, number+1, 3+i, err)
			}
			n[i] = v
		}
		root.Rules[fields[0]] = RuleCounts{Ours: Counts{Carriers: n[0], Value: n[1], Empty: n[2], NotDerived: n[3]},
			Pilot: Counts{Carriers: n[4], Value: n[5], Empty: n[6], NotDerived: n[7]}}
	}
	return nil
}

// header is the committed JSON: the provenance and every root's figures, without
// the rules the records carry or the totals derived from them.
// headerView is the committed header: the report without the records, totals and
// summary the records file carries or the reader derives.
type headerView struct {
	Provenance baseline.Record `json:"provenance"`
	Structure  string          `json:"structure"`
	Policy     []string        `json:"policy"`
	Roots      []rootHeader    `json:"roots"`
}

// rootHeader is a RootReport without its rules.
type rootHeader struct {
	Name            string `json:"name"`
	Dir             string `json:"dir"`
	Files           int    `json:"files"`
	Rejected        int    `json:"rejected"`
	Elements        int    `json:"elements"`
	Implied         int    `json:"implied"`
	PilotElements   int    `json:"pilotElements"`
	PilotImplied    int    `json:"pilotImplied"`
	PilotDuplicates int    `json:"pilotDuplicates"`
	PilotFailed     int    `json:"pilotFailed"`
}

func header(r *Report) headerView {
	h := headerView{Provenance: r.Provenance, Structure: r.Structure, Policy: r.Policy}
	for _, root := range r.Roots {
		h.Roots = append(h.Roots, rootHeader{
			Name: root.Name, Dir: root.Dir, Files: root.Files, Rejected: root.Rejected,
			Elements: root.Elements, Implied: root.Implied,
			PilotElements: root.PilotElements, PilotImplied: root.PilotImplied,
			PilotDuplicates: root.PilotDuplicates, PilotFailed: root.PilotFailed,
		})
	}
	return h
}
