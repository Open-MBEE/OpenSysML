package diff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

const (
	w6fSkillsRoot        = ".agents/skills"
	w6fXpectDocPath      = "docs/project/pilot-xpect.md"
	w6fXpectBaselinePath = "docs/project/pilot-xpect-baseline.json"
)

var (
	w6fSkillHeadlinePattern = regexp.MustCompile(`(\d+) file\(s\), (\d+) fully agreeing; (\d+) agreed(?: diagnostic\(s\))?, (\d+) only ours, (\d+) only the pilot's`)
	w6fSkillTotalsPattern   = regexp.MustCompile(`openSysMLDiagnostics (\d+) / pilotDiagnostics (\d+) / severityMismatch (\d+)`)
	w6fHistoricalPattern    = regexp.MustCompile(`<!-- doc-count:historical(?:: [^>]+)? -->`)
	w6fXpectTotalsFiles     = regexp.MustCompile(`^(\d+) \.xt file\(s\), (\d+) unparsed, (\d+) missing declared resource\(s\)$`)
	w6fXpectTotalsRows      = regexp.MustCompile(`^(\d+) assertion\(s\) declaring (\d+) expectation\(s\)$`)
	w6fXpectTotalsVerdicts  = regexp.MustCompile(`^agree (\d+) \(of which wording-only (\d+)\) \| disagree (\d+) \| unlocated (\d+) \| not adjudicated (\d+)$`)
	w6fXpectCensusFiles     = regexp.MustCompile(`fetches \*\*(\d+) KerML \+ (\d+) SysML = (\d+)\*\*`)
	w6fXpectCensusRows      = regexp.MustCompile(`recovers \*\*(\d+) assertions\*\*, declaring \*\*(\d+) individual expectations\*\*`)
)

type w6fSkillFlattened struct {
	text    string
	lines   []int
	offsets []int
}

type w6fSkillNumber struct {
	value  int
	offset int
}

type w6fSkillField struct {
	label string
	path  string
}

type w6fSkillMatch struct {
	start   int
	subject string
	numbers []w6fSkillNumber
	fields  []w6fSkillField
}

type w6fXpectTotals struct {
	Files          int `json:"files"`
	FilesUnparsed  int `json:"filesUnparsed"`
	Assertions     int `json:"assertions"`
	Rows           int `json:"rows"`
	Agree          int `json:"agree"`
	Disagree       int `json:"disagree"`
	WordingOnly    int `json:"wordingOnly"`
	Unlocated      int `json:"unlocated"`
	NotAdjudicated int `json:"notAdjudicated"`
	Missing        int `json:"missingResources"`
}

type w6fXpectKind struct {
	Kind            string `json:"kind"`
	Rows            int    `json:"rows"`
	Agree           int    `json:"agree"`
	WordingOnly     int    `json:"wordingOnly"`
	Disagree        int    `json:"disagree"`
	NotAdjudicated  int    `json:"notAdjudicated"`
	SameLocation    int    `json:"sameLocation"`
	SameLine        int    `json:"sameLine"`
	SeverityDiffers int    `json:"severityDiffers"`
	Elsewhere       int    `json:"elsewhereInFile"`
}

type w6fXpectSuite struct {
	Name   string         `json:"name"`
	Totals w6fXpectTotals `json:"totals"`
}

type w6fXpectReport struct {
	Totals w6fXpectTotals  `json:"totals"`
	Kinds  []w6fXpectKind  `json:"kinds"`
	Suites []w6fXpectSuite `json:"suites"`
}

// TestW6FSkillDocumentCountsMatchBaseline guards every discovered skill claim against
// docs/project/pilot-differential-baseline.json; claims are checked by default, markers exempt the next match, and dangling markers fail.
func TestW6FSkillDocumentCountsMatchBaseline(t *testing.T) {
	report := docReadBaselineReport(t)
	root := filepath.FromSlash("../../../" + w6fSkillsRoot)
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "SKILL.md" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("%s:1: scan skills: %v", w6fSkillsRoot, err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		w6fCheckSkillFile(t, path, report)
	}
}

func w6fCheckSkillFile(t *testing.T, path string, report Report) {
	t.Helper()
	content, err := os.ReadFile(path)
	displayPath := strings.TrimPrefix(filepath.ToSlash(path), "../../../")
	if err != nil {
		t.Fatalf("%s:1: read document: %v", displayPath, err)
	}
	w6fFlat := w6fFlattenSkill(string(content))
	var matches []w6fSkillMatch
	for _, found := range w6fSkillHeadlinePattern.FindAllStringSubmatchIndex(w6fFlat.text, -1) {
		w6fMatch := w6fSkillMatch{
			start:   found[0],
			subject: "differential headline",
			fields: []w6fSkillField{
				{label: "files", path: "totals.files"},
				{label: "fully agreeing", path: "totals.filesFullyAgreeing"},
				{label: "agreement", path: "totals.agreement"},
				{label: "only ours", path: "totals.openSysMLOnly"},
				{label: "only the pilot's", path: "totals.pilotOnly"},
			},
		}
		for i := range w6fMatch.fields {
			value, parseErr := strconv.Atoi(w6fFlat.text[found[2+i*2]:found[3+i*2]])
			if parseErr != nil {
				docFailPathAt(t, displayPath, w6fFlat.lines[found[2+i*2]], "malformed differential headline number %q", w6fFlat.text[found[2+i*2]:found[3+i*2]])
			}
			w6fMatch.numbers = append(w6fMatch.numbers, w6fSkillNumber{value: value, offset: found[2+i*2]})
		}
		matches = append(matches, w6fMatch)
	}
	for _, found := range w6fSkillTotalsPattern.FindAllStringSubmatchIndex(w6fFlat.text, -1) {
		w6fMatch := w6fSkillMatch{
			start:   found[0],
			subject: "JSON totals",
			fields: []w6fSkillField{
				{label: "openSysMLDiagnostics", path: "totals.openSysMLDiagnostics"},
				{label: "pilotDiagnostics", path: "totals.pilotDiagnostics"},
				{label: "severityMismatch", path: "totals.severityMismatch"},
			},
		}
		for i := range w6fMatch.fields {
			value, parseErr := strconv.Atoi(w6fFlat.text[found[2+i*2]:found[3+i*2]])
			if parseErr != nil {
				docFailPathAt(t, displayPath, w6fFlat.lines[found[2+i*2]], "malformed JSON totals number %q", w6fFlat.text[found[2+i*2]:found[3+i*2]])
			}
			w6fMatch.numbers = append(w6fMatch.numbers, w6fSkillNumber{value: value, offset: found[2+i*2]})
		}
		matches = append(matches, w6fMatch)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].start < matches[j].start })
	w6fMarkers := w6fFindHistoricalMarkers(string(content))
	consumed := make([]bool, len(matches))
	// Each marker consumes exactly the next unconsumed live-looking match; unused markers fail.
	for _, marker := range w6fMarkers {
		found := -1
		for i, match := range matches {
			if !consumed[i] && w6fFlat.offsets[match.start] >= marker[1] {
				found = i
				break
			}
		}
		if found < 0 {
			line := strings.Count(string(content)[:marker[0]], "\n") + 1
			docErrorPathAt(t, displayPath, line, "dangling doc-count:historical marker (no live-looking headline follows)")
			continue
		}
		consumed[found] = true
	}
	for i, match := range matches {
		if consumed[i] {
			continue
		}
		for j, number := range match.numbers {
			field := match.fields[j]
			want := w6fSkillReportValue(report, field.path)
			if number.value != want {
				docErrorPathAt(t, displayPath, w6fFlat.lines[number.offset], "%s %q: want %d (baseline %s), got %d", match.subject, field.label, want, field.path, number.value)
			}
		}
	}
}

func w6fFindHistoricalMarkers(content string) [][]int {
	var markers [][]int
	for _, marker := range w6fHistoricalPattern.FindAllStringIndex(content, -1) {
		if !w6fOffsetInMarkdownCodeSpan(content, marker[0]) {
			markers = append(markers, marker)
		}
	}
	return markers
}

func w6fOffsetInMarkdownCodeSpan(content string, offset int) bool {
	for i := 0; i < len(content); {
		if content[i] != '`' {
			i++
			continue
		}
		start := i
		for i < len(content) && content[i] == '`' {
			i++
		}
		run := content[start:i]
		closeOffset := strings.Index(content[i:], run)
		if closeOffset < 0 {
			return offset >= i
		}
		closeStart := i + closeOffset
		if offset >= i && offset < closeStart {
			return true
		}
		i = closeStart + len(run)
	}
	return false
}

func w6fSkillReportValue(report Report, field string) int {
	switch field {
	case "totals.files":
		return report.Totals.Files
	case "totals.filesFullyAgreeing":
		return report.Totals.FilesAgreeing
	case "totals.agreement":
		return report.Totals.Agreement
	case "totals.openSysMLOnly":
		return report.Totals.OpenSysMLOnly
	case "totals.pilotOnly":
		return report.Totals.PilotOnly
	case "totals.openSysMLDiagnostics":
		return report.Totals.OpenSysMLTotal
	case "totals.pilotDiagnostics":
		return report.Totals.PilotTotal
	case "totals.severityMismatch":
		return report.Totals.SeverityMismatch
	default:
		panic("unknown Wave 6F differential field " + field)
	}
}

// w6fFlattenSkill collapses wrapped whitespace so headlines match as one string while retaining
// original line numbers for each flattened offset.
func w6fFlattenSkill(content string) w6fSkillFlattened {
	var text strings.Builder
	var lines []int
	var offsets []int
	line := 1
	for offset := 0; offset < len(content); {
		runeValue, size := utf8.DecodeRuneInString(content[offset:])
		if unicode.IsSpace(runeValue) {
			startOffset := offset
			startLine := line
			for offset < len(content) {
				runeValue, size = utf8.DecodeRuneInString(content[offset:])
				if !unicode.IsSpace(runeValue) {
					break
				}
				if runeValue == '\n' {
					line++
				}
				offset += size
			}
			text.WriteByte(' ')
			lines = append(lines, startLine)
			offsets = append(offsets, startOffset)
			continue
		}
		text.WriteString(content[offset : offset+size])
		for i := 0; i < size; i++ {
			lines = append(lines, line)
			offsets = append(offsets, offset)
		}
		offset += size
	}
	return w6fSkillFlattened{text: text.String(), lines: lines, offsets: offsets}
}

// TestW6FXpectDocumentCountsMatchBaseline guards pilot-xpect.md against docs/project/pilot-xpect-baseline.json.
func TestW6FXpectDocumentCountsMatchBaseline(t *testing.T) {
	report := w6fReadXpectBaseline(t)
	lines := docReadNumberedFile(t, w6fXpectDocPath)
	w6fAssertXpectTotals(t, lines, report)
	w6fAssertXpectKinds(t, lines, report)
	w6fAssertXpectSuites(t, lines, report)
	w6fAssertXpectCensus(t, lines, report)
}

func w6fReadXpectBaseline(t *testing.T) w6fXpectReport {
	t.Helper()
	content, err := os.ReadFile(filepath.FromSlash("../../../" + w6fXpectBaselinePath))
	if err != nil {
		t.Fatalf("%s:1: read baseline: %v", w6fXpectBaselinePath, err)
	}
	var report w6fXpectReport
	if err := json.Unmarshal(content, &report); err != nil {
		t.Fatalf("%s:1: parse baseline: %v", w6fXpectBaselinePath, err)
	}
	return report
}

func w6fAssertXpectTotals(t *testing.T, lines []docLine, report w6fXpectReport) {
	t.Helper()
	start := -1
	for i, line := range lines {
		if line.text == "## Totals" {
			start = i
			break
		}
	}
	if start < 0 {
		docFailPathAt(t, w6fXpectDocPath, 1, "missing Totals heading")
	}
	found := [3]bool{}
	for i := start + 1; i < len(lines) && !strings.HasPrefix(lines[i].text, "## "); i++ {
		switch {
		case w6fXpectTotalsFiles.MatchString(lines[i].text):
			w6fAssertXpectLine(t, lines[i], w6fXpectTotalsFiles, []int{report.Totals.Files, report.Totals.FilesUnparsed, report.Totals.Missing}, []w6fSkillField{{label: "files", path: "totals.files"}, {label: "unparsed", path: "totals.filesUnparsed"}, {label: "missing declared resources", path: "totals.missingResources"}}, "Totals")
			found[0] = true
		case w6fXpectTotalsRows.MatchString(lines[i].text):
			w6fAssertXpectLine(t, lines[i], w6fXpectTotalsRows, []int{report.Totals.Assertions, report.Totals.Rows}, []w6fSkillField{{label: "assertions", path: "totals.assertions"}, {label: "expectations", path: "totals.rows"}}, "Totals")
			found[1] = true
		case w6fXpectTotalsVerdicts.MatchString(lines[i].text):
			w6fAssertXpectLine(t, lines[i], w6fXpectTotalsVerdicts, []int{report.Totals.Agree, report.Totals.WordingOnly, report.Totals.Disagree, report.Totals.Unlocated, report.Totals.NotAdjudicated}, []w6fSkillField{{label: "agree", path: "totals.agree"}, {label: "wording-only", path: "totals.wordingOnly"}, {label: "disagree", path: "totals.disagree"}, {label: "unlocated", path: "totals.unlocated"}, {label: "not adjudicated", path: "totals.notAdjudicated"}}, "Totals")
			found[2] = true
		}
	}
	labels := []string{"files", "assertions", "verdicts"}
	for i, ok := range found {
		if !ok {
			docFailPathAt(t, w6fXpectDocPath, lines[start].number, "Totals code block is missing %s line", labels[i])
		}
	}
}

func w6fAssertXpectLine(t *testing.T, line docLine, pattern *regexp.Regexp, wants []int, fields []w6fSkillField, subject string) {
	t.Helper()
	match := pattern.FindStringSubmatchIndex(line.text)
	if match == nil {
		docFailPathAt(t, w6fXpectDocPath, line.number, "malformed %s line", subject)
	}
	for i, want := range wants {
		got, err := strconv.Atoi(line.text[match[2+i*2]:match[3+i*2]])
		if err != nil {
			docFailPathAt(t, w6fXpectDocPath, line.number, "%s %q: malformed number %q", subject, fields[i].label, line.text[match[2+i*2]:match[3+i*2]])
		}
		if got != want {
			docErrorPathAt(t, w6fXpectDocPath, line.number, "%s %q: want %d (baseline %s), got %d", subject, fields[i].label, want, fields[i].path, got)
		}
	}
}

func w6fAssertXpectKinds(t *testing.T, lines []docLine, report w6fXpectReport) {
	t.Helper()
	header := []string{"Kind", "Expectations", "Agree", "of which wording-only", "Disagree", "Not adjudicated", "`same-location`", "`same-line`", "`severity-differs`", "`elsewhere`", "nothing"}
	table := w6fFindXpectTable(t, lines, header)
	baseline := make(map[string]w6fXpectKind, len(report.Kinds))
	for _, kind := range report.Kinds {
		if _, exists := baseline[kind.Kind]; exists {
			docFailPathAt(t, w6fXpectDocPath, table.headerLine, "baseline kind %q appears more than once", kind.Kind)
		}
		baseline[kind.Kind] = kind
	}
	seen := make(map[string]bool, len(table.rows))
	for _, row := range table.rows {
		if docIsSeparatorRow(row.cells) {
			continue
		}
		if len(row.cells) != len(header) {
			docFailPathAt(t, w6fXpectDocPath, row.line, "per-kind table row has %d cells, want %d", len(row.cells), len(header))
		}
		kindName := docStripMarkdown(row.cells[0])
		if seen[kindName] {
			docFailPathAt(t, w6fXpectDocPath, row.line, "per-kind table row %q appears more than once", kindName)
		}
		seen[kindName] = true
		kind, ok := baseline[kindName]
		if !ok {
			docFailPathAt(t, w6fXpectDocPath, row.line, "per-kind table row %q is not in baseline kinds", kindName)
		}
		w6fAssertXpectCell(t, row, 1, kind.Rows, "Expectations", "kinds["+kindName+"].rows", false)
		w6fAssertXpectCell(t, row, 2, kind.Agree, "Agree", "kinds["+kindName+"].agree", false)
		w6fAssertXpectCell(t, row, 3, kind.WordingOnly, "of which wording-only", "kinds["+kindName+"].wordingOnly", true)
		w6fAssertXpectCell(t, row, 4, kind.Disagree, "Disagree", "kinds["+kindName+"].disagree", false)
		w6fAssertXpectCell(t, row, 5, kind.NotAdjudicated, "Not adjudicated", "kinds["+kindName+"].notAdjudicated", false)
		w6fAssertXpectCell(t, row, 6, kind.SameLocation, "same-location", "kinds["+kindName+"].sameLocation", true)
		w6fAssertXpectCell(t, row, 7, kind.SameLine, "same-line", "kinds["+kindName+"].sameLine", true)
		w6fAssertXpectCell(t, row, 8, kind.SeverityDiffers, "severity-differs", "kinds["+kindName+"].severityDiffers", true)
		w6fAssertXpectCell(t, row, 9, kind.Elsewhere, "elsewhere", "kinds["+kindName+"].elsewhereInFile", true)
		w6fAssertXpectNothing(t, row, kind, kindName)
	}
	w6fAssertXpectSet(t, table.headerLine, "per-kind table kind", w6fXpectKindNames(baseline), seen)
}

func w6fAssertXpectCell(t *testing.T, row docTableRow, index, want int, label, path string, dashAllowed bool) {
	t.Helper()
	value := strings.TrimSpace(row.cells[index])
	if value == "—" && dashAllowed {
		return
	}
	if !docIntegerPattern.MatchString(value) {
		docFailPathAt(t, w6fXpectDocPath, row.line, "per-kind table %s: expected integer, got %q", label, value)
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		docFailPathAt(t, w6fXpectDocPath, row.line, "per-kind table %s: malformed integer %q", label, value)
	}
	if got != want {
		docErrorPathAt(t, w6fXpectDocPath, row.line, "per-kind table %s: want %d (baseline %s), got %d", label, want, path, got)
	}
}

func w6fAssertXpectNothing(t *testing.T, row docTableRow, kind w6fXpectKind, name string) {
	t.Helper()
	value := strings.TrimSpace(row.cells[10])
	if value == "—" {
		return
	}
	if !docIntegerPattern.MatchString(value) {
		docFailPathAt(t, w6fXpectDocPath, row.line, "per-kind table nothing: expected integer, got %q", value)
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		docFailPathAt(t, w6fXpectDocPath, row.line, "per-kind table nothing: malformed integer %q", value)
	}
	// Tolerances classify disagreements only, so strict agreements are not silence either.
	want := kind.Rows - (kind.Agree + kind.SameLocation + kind.SameLine + kind.SeverityDiffers + kind.Elsewhere)
	if got != want {
		docErrorPathAt(t, w6fXpectDocPath, row.line, "per-kind table %s nothing: want %d (baseline kinds[%s].rows - agreements - tolerance totals), got %d", name, want, name, got)
	}
}

func w6fAssertXpectSuites(t *testing.T, lines []docLine, report w6fXpectReport) {
	t.Helper()
	header := []string{"Suite", "Files", "Expectations", "Agree", "Disagree", "Not adjudicated"}
	table := w6fFindXpectTable(t, lines, header)
	baseline := make(map[string]w6fXpectTotals, len(report.Suites))
	for _, suite := range report.Suites {
		if _, exists := baseline[suite.Name]; exists {
			docFailPathAt(t, w6fXpectDocPath, table.headerLine, "baseline suite %q appears more than once", suite.Name)
		}
		baseline[suite.Name] = suite.Totals
	}
	seen := make(map[string]bool, len(table.rows))
	for _, row := range table.rows {
		if docIsSeparatorRow(row.cells) {
			continue
		}
		if len(row.cells) != len(header) {
			docFailPathAt(t, w6fXpectDocPath, row.line, "per-suite table row has %d cells, want %d", len(row.cells), len(header))
		}
		name := docStripMarkdown(row.cells[0])
		if seen[name] {
			docFailPathAt(t, w6fXpectDocPath, row.line, "per-suite table row %q appears more than once", name)
		}
		seen[name] = true
		totals, ok := baseline[name]
		if !ok {
			docFailPathAt(t, w6fXpectDocPath, row.line, "per-suite table row %q is not in baseline suites", name)
		}
		w6fAssertSuiteCell(t, row, 1, totals.Files, "Files", "suites["+name+"].totals.files")
		w6fAssertSuiteCell(t, row, 2, totals.Rows, "Expectations", "suites["+name+"].totals.rows")
		w6fAssertSuiteCell(t, row, 3, totals.Agree, "Agree", "suites["+name+"].totals.agree")
		w6fAssertSuiteCell(t, row, 4, totals.Disagree, "Disagree", "suites["+name+"].totals.disagree")
		w6fAssertSuiteCell(t, row, 5, totals.NotAdjudicated, "Not adjudicated", "suites["+name+"].totals.notAdjudicated")
	}
	w6fAssertXpectSet(t, table.headerLine, "per-suite table suite", w6fXpectSuiteNames(baseline), seen)
}

func w6fAssertSuiteCell(t *testing.T, row docTableRow, index, want int, label, path string) {
	t.Helper()
	value := strings.TrimSpace(row.cells[index])
	if !docIntegerPattern.MatchString(value) {
		docFailPathAt(t, w6fXpectDocPath, row.line, "per-suite table %s: expected integer, got %q", label, value)
	}
	got, err := strconv.Atoi(value)
	if err != nil {
		docFailPathAt(t, w6fXpectDocPath, row.line, "per-suite table %s: malformed integer %q", label, value)
	}
	if got != want {
		docErrorPathAt(t, w6fXpectDocPath, row.line, "per-suite table %s: want %d (baseline %s), got %d", label, want, path, got)
	}
}

func w6fXpectKindNames(baseline map[string]w6fXpectKind) map[string]bool {
	names := make(map[string]bool, len(baseline))
	for name := range baseline {
		names[name] = true
	}
	return names
}

func w6fXpectSuiteNames(baseline map[string]w6fXpectTotals) map[string]bool {
	names := make(map[string]bool, len(baseline))
	for name := range baseline {
		names[name] = true
	}
	return names
}

func w6fAssertXpectSet(t *testing.T, line int, subject string, baseline map[string]bool, seen map[string]bool) {
	t.Helper()
	wants := make([]string, 0, len(baseline))
	for name := range baseline {
		wants = append(wants, name)
	}
	gots := make([]string, 0, len(seen))
	for name := range seen {
		gots = append(gots, name)
	}
	sort.Strings(wants)
	sort.Strings(gots)
	if strings.Join(wants, "\x00") != strings.Join(gots, "\x00") {
		docErrorPathAt(t, w6fXpectDocPath, line, "%s set: want %q (baseline), got %q", subject, wants, gots)
	}
}

func w6fFindXpectTable(t *testing.T, lines []docLine, wantHeader []string) docTable {
	t.Helper()
	for i, line := range lines {
		if !docIsTableLine(line.text) || !w6fEqualStrings(docSplitTableCells(line.text), wantHeader) {
			continue
		}
		rows := make([]docTableRow, 0)
		for j := i + 1; j < len(lines) && docIsTableLine(lines[j].text); j++ {
			rows = append(rows, docTableRow{line: lines[j].number, cells: docSplitTableCells(lines[j].text)})
		}
		return docTable{header: wantHeader, headerLine: line.number, rows: rows}
	}
	docFailPathAt(t, w6fXpectDocPath, 1, "missing table with header %q", wantHeader)
	return docTable{}
}

func w6fEqualStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func w6fAssertXpectCensus(t *testing.T, lines []docLine, report w6fXpectReport) {
	t.Helper()
	kerml, sysml := w6fXpectSuiteTotals(t, report)
	w6fAssertXpectFileCensus(t, lines, kerml, sysml, report.Totals.Files)
	w6fAssertXpectRowCensus(t, lines, report.Totals.Assertions, report.Totals.Rows)
}

func w6fAssertXpectFileCensus(t *testing.T, lines []docLine, kerml, sysml w6fXpectTotals, total int) {
	t.Helper()
	for _, line := range lines {
		if match := w6fXpectCensusFiles.FindStringSubmatch(line.text); match != nil {
			w6fAssertXpectNumber(t, line, match[1], kerml.Files, "census KerML files", "suites[kerml].totals.files")
			w6fAssertXpectNumber(t, line, match[2], sysml.Files, "census SysML files", "suites[sysml].totals.files")
			w6fAssertXpectNumber(t, line, match[3], total, "census total files", "totals.files")
			if kerml.Files+sysml.Files != total {
				docErrorPathAt(t, w6fXpectDocPath, line.number, "census total files: want addends to sum to %d (baseline totals.files), got %d", total, kerml.Files+sysml.Files)
			}
			return
		}
	}
	docFailPathAt(t, w6fXpectDocPath, 1, "missing census files claim")
}

func w6fAssertXpectRowCensus(t *testing.T, lines []docLine, assertions, rows int) {
	t.Helper()
	for _, line := range lines {
		if match := w6fXpectCensusRows.FindStringSubmatch(line.text); match != nil {
			w6fAssertXpectNumber(t, line, match[1], assertions, "census assertions", "totals.assertions")
			w6fAssertXpectNumber(t, line, match[2], rows, "census expectations", "totals.rows")
			return
		}
	}
	docFailPathAt(t, w6fXpectDocPath, 1, "missing census assertions claim")
}

func w6fXpectSuiteTotals(t *testing.T, report w6fXpectReport) (w6fXpectTotals, w6fXpectTotals) {
	t.Helper()
	var kerml, sysml w6fXpectTotals
	var foundKerml, foundSysml bool
	for _, suite := range report.Suites {
		switch suite.Name {
		case "kerml":
			kerml, foundKerml = suite.Totals, true
		case "sysml":
			sysml, foundSysml = suite.Totals, true
		}
	}
	if !foundKerml {
		docFailPathAt(t, w6fXpectBaselinePath, 1, "baseline is missing suite %q", "kerml")
	}
	if !foundSysml {
		docFailPathAt(t, w6fXpectBaselinePath, 1, "baseline is missing suite %q", "sysml")
	}
	return kerml, sysml
}

func w6fAssertXpectNumber(t *testing.T, line docLine, text string, want int, subject, path string) {
	t.Helper()
	got, err := strconv.Atoi(text)
	if err != nil {
		docFailPathAt(t, w6fXpectDocPath, line.number, "%s: malformed number %q", subject, text)
	}
	if got != want {
		docErrorPathAt(t, w6fXpectDocPath, line.number, "%s: want %d (baseline %s), got %d", subject, want, path, got)
	}
}
