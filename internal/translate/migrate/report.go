package migrate

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Verdict is what the migration did with one SysML v1 element.
type Verdict int

const (
	// Mapped: the element has a SysML v2 counterpart that states the same thing.
	Mapped Verdict = iota
	// Approximated: the element was written, but the v2 form loses or
	// restates part of what v1 said; the note says what.
	Approximated
	// Unmapped: the element has no v2 counterpart in this migration and is
	// recorded as a comment where it stood.
	Unmapped
	// Skipped: the element is not the user's model — a profile, a library the
	// tool bundled, a diagram — or nothing in the model refers to it, so no v2
	// form would say anything; it is left out without a comment.
	Skipped
)

func (v Verdict) String() string {
	switch v {
	case Mapped:
		return "mapped"
	case Approximated:
		return "approximated"
	case Unmapped:
		return "unmapped"
	default:
		return "skipped"
	}
}

// MarshalText writes the verdict by name, so the JSON report reads as the text one.
func (v Verdict) MarshalText() ([]byte, error) { return []byte(v.String()), nil }

// UnmarshalText reads a verdict written by MarshalText.
func (v *Verdict) UnmarshalText(text []byte) error {
	for _, c := range []Verdict{Mapped, Approximated, Unmapped, Skipped} {
		if c.String() == string(text) {
			*v = c
			return nil
		}
	}
	return fmt.Errorf("unknown migration verdict %q", text)
}

// Entry records the verdict on one v1 element.
type Entry struct {
	// ID is the element's xmi:id, the handle a v1 tool addresses it by.
	ID string `json:"id"`
	// Kind is the v1 element as modeled: its stereotype when one classifies it
	// («Block», «Requirement»), else its UML metaclass.
	Kind string `json:"kind"`
	// Name is the element's qualified name in the v1 model.
	Name string `json:"name"`
	// Target is the v2 declaration written for it (a qualified name with its
	// keyword), or "" when nothing was.
	Target  string  `json:"target,omitempty"`
	Verdict Verdict `json:"verdict"`
	// Note explains an approximation or an omission.
	Note string `json:"note,omitempty"`
}

// Report is the per-element account of a migration.
type Report struct {
	// Source names the v1 document migrated.
	Source string `json:"source"`
	// Exporter is the tool the document says wrote it, when it says.
	Exporter string  `json:"exporter,omitempty"`
	Entries  []Entry `json:"entries"`
	// Layout accounts for the MTIP export the migration was augmented with;
	// nil when none was.
	Layout *LayoutSummary `json:"layout,omitempty"`
}

// LayoutSummary accounts for what an MTIP export contributed to a migration:
// how many of its diagram records joined, and how much of their geometry was
// written into the views.
type LayoutSummary struct {
	Source       string `json:"source"`
	MTIPVersion  string `json:"mtipVersion,omitempty"`
	CameoVersion string `json:"cameoVersion,omitempty"`
	ExportTime   string `json:"exportTime,omitempty"`
	// Diagrams counts the export's diagram records; DiagramsJoined those whose
	// id is a migrated diagram's, DiagramsUnmatched those matching none.
	Diagrams          int `json:"diagrams"`
	DiagramsJoined    int `json:"diagramsJoined"`
	DiagramsUnmatched int `json:"diagramsUnmatched"`
	// ViewsWithoutLayout counts the migrated diagrams the export does not cover.
	ViewsWithoutLayout int `json:"viewsWithoutLayout"`
	// Placements counts the shown elements the export positions;
	// PlacementsWritten those positioned into a view, PlacementsUnexposed
	// those resolving to an element the view does not expose, and
	// PlacementsDangling those resolving to no element. Routes likewise per
	// connector.
	Placements          int `json:"placements"`
	PlacementsWritten   int `json:"placementsWritten"`
	PlacementsUnexposed int `json:"placementsUnexposed"`
	PlacementsDangling  int `json:"placementsDangling"`
	Routes              int `json:"routes"`
	RoutesWritten       int `json:"routesWritten"`
	RoutesUnexposed     int `json:"routesUnexposed"`
	RoutesDangling      int `json:"routesDangling"`
	// Malformed counts the export's malformed records; Unsupported the
	// presentation properties DiagramLayout carries no attribute for, by tag.
	Malformed   int            `json:"malformed"`
	Unsupported map[string]int `json:"unsupported,omitempty"`
}

// Count returns how many entries carry each verdict.
func (r *Report) Count() map[Verdict]int {
	counts := map[Verdict]int{}
	for _, e := range r.Entries {
		counts[e.Verdict]++
	}
	return counts
}

// unreferencedNote opens the note of a model element skipped because nothing
// in the model refers to it, so that the summary can count those apart.
const unreferencedNote = "not referenced by any behavior"

// Unreferenced returns how many skipped entries are the user's own elements
// that nothing refers to, as opposed to profile, library or notation content.
func (r *Report) Unreferenced() int {
	n := 0
	for _, e := range r.Entries {
		if e.Verdict == Skipped && strings.HasPrefix(e.Note, unreferencedNote) {
			n++
		}
	}
	return n
}

// Summary is the one-line account a command prints after migrating.
func (r *Report) Summary() string {
	c := r.Count()
	total := len(r.Entries) - c[Skipped]
	unreferenced := r.Unreferenced()
	s := fmt.Sprintf("migrated %d element(s): %d mapped, %d approximated, %d unmapped (%d skipped as profile, library or notation-only content, %d as model elements nothing refers to)",
		total, c[Mapped], c[Approximated], c[Unmapped], c[Skipped]-unreferenced, unreferenced)
	if r.Layout != nil {
		s += fmt.Sprintf("; laid out %d of %d diagrams from %s: %s elements positioned, %s connectors routed",
			r.Layout.DiagramsJoined, r.Layout.DiagramsJoined+r.Layout.ViewsWithoutLayout, r.Layout.Source,
			commas(r.Layout.PlacementsWritten), commas(r.Layout.RoutesWritten))
	}
	return s
}

// commas groups an integer's digits by thousands for the summary sentence.
func commas(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// WriteText writes the report as a table, one element per line, grouped by
// verdict with the elements that need attention first.
func (r *Report) WriteText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# SysML v1 to v2 migration report: %s\n", r.Source)
	if r.Exporter != "" {
		fmt.Fprintf(&b, "# exported by %s\n", r.Exporter)
	}
	fmt.Fprintf(&b, "# %s\n", r.Summary())
	if l := r.Layout; l != nil {
		b.WriteString("\n## layout\n")
		version := l.MTIPVersion
		if l.CameoVersion != "" {
			version += ", cameo " + l.CameoVersion
		}
		if l.ExportTime != "" {
			version += ", exported " + l.ExportTime
		}
		if version != "" {
			fmt.Fprintf(&b, "# source: %s (%s)\n", l.Source, version)
		} else {
			fmt.Fprintf(&b, "# source: %s\n", l.Source)
		}
		fmt.Fprintf(&b, "# %d diagram records: %d joined, %d matching no diagram; %d views without layout\n",
			l.Diagrams, l.DiagramsJoined, l.DiagramsUnmatched, l.ViewsWithoutLayout)
		fmt.Fprintf(&b, "# placements: %d of %d written (%d not exposed, %d resolving to no element); routes: %d of %d written (%d not exposed, %d resolving to no element); malformed: %d\n",
			l.PlacementsWritten, l.Placements, l.PlacementsUnexposed, l.PlacementsDangling,
			l.RoutesWritten, l.Routes, l.RoutesUnexposed, l.RoutesDangling, l.Malformed)
		if len(l.Unsupported) > 0 {
			tags := make([]string, 0, len(l.Unsupported))
			for tag := range l.Unsupported {
				tags = append(tags, tag)
			}
			sort.Strings(tags)
			var props []string
			for _, tag := range tags {
				props = append(props, fmt.Sprintf("%s (%d)", tag, l.Unsupported[tag]))
			}
			fmt.Fprintf(&b, "# unsupported presentation properties, dropped: %s\n", strings.Join(props, ", "))
		}
	}
	for _, v := range []Verdict{Unmapped, Approximated, Mapped, Skipped} {
		var entries []Entry
		for _, e := range r.Entries {
			if e.Verdict == v {
				entries = append(entries, e)
			}
		}
		if len(entries) == 0 {
			continue
		}
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		fmt.Fprintf(&b, "\n## %s (%d)\n", v, len(entries))
		for _, e := range entries {
			fmt.Fprintf(&b, "%s\t%s\t%s", field(e.Kind), field(e.Name), field(e.ID))
			if e.Target != "" {
				fmt.Fprintf(&b, "\t-> %s", field(e.Target))
			}
			if e.Note != "" {
				fmt.Fprintf(&b, "\t(%s)", field(e.Note))
			}
			b.WriteByte('\n')
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// fieldEscaper escapes the whitespace that would break the one-line-per-entry table.
var fieldEscaper = strings.NewReplacer("\r\n", `\n`, "\n", `\n`, "\r", `\r`, "\t", `\t`)

func field(s string) string { return fieldEscaper.Replace(s) }

// WriteJSON writes the report as one JSON document.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
