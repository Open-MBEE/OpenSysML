package xpect

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	reports "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// Totals is one scope's adjudication, counted twice over: assertions are XPECT
// comments, rows are the expectations they declare (an errors note declares one
// per line).
type Totals struct {
	Files         int `json:"files"`
	FilesUnparsed int `json:"filesUnparsed"`
	Assertions    int `json:"assertions"`
	Rows          int `json:"rows"`
	// Agree includes WordingOnly: the same rule about the same element in our
	// own words is agreement, and the sub-count says how much of it that is.
	Agree          int `json:"agree"`
	WordingOnly    int `json:"wordingOnly"`
	Disagree       int `json:"disagree"`
	Unlocated      int `json:"unlocated"`
	NotAdjudicated int `json:"notAdjudicated"`
	MissingFiles   int `json:"missingResources"`
	// ForeignDiags counts diagnostics raised for another declared resource, not
	// adjudicated against the file under test.
	ForeignDiags int `json:"foreignDiagnostics"`
}

// KindTotals is one assertion kind's adjudication, with the tolerance columns
// that record what a weaker match would have accepted.
type KindTotals struct {
	Kind       string `json:"kind"`
	Assertions int    `json:"assertions"`
	// BlockAssertions are the `//* ... */` notes among Assertions.
	BlockAssertions int `json:"blockAssertions"`
	Rows            int `json:"rows"`
	Agree           int `json:"agree"`
	WordingOnly     int `json:"wordingOnly"`
	Disagree        int `json:"disagree"`
	Unlocated       int `json:"unlocated"`
	NotAdjudicated  int `json:"notAdjudicated"`
	SameLocation    int `json:"sameLocation"`
	SameLine        int `json:"sameLine"`
	OtherSeverity   int `json:"severityDiffers"`
	Elsewhere       int `json:"elsewhereInFile"`
	// Scope tolerance classes, counted only for the scope kind.
	OtherPaths      int `json:"otherPaths,omitempty"`
	ExtraNames      int `json:"extraNames,omitempty"`
	MissingNames    int `json:"missingNames,omitempty"`
	MissingAndExtra int `json:"missingAndExtra,omitempty"`
	LibraryNames    int `json:"libraryNames,omitempty"`
}

// SuiteReport is one Xpect plugin's adjudication.
type SuiteReport struct {
	Name   string       `json:"name"`
	Dir    string       `json:"dir"`
	Totals Totals       `json:"totals"`
	Kinds  []KindTotals `json:"kinds"`
	Files  []fileResult `json:"files"`
}

// Report is the whole run. It carries no timestamp and no absolute path, so two
// runs over the same pin produce byte-identical output.
type Report struct {
	Pilot   string `json:"pilot"`
	Corpus  string `json:"corpus"`
	Library string `json:"library"`
	// Provenance identifies the suites this run adjudicated, so a committed
	// baseline can be checked against the repository without provisioning them.
	Provenance baseline.Record `json:"provenance"`
	Totals     Totals          `json:"totals"`
	Kinds      []KindTotals    `json:"kinds"`
	Suites     []SuiteReport   `json:"suites"`
	Unparsed   []string        `json:"unparsed,omitempty"`
	Ignored    []string        `json:"ignoredNotes,omitempty"`
	Missing    []string        `json:"missingResources,omitempty"`
	// Errata is the same adjudication with the declared corrections applied;
	// Totals above stays the as-published headline.
	Errata *ErrataReport `json:"errata,omitempty"`
}

// kindOrder fixes the order of the per-kind table: the adjudicated kinds first,
// in the order the suites weigh them, then whatever else the files declare.
var kindOrder = []string{kindErrors, kindNoErrors, kindLinkedName, kindWarnings, kindScope}

func kindRank(kind string) int {
	for i, k := range kindOrder {
		if k == kind {
			return i
		}
	}
	return len(kindOrder)
}

// summarize aggregates per-suite and per-kind totals from the file results.
func (r *Report) summarize() {
	for i := range r.Suites {
		suite := &r.Suites[i]
		kinds := map[string]*KindTotals{}
		for i := range suite.Files {
			file := &suite.Files[i]
			suite.Totals.Files++
			if len(file.Problems) > 0 {
				suite.Totals.FilesUnparsed++
				r.Unparsed = append(r.Unparsed, fmt.Sprintf("%s/%s: %s",
					suite.Name, file.Path, strings.Join(file.Problems, "; ")))
			}
			for _, ignored := range file.Ignored {
				r.Ignored = append(r.Ignored, fmt.Sprintf("%s/%s: %s", suite.Name, file.Path, ignored))
			}
			suite.Totals.ForeignDiags += file.Foreign
			for _, missing := range file.Missing {
				suite.Totals.MissingFiles++
				r.Missing = append(r.Missing, fmt.Sprintf("%s/%s declares %s", suite.Name, file.Path, missing))
			}
			seen := map[int]bool{}
			for _, row := range file.Rows {
				kt := kinds[row.Kind]
				if kt == nil {
					kt = &KindTotals{Kind: row.Kind}
					kinds[row.Kind] = kt
				}
				kt.Rows++
				suite.Totals.Rows++
				if !seen[row.Line] {
					seen[row.Line] = true
					kt.Assertions++
					suite.Totals.Assertions++
					if row.Block {
						kt.BlockAssertions++
					}
				}
				switch row.Verdict {
				case verdictAgree:
					kt.Agree++
					suite.Totals.Agree++
				case verdictWordingOnly:
					kt.Agree++
					kt.WordingOnly++
					suite.Totals.Agree++
					suite.Totals.WordingOnly++
				case verdictDisagree:
					kt.Disagree++
					suite.Totals.Disagree++
				case verdictUnlocated:
					kt.Unlocated++
					suite.Totals.Unlocated++
				default:
					kt.NotAdjudicated++
					suite.Totals.NotAdjudicated++
				}
				switch row.Tolerance {
				case toleranceMessage:
					kt.SameLocation++
				case toleranceLine:
					kt.SameLine++
				case toleranceSeverity:
					kt.OtherSeverity++
				case toleranceAnywhere:
					kt.Elsewhere++
				case toleranceScopeSpelling:
					kt.OtherPaths++
				case toleranceScopeExtra:
					kt.ExtraNames++
				case toleranceScopeMissing:
					kt.MissingNames++
				case toleranceScopeBoth:
					kt.MissingAndExtra++
				case toleranceScopeLibrary:
					kt.LibraryNames++
				}
			}
			file.Expectations = len(file.Rows)
			for _, row := range file.Rows {
				if row.Verdict == verdictAgree || row.Verdict == verdictWordingOnly {
					file.Agree++
				}
			}
		}
		for _, kt := range kinds {
			suite.Kinds = append(suite.Kinds, *kt)
		}
		sortKinds(suite.Kinds)
		r.Totals.add(suite.Totals)
	}

	merged := map[string]*KindTotals{}
	for _, suite := range r.Suites {
		for _, kt := range suite.Kinds {
			m := merged[kt.Kind]
			if m == nil {
				m = &KindTotals{Kind: kt.Kind}
				merged[kt.Kind] = m
			}
			m.Assertions += kt.Assertions
			m.BlockAssertions += kt.BlockAssertions
			m.Rows += kt.Rows
			m.Agree += kt.Agree
			m.WordingOnly += kt.WordingOnly
			m.Disagree += kt.Disagree
			m.Unlocated += kt.Unlocated
			m.NotAdjudicated += kt.NotAdjudicated
			m.SameLocation += kt.SameLocation
			m.SameLine += kt.SameLine
			m.OtherSeverity += kt.OtherSeverity
			m.Elsewhere += kt.Elsewhere
			m.OtherPaths += kt.OtherPaths
			m.ExtraNames += kt.ExtraNames
			m.MissingNames += kt.MissingNames
			m.MissingAndExtra += kt.MissingAndExtra
			m.LibraryNames += kt.LibraryNames
		}
	}
	for _, kt := range merged {
		r.Kinds = append(r.Kinds, *kt)
	}
	sortKinds(r.Kinds)
	sort.Strings(r.Unparsed)
	sort.Strings(r.Ignored)
	sort.Strings(r.Missing)
}

func sortKinds(kinds []KindTotals) {
	sort.Slice(kinds, func(i, j int) bool {
		if ri, rj := kindRank(kinds[i].Kind), kindRank(kinds[j].Kind); ri != rj {
			return ri < rj
		}
		return kinds[i].Kind < kinds[j].Kind
	})
}

func (t *Totals) add(other Totals) {
	t.Files += other.Files
	t.FilesUnparsed += other.FilesUnparsed
	t.Assertions += other.Assertions
	t.Rows += other.Rows
	t.Agree += other.Agree
	t.WordingOnly += other.WordingOnly
	t.Disagree += other.Disagree
	t.Unlocated += other.Unlocated
	t.NotAdjudicated += other.NotAdjudicated
	t.MissingFiles += other.MissingFiles
	t.ForeignDiags += other.ForeignDiags
}

func writeReports(dir string, report *Report, log io.Writer) ([]byte, error) {
	files, err := reports.Open(dir, "pilot-xpect")
	if err != nil {
		return nil, err
	}
	encoded, err := files.JSON(report.pruned())
	if err != nil {
		return nil, err
	}
	if err := files.Text(renderText(report)); err != nil {
		return nil, err
	}
	files.Announce(log, fmt.Sprintf("%d .xt file(s), %d unparsed; %d assertion(s), %d expectation(s): %d agree (of which %d wording-only), %d disagree, %d unlocated, %d not adjudicated",
		report.Totals.Files, report.Totals.FilesUnparsed, report.Totals.Assertions, report.Totals.Rows,
		report.Totals.Agree, report.Totals.WordingOnly, report.Totals.Disagree,
		report.Totals.Unlocated, report.Totals.NotAdjudicated))
	return encoded, nil
}

// pruned drops the agreeing and not-adjudicated rows: the counts state how many
// there were, and listing them would treble the baseline for no verdict.
func (r *Report) pruned() *Report {
	out := *r
	out.Suites = make([]SuiteReport, len(r.Suites))
	for i, suite := range r.Suites {
		suite.Files = make([]fileResult, len(r.Suites[i].Files))
		for j, file := range r.Suites[i].Files {
			file.Rows = adjudicated(interesting(file.Rows))
			suite.Files[j] = file
		}
		out.Suites[i] = suite
	}
	return &out
}

func renderText(report *Report) string {
	var b strings.Builder
	b.WriteString("OpenSysML vs the OMG pilot implementation's own Xpect expectations\n")
	fmt.Fprintf(&b, "pilot pin: %s\ncorpus:    %s\nlibrary:   %s\n\n", report.Pilot, report.Corpus, report.Library)
	writeTotals(&b, "TOTAL", report.Totals)
	writeKinds(&b, report.Kinds)
	writeErrata(&b, report.Errata)

	for _, suite := range report.Suites {
		fmt.Fprintf(&b, "\n%s\n%s (%s)\n", strings.Repeat("=", 72), suite.Name, suite.Dir)
		writeTotals(&b, suite.Name, suite.Totals)
		writeKinds(&b, suite.Kinds)
		for _, file := range suite.Files {
			rows := interesting(file.Rows)
			if len(rows) == 0 && len(file.Problems) == 0 {
				continue
			}
			fmt.Fprintf(&b, "\n  %s\n", file.Path)
			for _, problem := range file.Problems {
				fmt.Fprintf(&b, "    unparsed: %s\n", problem)
			}
			for _, r := range rows {
				fmt.Fprintf(&b, "    line %-4d %-10s %-16s %s\n", r.Line, r.Kind, r.Verdict+tolerance(r), at(r))
				// A kind this harness does not rule on is listed, not quoted.
				if r.Verdict == verdictNotAdjudicated {
					continue
				}
				fmt.Fprintf(&b, "      declared: %s\n", r.Declared)
				if r.Actual != "" {
					fmt.Fprintf(&b, "      ours:     %s\n", r.Actual)
				}
			}
		}
	}

	if len(report.Unparsed) > 0 {
		fmt.Fprintf(&b, "\n%s\nunparsed files (%d)\n", strings.Repeat("=", 72), len(report.Unparsed))
		for _, line := range report.Unparsed {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	if len(report.Ignored) > 0 {
		fmt.Fprintf(&b, "\n%s\nXPECT-shaped text outside a `//` or `//*` note, not run (%d)\n", strings.Repeat("=", 72), len(report.Ignored))
		for _, line := range report.Ignored {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	if len(report.Missing) > 0 {
		fmt.Fprintf(&b, "\n%s\ndeclared resources missing from the download (%d)\n", strings.Repeat("=", 72), len(report.Missing))
		for _, line := range report.Missing {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	return b.String()
}

// interesting keeps the rows a reader has to see: everything but a strict
// agreement. A wording-only row is listed, so the class stays auditable.
func interesting(rows []row) []row {
	var out []row
	for _, r := range rows {
		if r.Verdict != verdictAgree {
			out = append(out, r)
		}
	}
	return out
}

// adjudicated keeps the rows this harness actually rules on.
func adjudicated(rows []row) []row {
	var out []row
	for _, r := range rows {
		if r.Verdict != verdictNotAdjudicated {
			out = append(out, r)
		}
	}
	return out
}

func tolerance(r row) string {
	if r.Tolerance == "" {
		return ""
	}
	return "(" + r.Tolerance + ")"
}

func at(r row) string {
	if r.At == "" {
		return ""
	}
	return fmt.Sprintf("at %q", r.At)
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
	fmt.Fprintf(b, "  %s\n", report.Note)
	writeTotals(b, "TOTAL with errata applied", report.Totals)
}

func writeTotals(b *strings.Builder, label string, t Totals) {
	fmt.Fprintf(b, "%s: %d .xt file(s), %d unparsed, %d missing declared resource(s)\n",
		label, t.Files, t.FilesUnparsed, t.MissingFiles)
	fmt.Fprintf(b, "  %d assertion(s) declaring %d expectation(s)\n", t.Assertions, t.Rows)
	fmt.Fprintf(b, "  agree %d (of which wording-only %d) | disagree %d | unlocated %d | not adjudicated %d\n",
		t.Agree, t.WordingOnly, t.Disagree, t.Unlocated, t.NotAdjudicated)
	if t.ForeignDiags > 0 {
		fmt.Fprintf(b, "  %d diagnostic(s) another declared resource raised, not adjudicated against the file\n", t.ForeignDiags)
	}
}

func writeKinds(b *strings.Builder, kinds []KindTotals) {
	if len(kinds) == 0 {
		return
	}
	fmt.Fprintf(b, "  %-16s %9s %7s %9s %7s %11s %9s %10s %14s %10s %10s %8s %10s\n",
		"kind", "asserts", "block", "expects", "agree", "wordingOnly", "disagree", "unlocated",
		"notAdjudicated", "sameLoc", "sameLine", "sevDiff", "elsewhere")
	for _, kt := range kinds {
		fmt.Fprintf(b, "  %-16s %9d %7d %9d %7d %11d %9d %10d %14d %10d %10d %8d %10d\n",
			kt.Kind, kt.Assertions, kt.BlockAssertions, kt.Rows, kt.Agree, kt.WordingOnly, kt.Disagree,
			kt.Unlocated, kt.NotAdjudicated, kt.SameLocation, kt.SameLine, kt.OtherSeverity, kt.Elsewhere)
	}
	writeScopeClasses(b, kinds)
}

// writeScopeClasses breaks scope disagreements down by which half of the
// declared set differs, which the shared tolerance columns cannot hold.
func writeScopeClasses(b *strings.Builder, kinds []KindTotals) {
	for _, kt := range kinds {
		if kt.Kind != kindScope || kt.Disagree == 0 {
			continue
		}
		fmt.Fprintf(b, "  scope classes: other-paths %d | extra-names %d | missing-names %d | missing-and-extra %d | library-names %d\n",
			kt.OtherPaths, kt.ExtraNames, kt.MissingNames, kt.MissingAndExtra, kt.LibraryNames)
	}
}
