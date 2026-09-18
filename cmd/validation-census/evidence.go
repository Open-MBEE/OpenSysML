package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// rejectionBaselinePath is the pilot-rejection record whose per-case buckets
// say which of the two tools rejects each corpus case.
const rejectionBaselinePath = "docs/project/pilot-rejection-baseline.json"

// Buckets as tools/referee/reject records them; only these three make a case
// evidence of a rejection.
const (
	bucketBothReject = "both-reject"
	bucketPilotOnly  = "pilot-only-rejects"
	bucketOursOnly   = "ours-only-rejects"
)

func rejectionBucket(bucket string) bool {
	return bucket == bucketBothReject || bucket == bucketPilotOnly || bucket == bucketOursOnly
}

// corpusCase is one rejection-corpus model: the specification constraint its
// header cites, the pilot constraint it attributes the rejection to, if any,
// and the bucket the referee recorded.
type corpusCase struct {
	Path   string
	Spec   string
	Pilot  string
	Bucket string
}

// pilotAttribution is the `pilot <constraint>` token a semantic case's header
// carries, naming the constraint the pinned pilot rejects the model under;
// specCitation is the constraint the citation before it names, when it does.
var (
	pilotAttribution = regexp.MustCompile(`\bpilot ((?:in)?validate[A-Za-z]+_?)\b`)
	specCitation     = regexp.MustCompile(`\b((?:in)?validate[A-Za-z]+_?); pilot\b`)
)

// loadCorpus reads every corpus case's header line and its recorded bucket.
func loadCorpus(root string) (map[string]corpusCase, error) {
	var record struct {
		Cases []struct {
			Path   string `json:"path"`
			Bucket string `json:"bucket"`
		} `json:"cases"`
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rejectionBaselinePath))) // #nosec G304 -- fixed repository path
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(content, &record); err != nil {
		return nil, fmt.Errorf("%s: %w", rejectionBaselinePath, err)
	}
	buckets := make(map[string]string, len(record.Cases))
	for _, c := range record.Cases {
		buckets[c.Path] = c.Bucket
	}
	dir := filepath.Join(root, filepath.FromSlash(negativeCorpusDir))
	cases := make(map[string]corpusCase)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || (filepath.Ext(path) != ".sysml" && filepath.Ext(path) != ".kerml") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		header, err := firstLine(path)
		if err != nil {
			return err
		}
		c := corpusCase{Path: filepath.ToSlash(rel), Bucket: buckets[filepath.ToSlash(rel)]}
		if m := pilotAttribution.FindStringSubmatch(header); m != nil {
			c.Pilot = m[1]
		}
		if m := specCitation.FindStringSubmatch(header); m != nil {
			c.Spec = m[1]
		}
		cases[c.Path] = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cases, nil
}

func firstLine(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- repository testdata
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	return "", scanner.Err()
}

// constraintNamed resolves a header token to the baseline constraint it names,
// by normalized or raw compiled name.
func constraintNamed(base *Baseline, token string) (Constraint, bool) {
	for _, c := range base.Constraints {
		if c.Name == token || (c.Raw != "" && c.Raw == token) {
			return c, true
		}
	}
	return Constraint{}, false
}

// attribution is the constraint a case's header attributes it to: the pilot
// token when present, else the specification citation; empty when neither.
func (c corpusCase) attribution() string {
	if c.Pilot != "" {
		return c.Pilot
	}
	return c.Spec
}

// attributedTo reports whether a case's header attributes it to the named
// constraint: either the pilot token or the specification citation resolves to it.
func attributedTo(base *Baseline, c corpusCase, name string) bool {
	for _, token := range []string{c.Pilot, c.Spec} {
		if token == "" {
			continue
		}
		if token == name {
			return true
		}
		if constraint, ok := constraintNamed(base, token); ok && constraint.Name == name {
			return true
		}
	}
	return false
}

// checkCaseEvidence verifies one listed negative case against the corpus: its
// header names a constraint, by pilot token or specification citation, that
// resolves to the row's constraint, and the bucket the referee recorded agrees
// with the row's status — a faithful row's case is one we reject, a
// not-implemented row's case is one only the pilot rejects, an approximate row
// may list either (a pilot-only case is the gap it records), and an unknown
// row has no case.
func checkCaseEvidence(base *Baseline, r row, name, status string, c corpusCase) []string {
	var problems []string
	switch attributed := c.attribution(); {
	case attributed == "":
		problems = append(problems, fmt.Sprintf("line %d: %s negative case %s names no constraint in its header (`<clause> validate…; pilot …` or `pilot validate…`)", r.Line, name, c.Path))
	case !attributedTo(base, c, name):
		problems = append(problems, fmt.Sprintf("line %d: %s negative case %s is attributed to %s, not to this constraint", r.Line, name, c.Path, attributed))
	}
	switch {
	case c.Bucket == "":
		problems = append(problems, fmt.Sprintf("line %d: %s negative case %s is not recorded in %s", r.Line, name, c.Path, rejectionBaselinePath))
	case !rejectionBucket(c.Bucket):
		problems = append(problems, fmt.Sprintf("line %d: %s negative case %s is recorded as %s in %s, which is not a rejection", r.Line, name, c.Path, c.Bucket, rejectionBaselinePath))
	case status == StatusFaithful && c.Bucket != bucketBothReject && c.Bucket != bucketOursOnly:
		problems = append(problems, fmt.Sprintf("line %d: %s is recorded %s but %s records its negative case %s as %s", r.Line, name, status, rejectionBaselinePath, c.Path, c.Bucket))
	case status == StatusNotImplemented && c.Bucket != bucketPilotOnly:
		problems = append(problems, fmt.Sprintf("line %d: %s is recorded %s but %s records its negative case %s as %s", r.Line, name, status, rejectionBaselinePath, c.Path, c.Bucket))
	case status == StatusUnknown:
		problems = append(problems, fmt.Sprintf("line %d: %s is recorded %s but lists negative case %s", r.Line, name, status, c.Path))
	}
	return problems
}

// checkAttributions verifies that every corpus case attributing its rejection
// to a pilot constraint names one the baseline lists, and that the constraint's
// row lists the case.
func checkAttributions(base *Baseline, cases map[string]corpusCase, listed map[string]map[string]bool) []string {
	var problems []string
	for _, c := range cases {
		if c.Pilot == "" {
			continue
		}
		constraint, ok := constraintNamed(base, c.Pilot)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s/%s attributes its rejection to %s, which %s does not list", negativeCorpusDir, c.Path, c.Pilot, baselinePath))
			continue
		}
		if !listed[constraint.Name][c.Path] {
			problems = append(problems, fmt.Sprintf("%s/%s attributes its rejection to %s, whose row does not list it as a negative case", negativeCorpusDir, c.Path, constraint.Name))
		}
	}
	sort.Strings(problems)
	return problems
}

// implementationRef is a `<file>.go:<func>` or `<file>.go:<Type>.<method>` citation;
// group 3 captures any continuation past the symbol other than the punctuation
// that ends a cell, list item or sentence, which is malformed.
var implementationRef = regexp.MustCompile(`\b([\w/.-]+\.go):([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)((?:\.\w+)+|[^\s,;.)\]` + "`" + `]+)?`)

// implementationRoot is the directory every cited Go file must be given relative to.
const implementationRoot = "internal/"

// declarations caches the function and method names each cited Go file declares.
type declarations struct {
	root  string
	files map[string]map[string]bool
}

func newDeclarations(root string) *declarations {
	return &declarations{root: root, files: make(map[string]map[string]bool)}
}

// declared reports whether file declares the named function or Type.method;
// a bare name matches a function or a method with that name.
func (d *declarations) declared(file, name string) (bool, error) {
	names, ok := d.files[file]
	if !ok {
		path := filepath.Join(d.root, filepath.FromSlash(file))
		if _, err := os.Stat(path); err != nil {
			return false, fmt.Errorf("%s does not exist", file)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return false, err
		}
		names = make(map[string]bool)
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			names[fn.Name.Name] = true
			if recv := receiverType(fn); recv != "" {
				names[recv+"."+fn.Name.Name] = true
			}
		}
		d.files[file] = names
	}
	return names[name], nil
}

func receiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.IndexExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexListExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// checkImplementation verifies that an implemented row cites at least one Go
// location and that every cited location exists and declares the function or
// method it names.
func checkImplementation(decls *declarations, r row, name, status string) []string {
	var problems []string
	refs := implementationRef.FindAllStringSubmatch(r.Cells[3], -1)
	if implemented(status) && len(refs) == 0 && r.Cells[3] != "" && r.Cells[3] != "—" {
		problems = append(problems, fmt.Sprintf("line %d: %s implementation %q cites no internal/<file>.go:<function> location", r.Line, name, r.Cells[3]))
	}
	for _, m := range refs {
		file, symbol := m[1], m[2]
		if !strings.HasPrefix(file, implementationRoot) {
			problems = append(problems, fmt.Sprintf("line %d: %s implementation %s:%s is not a repository-relative %s<file>.go location", r.Line, name, file, symbol, implementationRoot))
			continue
		}
		if m[3] != "" {
			problems = append(problems, fmt.Sprintf("line %d: %s implementation %s:%s%s is not a <function> or <Type>.<method> location", r.Line, name, file, symbol, m[3]))
			continue
		}
		found, err := decls.declared(file, symbol)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("line %d: %s implementation: %v", r.Line, name, err))
		case !found:
			problems = append(problems, fmt.Sprintf("line %d: %s implementation %s declares no %s", r.Line, name, file, symbol))
		}
	}
	return problems
}

// negativeCases returns the corpus paths a row's negative-case cell lists.
func negativeCases(cell string) []string {
	if cell == "none" {
		return nil
	}
	var paths []string
	for _, ref := range strings.Split(cell, ",") {
		if path, ok := backticked(strings.TrimSpace(ref)); ok {
			paths = append(paths, path)
		}
	}
	return paths
}
