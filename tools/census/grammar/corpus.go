package grammar

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// corpusRoot is one directory of model files searched for evidence.
type corpusRoot struct {
	Name string
	Dir  string
	// Skip lists sub-paths of Dir (slash-separated, relative to Dir) that belong
	// to another root.
	Skip []string
}

// evidenceRoots are the inputs our test suite already parses. Roots are searched
// in this order, so the citation a production gets is stable.
var evidenceRoots = []corpusRoot{
	{Name: "stdlib", Dir: "internal/core/libs/stdlib"},
	{Name: "training", Dir: "examples/sysml-v2-training"},
	{Name: "pilot-sysml-examples", Dir: "examples/pilot-corpora/sysml-examples"},
	{Name: "pilot-sysml-validation", Dir: "examples/pilot-corpora/sysml-validation"},
	{Name: "pilot-kerml-examples", Dir: "examples/pilot-corpora/kerml-examples"},
	{Name: "testdata", Dir: "testdata"},
	{Name: "parser-fixtures", Dir: "tests/parser/testdata/parse"},
	{Name: "examples", Dir: "examples", Skip: []string{"sysml-v2-training", "pilot-corpora"}},
}

// RootStat records what a root contributed, so a report says which corpora were
// present when it was produced.
type RootStat struct {
	Name  string `json:"name"`
	Dir   string `json:"dir"`
	Files int    `json:"files"`
	Lines int    `json:"lines"`
}

// Citation is where a literal was first seen in the corpora.
type Citation struct {
	Literal string `json:"literal"`
	Root    string `json:"root"`
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// fileIndex is one corpus file's literals with the line each was first seen on.
type fileIndex struct {
	root  string
	path  string
	lines map[string]int
	mask  mask
}

// has reports whether the file contains the literal.
func (f fileIndex) has(literal string) bool {
	_, ok := f.lines[literal]
	return ok
}

// literalIndex answers, for a literal, which corpus files contain it and where
// it was first seen. Evidence is decided per file rather than corpus-wide,
// because literals from two unrelated files can never have been one input.
type literalIndex struct {
	files []fileIndex
	first map[string]Citation
	roots []RootStat
	// mask is every literal seen anywhere in the corpora.
	mask mask
}

// Present reports whether any corpus file contains the literal.
func (idx *literalIndex) Present(literal string) bool {
	_, ok := idx.first[literal]
	return ok
}

// Citation returns the first occurrence of the literal in the corpora, if any.
func (idx *literalIndex) Citation(literal string) (Citation, bool) {
	c, ok := idx.first[literal]
	return c, ok
}

// Files returns the corpus files in search order.
func (idx *literalIndex) Files() []fileIndex { return idx.files }

// Roots returns what each searched root contributed.
func (idx *literalIndex) Roots() []RootStat { return idx.roots }

// buildLiteralIndex searches every root for the given literals. Word-shaped
// literals are matched against whole identifiers so that 'to' does not match
// "total"; the rest are matched as substrings.
func buildLiteralIndex(repo string, roots []corpusRoot, lits *litTable) (*literalIndex, error) {
	literals := lits.order
	words := map[string]bool{}
	var punct []string
	for _, lit := range literals {
		if isWordLiteral(lit) {
			words[lit] = true
		} else if lit != "" {
			punct = append(punct, lit)
		}
	}
	sort.Strings(punct)

	idx := &literalIndex{first: map[string]Citation{}}
	for _, root := range roots {
		stat := RootStat{Name: root.Name, Dir: root.Dir}
		files, err := collectModelFiles(repo, root)
		if err != nil {
			return nil, err
		}
		for _, rel := range files {
			// #nosec G304 -- the corpus roots are fixed, under the repository named on the command line.
			data, err := os.ReadFile(filepath.Join(repo, root.Dir, rel))
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", rel, err)
			}
			path := filepath.ToSlash(filepath.Join(root.Dir, rel))
			stat.Files++
			stat.Lines += idx.scanFile(fileIndex{root: root.Name, path: path, lines: map[string]int{}},
				stripModelSource(string(data)), words, punct)
		}
		idx.roots = append(idx.roots, stat)
	}
	for i, file := range idx.files {
		idx.files[i].mask = lits.maskOf(file.has)
		idx.mask = idx.mask.or(idx.files[i].mask)
	}
	return idx, nil
}

// scanFile records occurrences in one already-stripped file and returns its line
// count.
func (idx *literalIndex) scanFile(file fileIndex, content string, words map[string]bool, punct []string) int {
	lines := strings.Split(content, "\n")
	for n, line := range lines {
		for _, word := range identifiers(line) {
			if words[word] {
				idx.record(file, word, n+1)
			}
		}
		for _, lit := range punct {
			if strings.Contains(line, lit) {
				idx.record(file, lit, n+1)
			}
		}
	}
	idx.files = append(idx.files, file)
	return len(lines)
}

// record notes the literal's first line in this file, and its first citation
// corpus-wide.
func (idx *literalIndex) record(file fileIndex, literal string, line int) {
	if _, ok := file.lines[literal]; !ok {
		file.lines[literal] = line
	}
	if _, ok := idx.first[literal]; !ok {
		idx.first[literal] = Citation{Literal: literal, Root: file.root, File: file.path, Line: line}
	}
}

// identifiers splits a line into identifier tokens.
func identifiers(line string) []string {
	var out []string
	for i := 0; i < len(line); {
		if !isIdentStart(line[i]) {
			i++
			continue
		}
		j := i
		for j < len(line) && isIdentPart(line[j]) {
			j++
		}
		out = append(out, line[i:j])
		i = j
	}
	return out
}

func isWordLiteral(lit string) bool {
	if lit == "" || !isIdentStart(lit[0]) {
		return false
	}
	for i := 0; i < len(lit); i++ {
		if !isIdentPart(lit[i]) {
			return false
		}
	}
	return true
}

// collectModelFiles returns the root's model files as sorted slash-separated
// paths relative to the root directory. A missing root yields none, so the tool
// runs before the optional corpora are downloaded.
func collectModelFiles(repo string, root corpusRoot) ([]string, error) {
	dir := filepath.Join(repo, root.Dir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			for _, skip := range root.Skip {
				if rel == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".sysml", ".kerml":
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// stripModelSource blanks out notes, documentation comments, string values and
// quoted names, keeping line structure. Without this, prose inside a `doc`
// comment would count as evidence for the keywords it happens to mention.
func stripModelSource(src string) string {
	out := []byte(src)
	blank := func(from, to int) {
		for i := from; i < to && i < len(out); i++ {
			if out[i] != '\n' {
				out[i] = ' '
			}
		}
	}
	for i := 0; i < len(src); {
		switch {
		case strings.HasPrefix(src[i:], "//*"), strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				blank(i, len(src))
				return string(out)
			}
			blank(i, i+2+end+2)
			i += 2 + end + 2
		case strings.HasPrefix(src[i:], "//"):
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				blank(i, len(src))
				return string(out)
			}
			blank(i, i+end)
			i += end
		case src[i] == '"', src[i] == '\'':
			width := quotedWidth(src[i:], src[i])
			blank(i, i+width)
			i += width
		default:
			i++
		}
	}
	return string(out)
}

// quotedWidth returns how many bytes the quoted run at the start of s occupies,
// or the rest of s when it is unterminated.
func quotedWidth(s string, quote byte) int {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		case '\n':
			// Neither strings nor quoted names span lines; an unterminated one is
			// a malformed fixture, not a reason to blank the rest of the file.
			return i
		}
	}
	return len(s)
}
