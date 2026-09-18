package diff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/doccounts"
)

const (
	docCountPath               = "docs/project/pilot-differential.md"
	docCountBaselinePath       = "docs/project/pilot-differential-baseline.json"
	docCountReadmePath         = "README.md"
	docCountArchitecturePath   = "docs/internals/architecture.md"
	docCountSpecCompliancePath = "docs/project/spec-compliance.md"
	docCountReferenceMarker    = "**Reference differential:**"
	docCountRejectionMarker    = "**Rejection oracle:**"
)

type docLine struct {
	number int
	text   string
}

type docTable struct {
	header     []string
	headerLine int
	rows       []docTableRow
}

type docTableRow struct {
	line  int
	cells []string
}

type docNumber struct {
	start int
	end   int
	text  string
	line  int
}

var (
	docHeadlinePattern       = regexp.MustCompile(`^## Results \(pilot ` + "`" + `([^` + "`" + `]+)` + "`" + `, ([0-9]+) files\)$`)
	docParentheticalPattern  = regexp.MustCompile(`\s+\([^()]*\)$`)
	docCategoryItemPattern   = regexp.MustCompile("^(\\d+)\\s+(`?[A-Za-z][A-Za-z0-9-]*`?)")
	docNextCategoryPattern   = regexp.MustCompile(",\\s*\\d+\\s+`?[A-Za-z][A-Za-z0-9-]*`?")
	docIntegerPattern        = regexp.MustCompile(`^-?[0-9]+$`)
	docRootPattern           = regexp.MustCompile("(?s)^\\s*`([^`]+)`\\s*(.*)$")
	docReferencePattern      = regexp.MustCompile("^\\*\\*Reference differential:\\*\\* ([0-9]+) files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation \\(`([^`]+)`\\), ([0-9]+) in full agreement;")
	docMovementRowsUnchecked = map[string]bool{"new checks of ours": true}

	// The five headline numbers the README and the architecture guide lead with,
	// each re-derived below from the baseline of the harness that measured it.
	docCorpusAgreementPattern = regexp.MustCompile(`^- \*\*Corpus agreement:\*\* ([0-9]+) of ([0-9]+) files agree diagnostic-by-diagnostic; ([0-9]+) diagnostics are ours alone and ([0-9]+) the reference's alone`)
	docDeclaredSilencePattern = regexp.MustCompile("^- \\*\\*Declared-diagnostic silence:\\*\\* of the ([0-9]+) declared `errors` rows in the reference's own Xpect suites, we report nothing for ([0-9]+)\\. ([0-9]+) we report word-for-word; ([0-9]+) wording-only and ([0-9]+) location-only differences are agreement in substance and are not counted as gaps; ([0-9]+) more we report as a warning and ([0-9]+) elsewhere in the file")
	docScopeAgreementPattern  = regexp.MustCompile(`^- \*\*Scope agreement:\*\* ([0-9]+) of ([0-9]+) declared scope assertions match exactly`)
	docPermissivenessPattern  = regexp.MustCompile(`^- \*\*Permissiveness gaps:\*\* of ([0-9]+) invalid models we wrote ourselves, the reference rejects ([0-9]+) that we accept by default, and ([0-9]+) both reject; ([0-9]+) further cases agree only when we are asked strictly`)
	docErrataPattern          = regexp.MustCompile(`^- \*\*Declared errata:\*\* the registry declares ([0-9]+) defect\(s\) in the published reference material — ([0-9]+) with a specification-derived correction, ([0-9]+) documented without one, since no intended reading can be inferred \(\[OMG issues\]\([^)]+\), ` + "`tools/oracle/errata`" + `\)\. Every figure above is as published and stays the conformance statement; running the same oracles over the corrected text instead reports ([0-9]+) of ([0-9]+) files agreeing, ([0-9]+) diagnostics ours alone and ([0-9]+) the reference's alone, ([0-9]+) declared rows we are silent on, and ([0-9]+) of ([0-9]+) authored cases the reference alone rejects\.`)
	docRejectionLinePattern   = regexp.MustCompile(`^\*\*Rejection oracle:\*\* the reverse direction — do we reject what the reference rejects\? ([0-9]+) hand-written invalid models validated by both implementations, ([0-9]+) rejected by both, ([0-9]+) the pinned pilot rejects and we accept;`)
)

// TestPilotDifferentialDocumentCountsMatchBaseline guards headline totals, Results cells and
// root rows, per-category only-ours/only-pilot prose, and the movement table's Now column.
// The committed baseline JSON is the only input; validators and corpora are unnecessary.
// Causal claims, attributions, historical movement columns and adjudication-section counts are
// out of scope: this checks numbers, not why they moved. It also guards the five refereed
// headline numbers the README and architecture guide lead with, each against the
// baseline of the harness that measured it. The compliance map's own census is counted
// when the documentation site is built, so no committed line states it; only a 🚧 row,
// which that census refuses, is guarded here.
func TestPilotDifferentialDocumentCountsMatchBaseline(t *testing.T) {
	lines := docReadNumberedDocument(t)
	report := docReadBaselineReport(t)

	heading := docRequireResultsHeading(t, lines)
	docAssertHeadline(t, heading, report)

	results := docRequireTable(t, lines, heading.number+1, "")
	docAssertResultsTable(t, results, report)

	categoryStart := docRequireLineContaining(t, lines, "Per category, the only-ours totals are:")
	docAssertCategoryProse(t, lines, categoryStart, report)

	movementStart := docRequireLineContaining(t, lines, "Where the current counts stand")
	movement := docRequireTable(t, lines, movementStart.number+1, "Count")
	docAssertMovementTable(t, movement, report)

	roundStart := docRequireLineContaining(t, lines, "### Feature-initialization round")
	round := docRequireTable(t, lines, roundStart.number+1, "Count")
	docAssertMovementTable(t, round, report)

	readmeLines := docReadNumberedFile(t, docCountReadmePath)
	architectureLines := docReadNumberedFile(t, docCountArchitecturePath)
	ruleCounts := docReadSpecComplianceCounts(t)
	refereed := docReadRefereedCounts(t)
	docAssertReferenceLine(t, docRequireLineContainingPath(t, readmeLines, docCountReadmePath, docCountReferenceMarker), report)
	docAssertRejectionLine(t, docRequireLineContainingPath(t, readmeLines, docCountReadmePath, docCountRejectionMarker), refereed)
	docAssertRefereedHeadline(t, docCountReadmePath, readmeLines, refereed)
	docAssertRefereedHeadline(t, docCountArchitecturePath, architectureLines, refereed)
	if ruleCounts.Total == 0 {
		docFailPathAt(t, docCountSpecCompliancePath, 1, "no rule rows to count")
	}
	if ruleCounts.KnownFailure != 0 {
		docFailPathAt(t, docCountSpecCompliancePath, 1, "%d 🚧 rows have no place in the census; give them a status it states", ruleCounts.KnownFailure)
	}
}

func TestDifferentialBaselineDecodersAgree(t *testing.T) {
	report := docReadBaselineReport(t)
	counts, err := doccounts.ReadRefereedCounts("../../..")
	if err != nil {
		t.Fatalf("read refereed baselines: %v", err)
	}
	if counts.Files != report.Totals.Files ||
		counts.FilesAgreeing != report.Totals.FilesAgreeing ||
		counts.OursOnly != report.Totals.OpenSysMLOnly ||
		counts.PilotOnly != report.Totals.PilotOnly {
		t.Fatalf("differential decoder mismatch: doccounts=%+v report=%+v", counts, report.Totals)
	}
}

func docReadNumberedDocument(t *testing.T) []docLine {
	t.Helper()
	return docReadNumberedFile(t, docCountPath)
}

func docReadNumberedFile(t *testing.T, path string) []docLine {
	t.Helper()
	content, err := os.ReadFile(filepath.FromSlash("../../../" + path))
	if err != nil {
		t.Fatalf("%s:1: read document: %v", path, err)
	}
	raw := strings.Split(string(content), "\n")
	lines := make([]docLine, len(raw))
	for i, text := range raw {
		lines[i] = docLine{number: i + 1, text: text}
	}
	return lines
}

func docReadBaselineReport(t *testing.T) Report {
	t.Helper()
	content, err := os.ReadFile(filepath.FromSlash("../../../" + docCountBaselinePath))
	if err != nil {
		t.Fatalf("%s:1: read baseline: %v", docCountBaselinePath, err)
	}
	var report Report
	if err := json.Unmarshal(content, &report); err != nil {
		t.Fatalf("%s:1: parse baseline: %v", docCountBaselinePath, err)
	}
	return report
}

// docReadSpecComplianceCounts reads the row census through the same package the
// regenerator writes the derived lines from, so the two cannot disagree.
func docReadSpecComplianceCounts(t *testing.T) doccounts.RuleCounts {
	t.Helper()
	content, err := os.ReadFile(filepath.FromSlash("../../../" + docCountSpecCompliancePath))
	if err != nil {
		t.Fatalf("%s:1: read document: %v", docCountSpecCompliancePath, err)
	}
	return doccounts.CountRules(string(content))
}

func docReadRefereedCounts(t *testing.T) doccounts.RefereedCounts {
	t.Helper()
	counts, err := doccounts.ReadRefereedCounts("../../..")
	if err != nil {
		t.Fatalf("read refereed baselines: %v", err)
	}
	return counts
}

// docAssertRefereedHeadline checks the five refereed numbers against the baselines
// they are read from.
func docAssertRefereedHeadline(t *testing.T, path string, lines []docLine, counts doccounts.RefereedCounts) {
	t.Helper()
	checks := []struct {
		pattern *regexp.Regexp
		marker  string
		want    []int
		labels  []string
	}{{
		docCorpusAgreementPattern, "**Corpus agreement:**",
		[]int{counts.FilesAgreeing, counts.Files, counts.OursOnly, counts.PilotOnly},
		[]string{"files agreeing", "files compared", "diagnostics only ours", "diagnostics only the pilot's"},
	}, {
		docDeclaredSilencePattern, "**Declared-diagnostic silence:**",
		[]int{counts.DeclaredErrors, counts.Silent, counts.DeclaredAgree, counts.WordingOnly, counts.LocationOnly, counts.SeverityDiffers, counts.Elsewhere},
		[]string{"declared error rows", "rows we are silent on", "rows agreeing word-for-word", "wording-only rows", "location-only rows", "severity-differs rows", "elsewhere-in-file rows"},
	}, {
		docScopeAgreementPattern, "**Scope agreement:**",
		[]int{counts.ScopeExact, counts.ScopeTotal},
		[]string{"scope assertions agreeing", "scope assertions"},
	}, {
		docPermissivenessPattern, "**Permissiveness gaps:**",
		[]int{counts.RejectCases, counts.RejectDefaultPilotOnly, counts.RejectDefaultBoth, counts.RejectStrictOnly},
		[]string{"authored cases", "cases only the pilot rejects by default", "cases both reject by default", "strict-only agreements"},
	}, {
		docErrataPattern, "**Declared errata:**",
		[]int{counts.Errata.Registry, counts.Errata.Corrections, counts.Errata.Documented,
			counts.Errata.FilesAgreeing, counts.Errata.Files, counts.Errata.OursOnly, counts.Errata.PilotOnly,
			counts.Errata.Silent, counts.Errata.RejectPilotOnly, counts.Errata.RejectCases},
		[]string{"registry entries", "corrections", "documented without a correction",
			"files agreeing with the errata", "files compared with the errata",
			"diagnostics only ours with the errata", "diagnostics only the pilot's with the errata",
			"declared rows we are silent on with the errata",
			"cases only the pilot rejects with the errata", "authored cases with the errata"},
	}}
	for _, check := range checks {
		line := docRequireLineContainingPath(t, lines, path, check.marker)
		match := check.pattern.FindStringSubmatchIndex(line.text)
		if match == nil {
			docFailPathAt(t, path, line.number, "malformed %s line", check.marker)
		}
		var consumed []docNumber
		for i, want := range check.want {
			text := line.text[match[2+i*2]:match[3+i*2]]
			got, err := strconv.Atoi(text)
			if err != nil {
				docFailPathAt(t, path, line.number, "%s %s: malformed number %q", check.marker, check.labels[i], text)
			}
			if got != want {
				docErrorPathAt(t, path, line.number, "%s %s: want %d (baseline), got %d", check.marker, check.labels[i], want, got)
			}
			consumed = append(consumed, docNumbersInRange(line, match[2+i*2], match[3+i*2])...)
		}
		docAssertBareNumbersConsumed(t, path, line, consumed, check.marker+" line")
	}
}

func docAssertRejectionLine(t *testing.T, line docLine, counts doccounts.RefereedCounts) {
	t.Helper()
	match := docRejectionLinePattern.FindStringSubmatchIndex(line.text)
	if match == nil {
		docFailPathAt(t, docCountReadmePath, line.number, "malformed Rejection oracle line")
	}
	wants := []int{counts.RejectCases, counts.RejectBoth, counts.RejectPilotOnly}
	labels := []string{"cases", "rejected by both", "only the pilot rejects"}
	for i, want := range wants {
		text := line.text[match[2+i*2]:match[3+i*2]]
		got, err := strconv.Atoi(text)
		if err != nil {
			docFailPathAt(t, docCountReadmePath, line.number, "Rejection oracle %s: malformed number %q", labels[i], text)
		}
		if got != want {
			docErrorPathAt(t, docCountReadmePath, line.number, "Rejection oracle %s: want %d (baseline totals), got %d", labels[i], want, got)
		}
	}
}

func docAssertReferenceLine(t *testing.T, line docLine, report Report) {
	t.Helper()
	match := docReferencePattern.FindStringSubmatchIndex(line.text)
	if match == nil {
		docFailPathAt(t, docCountReadmePath, line.number, "malformed Reference differential line")
	}
	consumed := append(
		docNumbersInRange(line, match[2], match[3]),
		docNumbersInRange(line, match[4], match[5])...,
	)
	consumed = append(consumed, docNumbersInRange(line, match[6], match[7])...)

	gotFiles, err := strconv.Atoi(line.text[match[2]:match[3]])
	if err != nil {
		docFailPathAt(t, docCountReadmePath, line.number, "Reference differential files: malformed number %q", line.text[match[2]:match[3]])
	}
	if gotFiles != report.Totals.Files {
		docErrorPathAt(t, docCountReadmePath, line.number, "Reference differential files: want %d (baseline totals.files), got %d", report.Totals.Files, gotFiles)
	}
	wantRelease := report.Pilot
	if before, _, ok := strings.Cut(report.Pilot, " ("); ok {
		wantRelease = before
	}
	gotRelease := line.text[match[4]:match[5]]
	if gotRelease != wantRelease {
		docErrorPathAt(t, docCountReadmePath, line.number, "Reference differential release: want %q (baseline pilotRelease), got %q", wantRelease, gotRelease)
	}
	gotAgreement, err := strconv.Atoi(line.text[match[6]:match[7]])
	if err != nil {
		docFailPathAt(t, docCountReadmePath, line.number, "Reference differential fully agreeing: malformed number %q", line.text[match[6]:match[7]])
	}
	if gotAgreement != report.Totals.FilesAgreeing {
		docErrorPathAt(t, docCountReadmePath, line.number, "Reference differential fully agreeing: want %d (baseline totals.filesFullyAgreeing), got %d", report.Totals.FilesAgreeing, gotAgreement)
	}
	docAssertBareNumbersConsumed(t, docCountReadmePath, line, consumed, "Reference differential line")
}

func docNumbersInRange(line docLine, start, end int) []docNumber {
	tokens := docBareNumberSpans(line.text[start:end])
	for i := range tokens {
		tokens[i].start += start
		tokens[i].end += start
		tokens[i].line = line.number
	}
	return tokens
}

func docAssertBareNumbersConsumed(t *testing.T, path string, line docLine, consumed []docNumber, context string) {
	t.Helper()
	for _, token := range docBareNumberTokens(line.text, []int{0}, []docLine{line}) {
		if !docNumberWasConsumed(consumed, token) {
			docErrorPathAt(t, path, token.line, "unaccounted number %q in %s", token.text, context)
		}
	}
}

func docRequireResultsHeading(t *testing.T, lines []docLine) docLine {
	t.Helper()
	for _, line := range lines {
		if strings.HasPrefix(line.text, "## Results (pilot ") {
			return line
		}
	}
	docFailAt(t, 1, "missing Results heading")
	return docLine{}
}

func docAssertHeadline(t *testing.T, heading docLine, report Report) {
	t.Helper()
	match := docHeadlinePattern.FindStringSubmatch(heading.text)
	if match == nil {
		docFailAt(t, heading.number, "malformed Results heading")
	}
	wantRelease := report.Pilot
	if before, _, ok := strings.Cut(report.Pilot, " ("); ok {
		wantRelease = before
	}
	if got := match[1]; got != wantRelease {
		docErrorAt(t, heading.number, "headline release: want %q (baseline pilotRelease), got %q", wantRelease, got)
	}
	gotFiles, err := strconv.Atoi(match[2])
	if err != nil {
		docFailAt(t, heading.number, "headline files: malformed number %q", match[2])
	}
	if gotFiles != report.Totals.Files {
		docErrorAt(t, heading.number, "headline files: want %d (baseline totals.files), got %d", report.Totals.Files, gotFiles)
	}
}

func docRequireLineContaining(t *testing.T, lines []docLine, marker string) docLine {
	t.Helper()
	return docRequireLineContainingPath(t, lines, docCountPath, marker)
}

func docRequireLineContainingPath(t *testing.T, lines []docLine, path, marker string) docLine {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line.text, marker) {
			return line
		}
	}
	docFailPathAt(t, path, 1, "missing required section marker %q", marker)
	return docLine{}
}

func docRequireTable(t *testing.T, lines []docLine, from int, firstHeader string) docTable {
	t.Helper()
	for i := from - 1; i < len(lines); i++ {
		if !docIsTableLine(lines[i].text) {
			continue
		}
		header := docSplitTableCells(lines[i].text)
		if firstHeader != "" && (len(header) == 0 || header[0] != firstHeader) {
			continue
		}
		rows := make([]docTableRow, 0)
		for j := i + 1; j < len(lines) && docIsTableLine(lines[j].text); j++ {
			rows = append(rows, docTableRow{line: lines[j].number, cells: docSplitTableCells(lines[j].text)})
		}
		return docTable{header: header, headerLine: lines[i].number, rows: rows}
	}
	if firstHeader == "" {
		docFailAt(t, from, "missing Results table")
	} else {
		docFailAt(t, from, "missing movement table with first header %q", firstHeader)
	}
	return docTable{}
}

func docIsTableLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

func docSplitTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func docIsSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(cell)
		if len(cell) < 3 || strings.Trim(cell, ":-") != "" {
			return false
		}
	}
	return true
}

func docAssertResultsTable(t *testing.T, results docTable, report Report) {
	t.Helper()
	wantHeader := []string{"Root", "Files", "Fully agreeing", "Ours", "Pilot", "Agreed", "Severity-only", "Only ours", "Only pilot"}
	if len(results.header) != len(wantHeader) {
		docFailAt(t, results.headerLine, "Results table header: want %q, got %q", wantHeader, results.header)
	}
	for i := range wantHeader {
		if results.header[i] != wantHeader[i] {
			docFailAt(t, results.headerLine, "Results table header column %d: want %q, got %q", i+1, wantHeader[i], results.header[i])
		}
	}

	rootByDir := make(map[string]RootReport, len(report.Roots))
	for _, root := range report.Roots {
		rootByDir[root.Dir] = root
	}
	foundDirs := make(map[string]bool)
	totalSeen := false
	for _, row := range results.rows {
		if docIsSeparatorRow(row.cells) {
			continue
		}
		if len(row.cells) != len(wantHeader) {
			docFailAt(t, row.line, "Results table row has %d cells, want %d", len(row.cells), len(wantHeader))
		}
		label := docStripMarkdown(row.cells[0])
		label = docParentheticalPattern.ReplaceAllString(label, "")
		if label == "Total" {
			if totalSeen {
				docFailAt(t, row.line, "Results table total row appears more than once")
			}
			totalSeen = true
			docAssertTotalsRow(t, row, report.Totals)
			continue
		}
		root, ok := rootByDir[label]
		if !ok {
			docFailAt(t, row.line, "Results table root %q is not in baseline roots", label)
		}
		if foundDirs[label] {
			docFailAt(t, row.line, "Results table root %q appears more than once", label)
		}
		foundDirs[label] = true
		docAssertTotalsCells(t, row, root.Totals, "roots["+root.Name+"].totals", "root "+root.Dir)
	}
	if !totalSeen {
		docFailAt(t, results.headerLine, "Results table is missing the Total row")
	}
	wantDirs := make([]string, 0, len(report.Roots))
	for _, root := range report.Roots {
		wantDirs = append(wantDirs, root.Dir)
	}
	gotDirs := make([]string, 0, len(foundDirs))
	for dir := range foundDirs {
		gotDirs = append(gotDirs, dir)
	}
	sort.Strings(wantDirs)
	sort.Strings(gotDirs)
	if strings.Join(wantDirs, "\x00") != strings.Join(gotDirs, "\x00") {
		docErrorAt(t, results.headerLine, "Results table root set: want %q (baseline roots[].dir), got %q", wantDirs, gotDirs)
	}
}

func docAssertTotalsRow(t *testing.T, row docTableRow, totals Totals) {
	t.Helper()
	docAssertTotalsCells(t, row, totals, "totals", "root Total")
}

func docAssertTotalsCells(t *testing.T, row docTableRow, totals Totals, jsonPrefix, subject string) {
	t.Helper()
	columns := []struct {
		header string
		want   int
		field  string
	}{
		{"Files", totals.Files, "files"},
		{"Fully agreeing", totals.FilesAgreeing, "filesFullyAgreeing"},
		{"Ours", totals.OpenSysMLTotal, "openSysMLDiagnostics"},
		{"Pilot", totals.PilotTotal, "pilotDiagnostics"},
		{"Agreed", totals.Agreement, "agreement"},
		{"Severity-only", totals.SeverityMismatch, "severityMismatch"},
		{"Only ours", totals.OpenSysMLOnly, "openSysMLOnly"},
		{"Only pilot", totals.PilotOnly, "pilotOnly"},
	}
	for i, column := range columns {
		got := docParseCellInteger(t, row, i+1, column.header)
		if got != column.want {
			docErrorAt(t, row.line, `%s column %q: want %d (baseline %s.%s), got %d`, subject, column.header, column.want, jsonPrefix, column.field, got)
		}
	}
}

func docParseCellInteger(t *testing.T, row docTableRow, cell int, label string) int {
	t.Helper()
	value := docStripMarkdown(row.cells[cell])
	if !docIntegerPattern.MatchString(value) {
		docFailAt(t, row.line, "column %q: expected integer, got %q", label, value)
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		docFailAt(t, row.line, "column %q: malformed integer %q", label, value)
	}
	return got
}

func docStripMarkdown(value string) string {
	value = strings.ReplaceAll(value, "**", "")
	value = strings.ReplaceAll(value, "`", "")
	return strings.TrimSpace(value)
}

func docAssertCategoryProse(t *testing.T, lines []docLine, start docLine, report Report) {
	t.Helper()
	paragraphLines := []docLine{start}
	for i := start.number; i < len(lines) && strings.TrimSpace(lines[i].text) != ""; i++ {
		paragraphLines = append(paragraphLines, lines[i])
	}
	text := make([]string, len(paragraphLines))
	for i, line := range paragraphLines {
		text[i] = line.text
	}
	paragraph := strings.Join(text, "\n")
	onlyOursMarker := "only-ours totals are:"
	onlyPilotMarker := "Only-pilot:"
	oursAt := strings.Index(paragraph, onlyOursMarker)
	pilotAt := strings.Index(paragraph, onlyPilotMarker)
	if oursAt < 0 {
		docFailAt(t, start.number, "category paragraph is missing marker %q", onlyOursMarker)
	}
	if pilotAt < 0 {
		docFailAt(t, start.number, "category paragraph is missing marker %q", onlyPilotMarker)
	}
	if pilotAt <= oursAt+len(onlyOursMarker) {
		docFailAt(t, start.number, "category paragraph markers are out of order")
	}

	parse := &docCategoryProse{
		t:          t,
		paragraph:  paragraph,
		starts:     docParagraphLineStarts(paragraph),
		lines:      paragraphLines,
		report:     report,
		prose:      map[string]map[string]map[Category]int{},
		proseLines: map[string]map[string]int{},
	}
	parse.parsePart(oursAt+len(onlyOursMarker), pilotAt, "only-ours")
	parse.parsePart(pilotAt+len(onlyPilotMarker), len(paragraph), "only-pilot")

	docAssertCategoryMaps(t, report, parse.prose, parse.proseLines)
	for _, token := range docBareNumberTokens(paragraph, parse.starts, paragraphLines) {
		if !docNumberWasConsumed(parse.consumed, token) {
			docErrorAt(t, token.line, "unaccounted number %q in category paragraph", token.text)
		}
	}
}

// docCategoryProse reads the category paragraph into per-direction, per-root
// category counts, recording the numbers it consumed so a leftover one can be
// reported.
type docCategoryProse struct {
	t          *testing.T
	paragraph  string
	starts     []int
	lines      []docLine
	report     Report
	prose      map[string]map[string]map[Category]int
	proseLines map[string]map[string]int
	consumed   []docNumber
}

// parsePart reads the semicolon-separated segments of one direction's half of
// the paragraph.
func (p *docCategoryProse) parsePart(from, to int, direction string) {
	p.t.Helper()
	paragraph := p.paragraph
	part := paragraph[from:to]
	partOffset := from
	for len(part) > 0 {
		semicolon := strings.IndexByte(part, ';')
		rawSegment := part
		if semicolon >= 0 {
			rawSegment = part[:semicolon]
		}
		leading := len(rawSegment) - len(strings.TrimLeft(rawSegment, " \t\n"))
		segmentStart := partOffset + leading
		segment := strings.TrimSpace(rawSegment)
		if segment == "" {
			if semicolon < 0 {
				break
			}
			partOffset += semicolon + 1
			part = paragraph[partOffset:to]
			continue
		}
		p.parseSegment(segment, segmentStart, direction)
		if semicolon < 0 {
			break
		}
		partOffset += semicolon + 1
		part = paragraph[partOffset:to]
	}
}

// parseSegment reads one root's counts within a direction's half.
func (p *docCategoryProse) parseSegment(segment string, segmentStart int, direction string) {
	t, starts, lines := p.t, p.starts, p.lines
	report, prose, proseLines := p.report, p.prose, p.proseLines
	t.Helper()
	rootMatch := docRootPattern.FindStringSubmatchIndex(segment)
	if rootMatch == nil {
		if tokens := docBareNumberSpans(segment); len(tokens) > 0 {
			token := tokens[0]
			docErrorAt(t, docLineAt(starts, lines, segmentStart+token.start), "unaccounted number %q in category paragraph", token.text)
		}
		docFailAt(t, docLineAt(starts, lines, segmentStart), "category %s segment must start with a backticked root name: %q", direction, segment)
	}
	rootName := segment[rootMatch[2]:rootMatch[3]]
	root, ok := docRootByName(report, rootName)
	if !ok {
		docFailAt(t, docLineAt(starts, lines, segmentStart+rootMatch[2]), "category %s names unknown root %q", direction, rootName)
	}
	restStart := segmentStart + rootMatch[4]
	rest := segment[rootMatch[4]:rootMatch[5]]
	if prose[direction] == nil {
		prose[direction] = map[string]map[Category]int{}
		proseLines[direction] = map[string]int{}
	}
	if prose[direction][rootName] == nil {
		prose[direction][rootName] = map[Category]int{}
	}
	if _, exists := proseLines[direction][rootName]; !exists {
		proseLines[direction][rootName] = docLineAt(starts, lines, segmentStart+rootMatch[2])
	}

	knownCategories := docReportCategories(report)
	parsed := 0
	for {
		leading := len(rest) - len(strings.TrimLeft(rest, " \t\n"))
		rest = rest[leading:]
		restStart += leading
		match := docCategoryItemPattern.FindStringSubmatchIndex(rest)
		if match == nil {
			if parsed == 0 {
				docFailAt(t, docLineAt(starts, lines, restStart), "category %s root %q has no count/category items", direction, root.Name)
			}
			break
		}
		categoryText := strings.Trim(rest[match[4]:match[5]], "`")
		category := Category(categoryText)
		if !knownCategories[category] {
			docFailAt(t, docLineAt(starts, lines, restStart+match[4]), "category %s root %q names unknown category %q", direction, root.Name, categoryText)
		}
		count, err := strconv.Atoi(rest[match[2]:match[3]])
		if err != nil {
			docFailAt(t, docLineAt(starts, lines, restStart), "category %s root %q has malformed count", direction, root.Name)
		}
		numberStart := restStart + match[2]
		numberEnd := restStart + match[3]
		boundary := docNextCategoryPattern.FindStringIndex(rest[match[1]:])
		tailEnd := len(rest)
		if boundary != nil {
			tailEnd = match[1] + boundary[0]
		}
		tail := rest[match[1]:tailEnd]
		if tokens := docBareNumberSpans(tail); len(tokens) > 0 {
			token := tokens[0]
			docErrorAt(t, docLineAt(starts, lines, restStart+match[1]+token.start), "unaccounted number %q in %s %s tail", token.text, direction, root.Name)
		}
		prose[direction][rootName][category] += count
		p.consumed = append(p.consumed, docNumber{start: numberStart, end: numberEnd, text: rest[match[2]:match[3]], line: docLineAt(starts, lines, numberStart)})
		parsed++
		if boundary == nil {
			break
		}
		nextStart := match[1] + boundary[0] + 1
		restStart += nextStart
		rest = rest[nextStart:]
	}
}

func docAssertCategoryMaps(t *testing.T, report Report, prose map[string]map[string]map[Category]int, proseLines map[string]map[string]int) {
	t.Helper()
	for _, direction := range []string{"only-ours", "only-pilot"} {
		for rootName, categories := range prose[direction] {
			root, ok := docRootByName(report, rootName)
			if !ok {
				docFailAt(t, proseLines[direction][rootName], "root %s %s is not present in baseline roots", rootName, direction)
			}
			for category, got := range categories {
				want := docCategoryTotal(root, direction, category)
				if want == 0 || got != want {
					docErrorAt(t, proseLines[direction][rootName], "root %s %s category %q: want %d (baseline roots[%s].files[].%s[].count), got %d", root.Name, direction, category, want, root.Name, docDirectionJSONField(direction), got)
				}
			}
		}
		for _, root := range report.Roots {
			for category, want := range docCategoryTotals(root, direction) {
				if want == 0 {
					continue
				}
				if _, present := prose[direction][root.Name][category]; present {
					continue
				}
				got := 0
				line := proseLines[direction][root.Name]
				if line == 0 {
					line = 1
				}
				docErrorAt(t, line, "root %s %s category %q: want %d (baseline roots[%s].files[].%s[].count), got %d (missing from prose)", root.Name, direction, category, want, root.Name, docDirectionJSONField(direction), got)
			}
		}
	}
}

func docRootByName(report Report, name string) (RootReport, bool) {
	for _, root := range report.Roots {
		if root.Name == name {
			return root, true
		}
	}
	return RootReport{}, false
}

func docReportCategories(report Report) map[Category]bool {
	categories := map[Category]bool{}
	for _, root := range report.Roots {
		for _, file := range root.Files {
			for _, entry := range file.OpenSysMLOnly {
				categories[entry.Category] = true
			}
			for _, entry := range file.PilotOnly {
				categories[entry.Category] = true
			}
		}
	}
	return categories
}

func docCategoryTotals(root RootReport, direction string) map[Category]int {
	totals := map[Category]int{}
	for _, file := range root.Files {
		var entries []Entry
		if direction == "only-ours" {
			entries = file.OpenSysMLOnly
		} else {
			entries = file.PilotOnly
		}
		for _, entry := range entries {
			totals[entry.Category] += entry.Count
		}
	}
	return totals
}

func docCategoryTotal(root RootReport, direction string, category Category) int {
	return docCategoryTotals(root, direction)[category]
}

func docDirectionJSONField(direction string) string {
	if direction == "only-ours" {
		return "openSysMLOnly"
	}
	return "pilotOnly"
}

func docParagraphLineStarts(paragraph string) []int {
	starts := []int{0}
	for i, char := range paragraph {
		if char == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func docLineAt(starts []int, lines []docLine, offset int) int {
	line := 0
	for i, start := range starts {
		if start > offset {
			break
		}
		line = i
	}
	return lines[line].number
}

func docBareNumberSpans(text string) []docNumber {
	var tokens []docNumber
	for i := 0; i < len(text); {
		if text[i] < '0' || text[i] > '9' || (i > 0 && docIsWordBefore(text, i)) {
			i++
			continue
		}
		j := i + 1
		for j < len(text) && text[j] >= '0' && text[j] <= '9' {
			j++
		}
		if j == len(text) || !docIsWordAt(text, j) {
			tokens = append(tokens, docNumber{start: i, end: j, text: text[i:j]})
		}
		i = j
	}
	return tokens
}

func docBareNumberTokens(text string, starts []int, lines []docLine) []docNumber {
	tokens := docBareNumberSpans(text)
	for i := range tokens {
		tokens[i].line = docLineAt(starts, lines, tokens[i].start)
	}
	return tokens
}

func docNumberWasConsumed(consumed []docNumber, token docNumber) bool {
	for _, item := range consumed {
		if item.start == token.start && item.end == token.end {
			return true
		}
	}
	return false
}

func docIsWordBefore(text string, offset int) bool {
	_, size := utf8.DecodeLastRuneInString(text[:offset])
	r, _ := utf8.DecodeLastRuneInString(text[offset-size : offset])
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func docIsWordAt(text string, offset int) bool {
	r, _ := utf8.DecodeRuneInString(text[offset:])
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func docAssertMovementTable(t *testing.T, movement docTable, report Report) {
	t.Helper()
	now := -1
	for i, header := range movement.header {
		if header == "Now" {
			now = i
			break
		}
	}
	if now < 0 {
		docFailAt(t, movement.headerLine, "movement table is missing a Now column")
	}
	if len(movement.header) == 0 || movement.header[0] != "Count" {
		docFailAt(t, movement.headerLine, "movement table first header: want %q, got %q", "Count", movement.header)
	}
	seen := map[string]bool{}
	for _, row := range movement.rows {
		if docIsSeparatorRow(row.cells) {
			continue
		}
		if len(row.cells) <= now {
			docFailAt(t, row.line, "movement table row has %d cells, missing Now column", len(row.cells))
		}
		label := docStripMarkdown(row.cells[0])
		if seen[label] {
			docFailAt(t, row.line, "movement table row %q appears more than once", label)
		}
		seen[label] = true
		if docMovementRowsUnchecked[label] {
			continue
		}
		if label == "overall: fully agreeing / only ours / our diagnostics" {
			values := strings.Split(docStripMarkdown(row.cells[now]), "/")
			if len(values) != 3 {
				docFailAt(t, row.line, "movement row %q column %q: want three slash-separated counts, got %q", label, movement.header[now], row.cells[now])
			}
			wants := []struct {
				value int
				path  string
			}{
				{report.Totals.FilesAgreeing, "totals.filesFullyAgreeing"},
				{report.Totals.OpenSysMLOnly, "totals.openSysMLOnly"},
				{report.Totals.OpenSysMLTotal, "totals.openSysMLDiagnostics"},
			}
			for i, want := range wants {
				got := docParseMovementInteger(t, row.line, strings.TrimSpace(values[i]), movement.header[now])
				if got != want.value {
					docErrorAt(t, row.line, "movement row %q item %d column %q: want %d (baseline %s), got %d", label, i+1, movement.header[now], want.value, want.path, got)
				}
			}
			continue
		}
		want, jsonPath, ok := docMovementValue(report, label)
		if !ok {
			docFailAt(t, row.line, "movement table row %q is not mapped or allowlisted", label)
		}
		got := docParseCellInteger(t, docTableRow{line: row.line, cells: row.cells}, now, "Now")
		if got != want {
			docErrorAt(t, row.line, "movement row %q column %q: want %d (baseline %s), got %d", label, movement.header[now], want, jsonPath, got)
		}
	}
}

func docParseMovementInteger(t *testing.T, line int, value, column string) int {
	t.Helper()
	if !docIntegerPattern.MatchString(value) {
		docFailAt(t, line, "column %q: expected integer, got %q", column, value)
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		docFailAt(t, line, "column %q: malformed integer %q", column, value)
	}
	return got
}

func docMovementValue(report Report, label string) (int, string, bool) {
	switch label {
	case "only pilot":
		return report.Totals.PilotOnly, "totals.pilotOnly", true
	case "pilot diagnostics":
		return report.Totals.PilotTotal, "totals.pilotDiagnostics", true
	case "severity-only":
		return report.Totals.SeverityMismatch, "totals.severityMismatch", true
	case "unmapped, our side":
		total := 0
		for _, row := range report.Unmapped {
			if row.Side == "opensysml" {
				total += row.Count
			}
		}
		return total, "unmapped[side=opensysml].count", true
	}
	for _, root := range report.Roots {
		prefix := root.Name + ": "
		if !strings.HasPrefix(label, prefix) {
			continue
		}
		switch strings.TrimPrefix(label, prefix) {
		case "only ours":
			return root.Totals.OpenSysMLOnly, "roots[" + root.Name + "].totals.openSysMLOnly", true
		case "only pilot":
			return root.Totals.PilotOnly, "roots[" + root.Name + "].totals.pilotOnly", true
		case "fully agreeing":
			return root.Totals.FilesAgreeing, "roots[" + root.Name + "].totals.filesFullyAgreeing", true
		}
	}
	return 0, "", false
}

func docFailAt(t *testing.T, line int, format string, args ...any) {
	t.Helper()
	docFailPathAt(t, docCountPath, line, format, args...)
}

func docErrorAt(t *testing.T, line int, format string, args ...any) {
	t.Helper()
	docErrorPathAt(t, docCountPath, line, format, args...)
}

func docFailPathAt(t *testing.T, path string, line int, format string, args ...any) {
	t.Helper()
	t.Fatalf("%s:%d: %s", path, line, fmt.Sprintf(format, args...))
}

func docErrorPathAt(t *testing.T, path string, line int, format string, args ...any) {
	t.Helper()
	t.Errorf("%s:%d: %s", path, line, fmt.Sprintf(format, args...))
}
