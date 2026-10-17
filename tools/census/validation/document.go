package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/tools/census/doccounts"
	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
)

// The provenance and summary lines of the census document are derived from the
// manifest: this program writes them and -check refuses a hand-edited value.
const summaryMarker = "**Census:**"

// derivedLine is one document line whose capture groups the manifest dictates.
type derivedLine struct {
	marker  string
	pattern *regexp.Regexp
	values  func(*manifest.Manifest) []string
}

var derivedLines = []derivedLine{
	{
		marker:  "**Pilot:**",
		pattern: regexp.MustCompile("^\\*\\*Pilot:\\*\\* .* release `([^`]*)`, commit `([^`]*)`, artifact `jupyter-sysml-kernel ([^`]*)` — .*$"),
		values:  func(m *manifest.Manifest) []string { return []string{m.PilotTag, m.PilotCommit, m.PilotArtifact} },
	},
	{
		marker:  "**Jar:**",
		pattern: regexp.MustCompile("^\\*\\*Jar:\\*\\* `([^`]*)` \\(`([^`]*)`\\), .*$"),
		values:  func(m *manifest.Manifest) []string { return []string{m.Jar.Name, m.Jar.Digest} },
	},
	{
		marker:  summaryMarker,
		pattern: regexp.MustCompile(`^\*\*Census:\*\* .*$`),
		values:  func(m *manifest.Manifest) []string { return []string{m.Summary()} },
	},
}

// tableBlock is the generated block the census table is rendered into; doc-counts
// renders it from the manifest and this program verifies and rewrites it too.
var tableBlock = doccounts.Block{Path: censusDocPath, Name: "validation-census-table"}

// rewriteDerivedLines restates the provenance and summary lines and the census
// table from the manifest and leaves every other byte of the document alone.
func rewriteDerivedLines(content string, m *manifest.Manifest) (string, error) {
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
		values := d.values(m)
		rewritten := lines[index]
		if len(match) == 2 {
			rewritten = values[0]
		}
		for i := len(values) - 1; i >= 0 && len(match) > 2; i-- {
			rewritten = rewritten[:match[2+i*2]] + values[i] + rewritten[match[3+i*2]:]
		}
		lines[index] = rewritten
	}
	return doccounts.ReplaceBlock(strings.Join(lines, "\n"), tableBlock, m.RenderTable())
}

// negativeCorpusDir holds the rejection corpus a row's negative case must be in.
const negativeCorpusDir = "tools/referee/reject/testdata/negative"

// checkDocument verifies the census document against the manifest, and the
// manifest's rows against the tree: the derived lines and the table are
// current, every implemented row cites a declared implementation, every listed
// negative case exists and is attributed to its row, and every corpus case
// attributed to a constraint is listed on its row.
func checkDocument(root, content string, m *manifest.Manifest) error {
	rewritten, err := rewriteDerivedLines(content, m)
	if err != nil {
		return err
	}
	if rewritten != content {
		return fmt.Errorf("%s: a derived line or the census table is stale; run `go run -C tools ./cmd/validation-census`", censusDocPath)
	}
	cases, err := loadCorpus(root)
	if err != nil {
		return err
	}
	decls := newDeclarations(root)
	listed := make(map[string]map[string]bool, len(m.Constraints))
	var problems []string
	for _, c := range m.Constraints {
		if manifest.Implemented(c.Status) && (c.Implementation == "" || c.Implementation == "—") {
			problems = append(problems, fmt.Sprintf("%s is recorded %s but names no implementation", c.Name, c.Status))
		}
		problems = append(problems, checkImplementation(decls, c)...)
		problems = append(problems, checkNegativeCase(root, c)...)
		listed[c.Name] = make(map[string]bool)
		for _, path := range negativeCases(c.NegativeCase) {
			listed[c.Name][path] = true
			if cc, ok := cases[path]; ok {
				problems = append(problems, checkCaseEvidence(m, c, cc)...)
			}
		}
	}
	problems = append(problems, checkAttributions(m, cases, listed)...)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s does not match the tree:\n  %s", manifest.Path, strings.Join(problems, "\n  "))
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
func checkNegativeCase(root string, c manifest.Constraint) []string {
	cell := c.NegativeCase
	if cell == "none" {
		return nil
	}
	var problems []string
	for _, ref := range strings.Split(cell, ",") {
		ref = strings.TrimSpace(ref)
		path, ok := backticked(ref)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s negative case %q is neither `none` nor a backticked corpus path", c.Name, ref))
			continue
		}
		if ext := filepath.Ext(path); ext != ".sysml" && ext != ".kerml" {
			problems = append(problems, fmt.Sprintf("%s negative case %s is not a .sysml or .kerml file", c.Name, path))
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(negativeCorpusDir), filepath.FromSlash(path)))
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("%s negative case %s/%s does not exist", c.Name, negativeCorpusDir, path))
		case !info.Mode().IsRegular():
			problems = append(problems, fmt.Sprintf("%s negative case %s/%s is not a file", c.Name, negativeCorpusDir, path))
		}
	}
	return problems
}

// probesDir holds the minimal violating models that back every implemented row.
const probesDir = "tools/census/validation/testdata/probes"

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
	probeConstraintLine = regexp.MustCompile(`^// Census: ((?:validate|check|derive)[A-Za-z]+)$`)
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

// checkProbes verifies that every implemented validate* constraint has a probe
// (one per notation when both validators declare it) and a recorded diagnostic
// code, and that no probe claims a diagnostic for a constraint the manifest
// records as unreported. An implemented check* or derive* row establishes an
// implied relationship or a derived value rather than reporting a diagnostic,
// so its evidence is the implementation it cites; a probe backs it only when a
// pass does report. The codes and passes themselves are held to the probes by
// the package's tests.
func checkProbes(root string, m *manifest.Manifest) error {
	probes, err := loadProbes(root)
	if err != nil {
		return err
	}
	backed := make(map[string]map[string]bool)
	var problems []string
	for _, p := range probes {
		c, ok := m.Constraint(p.Constraint)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s names %s, which %s does not list", p.Path, p.Constraint, manifest.Path))
			continue
		}
		if !manifest.Implemented(c.Status) {
			problems = append(problems, fmt.Sprintf("%s expects a %s for %s, which the manifest records as %s", p.Path, p.Severity, p.Constraint, c.Status))
			continue
		}
		if backed[p.Constraint] == nil {
			backed[p.Constraint] = make(map[string]bool)
		}
		backed[p.Constraint][p.Language] = true
		backed[p.Constraint][""] = true
	}
	for _, c := range m.Constraints {
		if !manifest.Implemented(c.Status) {
			if len(c.Codes) != 0 || len(c.Passes) != 0 {
				problems = append(problems, fmt.Sprintf("%s is recorded %s but records diagnostic codes %q from passes %q", c.Name, c.Status, c.Codes, c.Passes))
			}
			continue
		}
		if c.Kind != "validate" {
			continue
		}
		if len(c.Codes) == 0 || len(c.Passes) == 0 {
			problems = append(problems, fmt.Sprintf("%s is recorded %s but records no diagnostic code or pass", c.Name, c.Status))
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
