package diff

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
	"github.com/Open-MBEE/OpenSysML/internal/junit"
	reports "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// Report is the machine-readable result. It holds the compared tuples and their
// counts, but no message text, so two runs diff cleanly.
type Report struct {
	Validator string `json:"validator"`
	Pilot     string `json:"pilotRelease"`
	// Provenance identifies the inputs this run measured, so a committed
	// baseline can be checked against the repository without the validators.
	Provenance baseline.Record `json:"provenance"`
	Totals     Totals          `json:"totals"`
	Roots      []RootReport    `json:"roots"`
	Unmapped   []UnmappedRow   `json:"unmapped"`
	// Syside is the optional third implementation's column, additive in every
	// respect: absent, the report is byte-identical to a two-way run.
	Syside *SysideInfo `json:"syside,omitempty"`
	// Errata is the same run over the corpus with the declared corrections
	// applied; Totals above stays the as-published headline.
	Errata *ErrataReport `json:"errata,omitempty"`
}

type Totals struct {
	Files            int `json:"files"`
	FilesAgreeing    int `json:"filesFullyAgreeing"`
	Agreement        int `json:"agreement"`
	SeverityMismatch int `json:"severityMismatch"`
	OpenSysMLOnly    int `json:"openSysMLOnly"`
	PilotOnly        int `json:"pilotOnly"`
	OpenSysMLTotal   int `json:"openSysMLDiagnostics"`
	PilotTotal       int `json:"pilotDiagnostics"`
}

type RootReport struct {
	Name   string        `json:"name"`
	Dir    string        `json:"dir"`
	Totals Totals        `json:"totals"`
	Files  []FileReport  `json:"files"`
	Syside *SysideTotals `json:"syside,omitempty"`
	// ErrataTotals is this root compared again with its corrections applied,
	// present only for a root some correction lies in.
	ErrataTotals *Totals `json:"errataTotals,omitempty"`
}

// FileReport holds one file's buckets. Files where both implementations are
// silent are omitted.
type FileReport struct {
	Path             string          `json:"path"`
	Agreement        []Entry         `json:"agreement"`
	SeverityMismatch []SeverityEntry `json:"severityMismatch"`
	OpenSysMLOnly    []Entry         `json:"openSysMLOnly"`
	PilotOnly        []Entry         `json:"pilotOnly"`
	Syside           *SysideFile     `json:"syside,omitempty"`
}

// SeverityEntry is a (line, category) both implementations flag with different
// severities. Kept apart so such a pair is neither counted as agreement nor
// double-counted as two independent disagreements.
type SeverityEntry struct {
	Line      int      `json:"line"`
	Category  Category `json:"category"`
	OpenSysML string   `json:"openSysMLSeverity"`
	Pilot     string   `json:"pilotSeverity"`
	Count     int      `json:"count"`
	Examples  []string `json:"examples,omitempty"`
}

// Entry is a compared tuple with how many diagnostics carry it. Examples hold
// message text for human adjudication only; they are never compared.
type Entry struct {
	Line     int      `json:"line"`
	Severity string   `json:"severity"`
	Category Category `json:"category"`
	Count    int      `json:"count"`
	Examples []string `json:"examples,omitempty"`
}

// UnmappedRow records a message that no category rule claimed, so the report
// shows what the categorisation still owes rather than hiding it.
type UnmappedRow struct {
	Side    string `json:"side"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

func compareRoot(name, dir string, files []string, ours, theirs map[string][]diagnostic) RootReport {
	report := RootReport{Name: name, Dir: dir}
	for _, rel := range files {
		file := compareFile(rel, ours[rel], theirs[rel])
		report.Totals.Files++
		report.Totals.OpenSysMLTotal += len(ours[rel])
		report.Totals.PilotTotal += len(theirs[rel])
		for _, entry := range file.Agreement {
			report.Totals.Agreement += entry.Count
		}
		for _, entry := range file.SeverityMismatch {
			report.Totals.SeverityMismatch += entry.Count
		}
		for _, entry := range file.OpenSysMLOnly {
			report.Totals.OpenSysMLOnly += entry.Count
		}
		for _, entry := range file.PilotOnly {
			report.Totals.PilotOnly += entry.Count
		}
		if len(file.OpenSysMLOnly) == 0 && len(file.PilotOnly) == 0 && len(file.SeverityMismatch) == 0 {
			report.Totals.FilesAgreeing++
		}
		if len(file.Agreement) > 0 || len(file.SeverityMismatch) > 0 || len(file.OpenSysMLOnly) > 0 || len(file.PilotOnly) > 0 {
			report.Files = append(report.Files, file)
		}
	}
	return report
}

// compareFile buckets one file's diagnostics. Tuples are compared as multisets:
// when one side reports a tuple three times and the other twice, two are
// agreement and the third stays a disagreement.
func compareFile(rel string, ours, theirs []diagnostic) FileReport {
	ourGroups := group(ours)
	theirGroups := group(theirs)

	report := FileReport{Path: rel}
	ourLeft := map[key][]diagnostic{}
	theirLeft := map[key][]diagnostic{}
	for _, k := range sortedKeys(ourGroups, theirGroups) {
		mine, yours := ourGroups[k], theirGroups[k]
		shared := min(len(mine), len(yours))
		if shared > 0 {
			report.Agreement = append(report.Agreement, entry(k, shared, mine[:shared], yours[:shared]))
		}
		if len(mine) > shared {
			ourLeft[k] = mine[shared:]
		}
		if len(yours) > shared {
			theirLeft[k] = yours[shared:]
		}
	}

	// Second pass: a leftover pair that agrees on line and category but not on
	// severity is its own bucket rather than two unrelated disagreements.
	for _, k := range sortedKeys(ourLeft) {
		for _, other := range sortedKeys(theirLeft) {
			if other.Line != k.Line || other.Category != k.Category || other.Severity == k.Severity {
				continue
			}
			mine, yours := ourLeft[k], theirLeft[other]
			shared := min(len(mine), len(yours))
			if shared == 0 {
				continue
			}
			se := SeverityEntry{
				Line: k.Line, Category: k.Category,
				OpenSysML: k.Severity, Pilot: other.Severity, Count: shared,
				Examples: entry(k, shared, mine[:shared], yours[:shared]).Examples,
			}
			report.SeverityMismatch = append(report.SeverityMismatch, se)
			ourLeft[k], theirLeft[other] = mine[shared:], yours[shared:]
		}
	}

	for _, k := range sortedKeys(ourLeft) {
		if left := ourLeft[k]; len(left) > 0 {
			report.OpenSysMLOnly = append(report.OpenSysMLOnly, entry(k, len(left), left, nil))
		}
	}
	for _, k := range sortedKeys(theirLeft) {
		if left := theirLeft[k]; len(left) > 0 {
			report.PilotOnly = append(report.PilotOnly, entry(k, len(left), nil, left))
		}
	}
	return report
}

func entry(k key, count int, ours, theirs []diagnostic) Entry {
	e := Entry{Line: k.Line, Severity: k.Severity, Category: k.Category, Count: count}
	for _, d := range ours {
		e.Examples = append(e.Examples, "opensysml: "+d.Message)
	}
	for _, d := range theirs {
		e.Examples = append(e.Examples, "pilot: "+d.Message)
	}
	return e
}

func group(diagnostics []diagnostic) map[key][]diagnostic {
	groups := make(map[key][]diagnostic, len(diagnostics))
	for _, d := range diagnostics {
		groups[d.key()] = append(groups[d.key()], d)
	}
	return groups
}

func sortedKeys(groups ...map[key][]diagnostic) []key {
	seen := make(map[key]bool)
	var keys []key
	for _, g := range groups {
		for k := range g {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Line != keys[j].Line {
			return keys[i].Line < keys[j].Line
		}
		if keys[i].Severity != keys[j].Severity {
			return keys[i].Severity < keys[j].Severity
		}
		return keys[i].Category < keys[j].Category
	})
	return keys
}

// summarize fills in the aggregate totals and the unmapped-message table.
func (r *Report) summarize() {
	unmapped := make(map[UnmappedRow]int)
	for _, root := range r.Roots {
		if r.Errata != nil {
			// A root no correction lies in contributes its published totals.
			errataTotals := root.Totals
			if root.ErrataTotals != nil {
				errataTotals = *root.ErrataTotals
			}
			r.Errata.Totals.add(errataTotals)
		}
		r.Totals.Files += root.Totals.Files
		r.Totals.FilesAgreeing += root.Totals.FilesAgreeing
		r.Totals.Agreement += root.Totals.Agreement
		r.Totals.SeverityMismatch += root.Totals.SeverityMismatch
		r.Totals.OpenSysMLOnly += root.Totals.OpenSysMLOnly
		r.Totals.PilotOnly += root.Totals.PilotOnly
		r.Totals.OpenSysMLTotal += root.Totals.OpenSysMLTotal
		r.Totals.PilotTotal += root.Totals.PilotTotal

		for _, file := range root.Files {
			for _, bucket := range [][]Entry{file.Agreement, file.OpenSysMLOnly, file.PilotOnly} {
				for _, e := range bucket {
					if e.Category != CategoryUnmapped {
						continue
					}
					countUnmapped(unmapped, e.Examples)
				}
			}
			for _, e := range file.SeverityMismatch {
				if e.Category == CategoryUnmapped {
					countUnmapped(unmapped, e.Examples)
				}
			}
			if file.Syside != nil {
				countSysideUnmapped(unmapped, *file.Syside)
			}
		}
		if root.Syside != nil && r.Syside != nil {
			r.Syside.Totals.addRoot(*root.Syside)
		}
	}

	for row, count := range unmapped {
		row.Count = count
		r.Unmapped = append(r.Unmapped, row)
	}
	sort.Slice(r.Unmapped, func(i, j int) bool {
		if r.Unmapped[i].Count != r.Unmapped[j].Count {
			return r.Unmapped[i].Count > r.Unmapped[j].Count
		}
		if r.Unmapped[i].Side != r.Unmapped[j].Side {
			return r.Unmapped[i].Side < r.Unmapped[j].Side
		}
		return r.Unmapped[i].Message < r.Unmapped[j].Message
	})
}

func (t *Totals) add(other Totals) {
	t.Files += other.Files
	t.FilesAgreeing += other.FilesAgreeing
	t.Agreement += other.Agreement
	t.SeverityMismatch += other.SeverityMismatch
	t.OpenSysMLOnly += other.OpenSysMLOnly
	t.PilotOnly += other.PilotOnly
	t.OpenSysMLTotal += other.OpenSysMLTotal
	t.PilotTotal += other.PilotTotal
}

func countUnmapped(unmapped map[UnmappedRow]int, examples []string) {
	for _, example := range examples {
		side, message, _ := strings.Cut(example, ": ")
		unmapped[UnmappedRow{Side: side, Message: message}]++
	}
}

func writeReports(dir string, report *Report, log io.Writer) ([]byte, error) {
	files, err := reports.Open(dir, "pilot-diff")
	if err != nil {
		return nil, err
	}

	// The JSON drops the message examples so a later run diffs cleanly.
	machine := *report
	machine.Roots = make([]RootReport, len(report.Roots))
	for i, root := range report.Roots {
		machine.Roots[i] = root
		machine.Roots[i].Files = make([]FileReport, len(root.Files))
		for j, file := range root.Files {
			machine.Roots[i].Files[j] = FileReport{
				Path:             file.Path,
				Agreement:        stripExamples(file.Agreement),
				SeverityMismatch: stripSeverityExamples(file.SeverityMismatch),
				OpenSysMLOnly:    stripExamples(file.OpenSysMLOnly),
				PilotOnly:        stripExamples(file.PilotOnly),
				Syside:           stripSysideExamples(file.Syside),
			}
		}
	}
	encoded, err := files.JSON(machine)
	if err != nil {
		return nil, err
	}
	if err := files.Text(renderText(report)); err != nil {
		return nil, err
	}
	xml, err := junit.Marshal(junitReport(report))
	if err != nil {
		return nil, err
	}
	if err := files.Write("xml", xml); err != nil {
		return nil, err
	}
	sarif, err := marshalSarif(sarifReport(report))
	if err != nil {
		return nil, err
	}
	if err := files.Write("sarif", sarif); err != nil {
		return nil, err
	}
	files.Announce(log, fmt.Sprintf("%d file(s), %d fully agreeing; %d agreed diagnostic(s), %d only ours, %d only the pilot's",
		report.Totals.Files, report.Totals.FilesAgreeing, report.Totals.Agreement,
		report.Totals.OpenSysMLOnly, report.Totals.PilotOnly))
	return encoded, nil
}

func stripExamples(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	for i, e := range entries {
		e.Examples = nil
		out[i] = e
	}
	return out
}

func stripSeverityExamples(entries []SeverityEntry) []SeverityEntry {
	out := make([]SeverityEntry, len(entries))
	for i, e := range entries {
		e.Examples = nil
		out[i] = e
	}
	return out
}

func renderText(report *Report) string {
	var b strings.Builder
	b.WriteString("OpenSysML vs OMG SysML v2 Pilot Implementation — diagnostic comparison\n")
	fmt.Fprintf(&b, "pilot release: %s\nvalidator:     %s\n\n", report.Pilot, report.Validator)
	if report.Syside != nil {
		fmt.Fprintf(&b, "third implementation: Sensmetry SysIDE %s, %s standard library (%s)\n",
			report.Syside.Version, report.Syside.Library, report.Syside.Validator)
		fmt.Fprintf(&b, "scope: %s\n\n", report.Syside.Scope)
	}
	writeTotals(&b, "TOTAL", report.Totals)
	if report.Syside != nil {
		writeSysideTotals(&b, report.Syside.Totals)
	}
	writeErrata(&b, report.Errata)

	for _, root := range report.Roots {
		fmt.Fprintf(&b, "\n%s\n", strings.Repeat("=", 72))
		fmt.Fprintf(&b, "%s (%s)\n", root.Name, root.Dir)
		writeTotals(&b, root.Name, root.Totals)
		if root.Syside != nil {
			writeSysideTotals(&b, *root.Syside)
		}
		for _, file := range root.Files {
			fmt.Fprintf(&b, "\n  %s\n", file.Path)
			writeBucket(&b, "agreement", file.Agreement)
			writeSeverityBucket(&b, file.SeverityMismatch)
			writeBucket(&b, "only OpenSysML (candidate false positives)", file.OpenSysMLOnly)
			writeBucket(&b, "only the pilot (candidate gaps)", file.PilotOnly)
			writeSysideBucket(&b, file.Syside)
		}
	}

	if len(report.Unmapped) > 0 {
		fmt.Fprintf(&b, "\n%s\nunmapped messages (no category rule claimed these)\n", strings.Repeat("=", 72))
		for _, row := range report.Unmapped {
			fmt.Fprintf(&b, "  %4d  %-10s %s\n", row.Count, row.Side, row.Message)
		}
	}
	return b.String()
}

// writeErrata states the second figure after the headline, never instead of it.
func writeErrata(b *strings.Builder, report *ErrataReport) {
	if report == nil {
		return
	}
	fmt.Fprintf(b, "\ndeclared errata: %d entr(ies), %d correction(s), %d documented without one; %d applied here\n",
		report.Registry, report.Corrections, report.Documented, report.Applied)
	for _, entry := range report.Entries {
		shape := "documented, no substitution"
		if entry.Corrected {
			shape = "corrected"
		}
		fmt.Fprintf(b, "  %s %s:%d (%s) — %s\n", entry.ID, entry.Path, entry.Line, entry.Citation, shape)
	}
	writeTotals(b, "TOTAL with errata applied", report.Totals)
	for _, f := range report.Findings {
		fmt.Fprintf(b, "  %s %s:%d ours %d->%d, pilot %d->%d: %s\n",
			f.ID, f.Path, f.Line, f.OursPublished, f.OursCorrected, f.PilotPublished, f.PilotCorrected, f.Note)
	}
}

func writeTotals(b *strings.Builder, label string, totals Totals) {
	fmt.Fprintf(b, "%s: %d file(s), %d fully agreeing\n", label, totals.Files, totals.FilesAgreeing)
	fmt.Fprintf(b, "  diagnostics: %d ours, %d pilot\n", totals.OpenSysMLTotal, totals.PilotTotal)
	fmt.Fprintf(b, "  agreement %d | severity-only %d | only ours %d | only pilot %d\n",
		totals.Agreement, totals.SeverityMismatch, totals.OpenSysMLOnly, totals.PilotOnly)
}

func writeSeverityBucket(b *strings.Builder, entries []SeverityEntry) {
	if len(entries) == 0 {
		return
	}
	b.WriteString("    same line and category, different severity:\n")
	for _, e := range entries {
		fmt.Fprintf(b, "      line %-5d ours=%-8s pilot=%-8s %-20s x%d\n",
			e.Line, e.OpenSysML, e.Pilot, e.Category, e.Count)
		for _, example := range e.Examples {
			fmt.Fprintf(b, "        %s\n", example)
		}
	}
}

func writeBucket(b *strings.Builder, label string, entries []Entry) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(b, "    %s:\n", label)
	for _, e := range entries {
		fmt.Fprintf(b, "      line %-5d %-8s %-20s x%d\n", e.Line, e.Severity, e.Category, e.Count)
		for _, example := range e.Examples {
			fmt.Fprintf(b, "        %s\n", example)
		}
	}
}
