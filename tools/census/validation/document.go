package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The provenance and summary lines of the census document are derived from the
// baseline: this program writes them and -check refuses a hand-edited value.
const summaryMarker = "**Census:**"

// derivedLine is one document line whose capture groups the baseline dictates.
type derivedLine struct {
	marker  string
	pattern *regexp.Regexp
	values  func(*Baseline) []string
}

var derivedLines = []derivedLine{
	{
		marker:  "**Pilot:**",
		pattern: regexp.MustCompile("^\\*\\*Pilot:\\*\\* .* release `([^`]*)`, commit `([^`]*)`, artifact `jupyter-sysml-kernel ([^`]*)` — .*$"),
		values:  func(b *Baseline) []string { return []string{b.PilotTag, b.PilotCommit, b.PilotArtifact} },
	},
	{
		marker:  "**Jar:**",
		pattern: regexp.MustCompile("^\\*\\*Jar:\\*\\* `([^`]*)` \\(`([^`]*)`\\), .*$"),
		values:  func(b *Baseline) []string { return []string{b.Jar.Name, b.Jar.Digest} },
	},
	{
		marker:  summaryMarker,
		pattern: regexp.MustCompile(`^\*\*Census:\*\* (\d+) of (\d+) named constraints are reported by OpenSysML — (\d+) ✅ faithful and (\d+) ⚠️ approximate; (\d+) ❌ not implemented, (\d+) ⛔ deliberate, (\d+) 🚧 known failure, (\d+) ❔ unknown\.$`),
		values:  func(b *Baseline) []string { return summaryValues(b.counts()) },
	},
}

func summaryValues(c counts) []string {
	return []string{
		strconv.Itoa(c.Implemented()), strconv.Itoa(c.Total),
		strconv.Itoa(c.ByState[StatusFaithful]), strconv.Itoa(c.ByState[StatusApproximate]),
		strconv.Itoa(c.ByState[StatusNotImplemented]), strconv.Itoa(c.ByState[StatusDeliberate]),
		strconv.Itoa(c.ByState[StatusKnownFailure]), strconv.Itoa(c.ByState[StatusUnknown]),
	}
}

// rewriteDerivedLines restates the provenance and summary lines from the
// baseline and leaves every other byte of the document alone.
func rewriteDerivedLines(content string, base *Baseline) (string, error) {
	lines := strings.Split(content, "\n")
	for _, d := range derivedLines {
		index := -1
		for i, line := range lines {
			if strings.HasPrefix(line, d.marker) {
				if index >= 0 {
					return "", fmt.Errorf("%s: two lines carry %s", censusDocPath, d.marker)
				}
				index = i
			}
		}
		if index < 0 {
			return "", fmt.Errorf("%s: no line carries %s", censusDocPath, d.marker)
		}
		match := d.pattern.FindStringSubmatchIndex(lines[index])
		if match == nil {
			return "", fmt.Errorf("%s:%d: the %s line does not match the derived-line pattern", censusDocPath, index+1, d.marker)
		}
		values := d.values(base)
		rewritten := lines[index]
		for i := len(values) - 1; i >= 0; i-- {
			rewritten = rewritten[:match[2+i*2]] + values[i] + rewritten[match[3+i*2]:]
		}
		lines[index] = rewritten
	}
	return strings.Join(lines, "\n"), nil
}

// tableColumns is the census table's header, which fixes the cell order the
// checks below read.
var tableColumns = []string{"Constraint", "Language", "Checks", "Implementation", "Our message", "Negative case", "Status"}

var separatorPattern = regexp.MustCompile(`^\|(?: *:?-+:? *\|)+$`)

// row is one parsed table row.
type row struct {
	Line  int
	Cells []string
}

// parseTable returns the rows of the census table, located by its header.
func parseTable(content string) ([]row, error) {
	lines := strings.Split(content, "\n")
	header := "| " + strings.Join(tableColumns, " | ") + " |"
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			if start >= 0 {
				return nil, fmt.Errorf("%s: two census tables", censusDocPath)
			}
			start = i
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("%s: no table headed %q", censusDocPath, header)
	}
	if start+1 >= len(lines) || !separatorPattern.MatchString(strings.TrimSpace(lines[start+1])) {
		return nil, fmt.Errorf("%s:%d: the census table header has no separator row", censusDocPath, start+2)
	}
	var rows []row
	for i := start + 2; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := splitCells(line)
		if len(cells) != len(tableColumns) {
			return nil, fmt.Errorf("%s:%d: row has %d cells, the table %d columns", censusDocPath, i+1, len(cells), len(tableColumns))
		}
		rows = append(rows, row{Line: i + 1, Cells: cells})
	}
	return rows, nil
}

// splitCells splits a table row on its unescaped pipes.
func splitCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	var cells []string
	var cell strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cell.WriteRune('\\')
			cell.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(cell.String()))
	return cells
}

var languageBySource = map[string]string{"kerml": "KerML", "sysml": "SysML", "both": "both"}

// negativeCorpusDir holds the rejection corpus a row's negative case must be in.
const negativeCorpusDir = "tools/referee/reject/testdata/negative"

// checkDocument verifies the census document against the baseline and the
// rejection corpus: current summary figures, one row per baseline constraint
// and no other, each row's language and status as recorded, a location on
// every implemented row that resolves to a declared function, and negative
// cases that exist, name the row's constraint, and whose recorded bucket agrees
// with the row's status — with every case the corpus attributes to a pilot
// constraint listed on that constraint's row.
func checkDocument(root, content string, base *Baseline) error {
	rewritten, err := rewriteDerivedLines(content, base)
	if err != nil {
		return err
	}
	if rewritten != content {
		return fmt.Errorf("%s: a derived line is stale; run `go run ./cmd/validation-census`", censusDocPath)
	}
	rows, err := parseTable(content)
	if err != nil {
		return err
	}
	cases, err := loadCorpus(root)
	if err != nil {
		return err
	}
	decls := newDeclarations(root)
	recorded := make(map[string]Constraint, len(base.Constraints))
	for _, c := range base.Constraints {
		recorded[c.Name] = c
	}
	seen := make(map[string]int, len(rows))
	listed := make(map[string]map[string]bool, len(rows))
	var problems []string
	for _, r := range rows {
		name, ok := backticked(r.Cells[0])
		if !ok {
			problems = append(problems, fmt.Sprintf("line %d: the constraint cell %q is not a backticked name", r.Line, r.Cells[0]))
			continue
		}
		if line, dup := seen[name]; dup {
			problems = append(problems, fmt.Sprintf("line %d: %s already has a row at line %d", r.Line, name, line))
			continue
		}
		seen[name] = r.Line
		c, ok := recorded[name]
		if !ok {
			problems = append(problems, fmt.Sprintf("line %d: %s is in the table but not in %s", r.Line, name, baselinePath))
			continue
		}
		if want := languageBySource[c.Source]; r.Cells[1] != want {
			problems = append(problems, fmt.Sprintf("line %d: %s has language %q, the baseline source %s says %q", r.Line, name, r.Cells[1], c.Source, want))
		}
		marker, _ := markerFor(c.Status)
		if r.Cells[6] != marker {
			problems = append(problems, fmt.Sprintf("line %d: %s has status %q, the baseline records %s (%q)", r.Line, name, r.Cells[6], c.Status, marker))
		}
		if implemented(c.Status) && (r.Cells[3] == "" || r.Cells[3] == "—") {
			problems = append(problems, fmt.Sprintf("line %d: %s is recorded %s but names no implementation", r.Line, name, c.Status))
		}
		problems = append(problems, checkImplementation(decls, r, name, c.Status)...)
		problems = append(problems, checkNegativeCase(root, r, name)...)
		listed[name] = make(map[string]bool)
		for _, path := range negativeCases(r.Cells[5]) {
			listed[name][path] = true
			if cc, ok := cases[path]; ok {
				problems = append(problems, checkCaseEvidence(base, r, name, c.Status, cc)...)
			}
		}
	}
	for _, c := range base.Constraints {
		if _, ok := seen[c.Name]; !ok {
			problems = append(problems, fmt.Sprintf("%s is in %s but has no row", c.Name, baselinePath))
		}
	}
	problems = append(problems, checkAttributions(base, cases, listed)...)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s does not match %s:\n  %s", censusDocPath, baselinePath, strings.Join(problems, "\n  "))
}

// backticked returns the text of a cell that is exactly one backticked code span.
func backticked(cell string) (string, bool) {
	text := strings.TrimSuffix(strings.TrimPrefix(cell, "`"), "`")
	if text == "" || strings.ContainsRune(text, '`') || cell != "`"+text+"`" {
		return "", false
	}
	return text, true
}

// checkNegativeCase verifies that a row's negative-case cell is `none` or names
// model files of the rejection corpus that exist.
func checkNegativeCase(root string, r row, name string) []string {
	cell := r.Cells[5]
	if cell == "none" {
		return nil
	}
	var problems []string
	for _, ref := range strings.Split(cell, ",") {
		ref = strings.TrimSpace(ref)
		path, ok := backticked(ref)
		if !ok {
			problems = append(problems, fmt.Sprintf("line %d: %s negative case %q is neither `none` nor a backticked corpus path", r.Line, name, ref))
			continue
		}
		if ext := filepath.Ext(path); ext != ".sysml" && ext != ".kerml" {
			problems = append(problems, fmt.Sprintf("line %d: %s negative case %s is not a .sysml or .kerml file", r.Line, name, path))
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(negativeCorpusDir), filepath.FromSlash(path)))
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("line %d: %s negative case %s/%s does not exist", r.Line, name, negativeCorpusDir, path))
		case !info.Mode().IsRegular():
			problems = append(problems, fmt.Sprintf("line %d: %s negative case %s/%s is not a file", r.Line, name, negativeCorpusDir, path))
		}
	}
	return problems
}

// probesDir holds the minimal violating models that back every implemented row.
const probesDir = "cmd/validation-census/testdata/probes"

// probe is one violating model and the diagnostic it expects from us.
type probe struct {
	Path       string
	Constraint string
	// Language is the model's notation, kerml or sysml, from its extension.
	Language string
	Severity string
	// Message is a fragment the diagnostic's message must contain.
	Message string
}

var (
	probeConstraintLine = regexp.MustCompile(`^// Census: (validate[A-Za-z]+)$`)
	probeExpectLine     = regexp.MustCompile(`^// Expect: (error|warning): (.+)$`)
)

// loadProbes reads every probe's two-line header.
func loadProbes(root string) ([]probe, error) {
	dir := filepath.Join(root, filepath.FromSlash(probesDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var probes []probe
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".sysml" && ext != ".kerml" {
			return nil, fmt.Errorf("%s/%s: probes are .sysml or .kerml files", probesDir, entry.Name())
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- repository testdata
		if err != nil {
			return nil, err
		}
		lines := strings.SplitN(string(content), "\n", 3)
		if len(lines) < 3 {
			return nil, fmt.Errorf("%s/%s: a probe opens with `// Census: <constraint>` and `// Expect: ...` lines", probesDir, entry.Name())
		}
		name := probeConstraintLine.FindStringSubmatch(lines[0])
		expect := probeExpectLine.FindStringSubmatch(lines[1])
		if name == nil || expect == nil {
			return nil, fmt.Errorf("%s/%s: a probe opens with `// Census: <constraint>` and `// Expect: <error|warning>: <message>` lines", probesDir, entry.Name())
		}
		stem, _, _ := strings.Cut(strings.TrimSuffix(entry.Name(), ext), ".")
		if stem != name[1] {
			return nil, fmt.Errorf("%s/%s: the file is named for %s but declares %s", probesDir, entry.Name(), stem, name[1])
		}
		probes = append(probes, probe{Path: filepath.Join(probesDir, entry.Name()), Constraint: name[1], Language: ext[1:], Severity: expect[1], Message: expect[2]})
	}
	return probes, nil
}

// probeLanguages lists the notations a constraint's probes must cover; a
// constraint both validators declare has a mapping per notation.
func probeLanguages(source string) []string {
	if source == "both" {
		return []string{"kerml", "sysml"}
	}
	return []string{""}
}

// checkProbes verifies that every implemented constraint has a probe (one per
// notation when both validators declare it) and that no probe claims a
// diagnostic for a constraint the baseline records as unreported.
func checkProbes(root string, base *Baseline) error {
	probes, err := loadProbes(root)
	if err != nil {
		return err
	}
	recorded := make(map[string]Constraint, len(base.Constraints))
	for _, c := range base.Constraints {
		recorded[c.Name] = c
	}
	backed := make(map[string]map[string]bool)
	var problems []string
	for _, p := range probes {
		c, ok := recorded[p.Constraint]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s names %s, which %s does not list", p.Path, p.Constraint, baselinePath))
			continue
		}
		if !implemented(c.Status) {
			problems = append(problems, fmt.Sprintf("%s expects a %s for %s, which the baseline records as %s", p.Path, p.Severity, p.Constraint, c.Status))
			continue
		}
		if backed[p.Constraint] == nil {
			backed[p.Constraint] = make(map[string]bool)
		}
		backed[p.Constraint][p.Language] = true
		backed[p.Constraint][""] = true
	}
	for _, c := range base.Constraints {
		if !implemented(c.Status) {
			continue
		}
		for _, lang := range probeLanguages(c.Source) {
			if backed[c.Name][lang] {
				continue
			}
			if lang == "" {
				problems = append(problems, fmt.Sprintf("%s is recorded %s but no probe under %s expects its diagnostic", c.Name, c.Status, probesDir))
			} else {
				problems = append(problems, fmt.Sprintf("%s is recorded %s in both notations but no .%s probe under %s expects its diagnostic", c.Name, c.Status, lang, probesDir))
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("the probes do not back the census:\n  %s", strings.Join(problems, "\n  "))
}
