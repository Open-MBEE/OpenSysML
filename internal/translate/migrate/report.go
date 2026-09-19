package migrate

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
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
	// tool bundled, a diagram — and is left out without a comment.
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
}

// Count returns how many entries carry each verdict.
func (r *Report) Count() map[Verdict]int {
	counts := map[Verdict]int{}
	for _, e := range r.Entries {
		counts[e.Verdict]++
	}
	return counts
}

// Summary is the one-line account a command prints after migrating.
func (r *Report) Summary() string {
	c := r.Count()
	total := len(r.Entries) - c[Skipped]
	return fmt.Sprintf("migrated %d element(s): %d mapped, %d approximated, %d unmapped (%d skipped as profile or library content)",
		total, c[Mapped], c[Approximated], c[Unmapped], c[Skipped])
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
