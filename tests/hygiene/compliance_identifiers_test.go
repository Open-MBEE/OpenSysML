package hygiene

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// complianceDocsDir holds the compliance tables whose Implementation column
// maps a rule to the Go code that carries it.
const complianceDocsDir = "docs/project"

// TestComplianceTablesNameLiveGoIdentifiers checks the Implementation column
// of every table under docs/project against the Go tree, so a deleted, renamed
// or moved function cannot stay cited. A cell is read left to right:
//
//   - a code span naming a `.go` file (`view/dot.go`, `lower/graph.go:lowerBlock`,
//     `runtime/exec.go:686`) opens a file context; the path is a suffix of a
//     tracked path, so package-relative spellings resolve, and a file cited
//     without any identifier must still exist;
//   - every backticked identifier that follows — `name`, `Type.member`,
//     `pkg.Name`, with or without a leading `:` or `*` — must be declared in
//     that file: a function, type, constant or variable; a method, field or
//     typed constant of its named type; an import of the file (`errors.As`) or
//     another package of the tree (`ast.Node`) resolved in that package;
//   - `Type.a`/`b` and `Decl` (`a`, `b`) list members of the preceding
//     declaration: under a type, `b` must be `Type.b` unless it is a top-level
//     declaration of the file in its own right (a list of sibling types or
//     constants), so a member of another type cannot stand in for a deleted
//     one; under a function, `b` is a name of its own;
//   - a bare word of prose closes the context, as does a code span that is
//     neither a Go file nor an identifier (a snippet, flag or message), so a
//     file's identifiers are the list that directly follows it. A
//     parenthetical holding prose is commentary and is skipped whole.
//
// Pilot, specification, client-library and wire names are therefore never
// checked without an allowlist: they are not backticked identifiers listed
// directly after a Go file, and a token with an underscore (an environment
// variable, protobuf field or emitted C name) is never a Go citation.
func TestComplianceTablesNameLiveGoIdentifiers(t *testing.T) {
	index := newGoDeclIndex(t)
	docs, err := filepath.Glob(filepath.Join(repoRoot, complianceDocsDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	checked, problems := 0, 0
	for _, doc := range docs {
		rel, _ := filepath.Rel(repoRoot, doc)
		for _, cell := range implementationCells(t, doc) {
			for _, c := range goCitations(cell.text) {
				checked++
				if problem := index.resolve(c); problem != "" {
					problems++
					t.Errorf("%s:%d: `%s` %s", rel, cell.line, c.display(), problem)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no Go citation found in any Implementation column; the table format or the tokenizer changed")
	}
	t.Logf("%d Go citations checked, %d stale", checked, problems)
}

// implementationCell is one row's Implementation column and its line number.
type implementationCell struct {
	line int
	text string
}

// implementationCells returns the Implementation cell of every row of every
// table in the file whose header has a column starting with "Implementation".
func implementationCells(t *testing.T, doc string) []implementationCell {
	t.Helper()
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	var cells []implementationCell
	column := -1
	for i, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			column = -1
			continue
		}
		row := splitTableRow(trimmed)
		if column == -1 {
			column = implementationColumn(row)
			if column == -1 {
				column = -2
			}
			continue
		}
		if column < 0 || isTableSeparator(row) || column >= len(row) {
			continue
		}
		cells = append(cells, implementationCell{line: i + 1, text: row[column]})
	}
	return cells
}

func implementationColumn(header []string) int {
	for i, cell := range header {
		if strings.HasPrefix(strings.Trim(cell, "* "), "Implementation") {
			return i
		}
	}
	return -1
}

func isTableSeparator(row []string) bool {
	for _, cell := range row {
		if strings.Trim(cell, ":- ") != "" {
			return false
		}
	}
	return true
}

// splitTableRow splits a Markdown table row at the pipes outside code spans.
func splitTableRow(row string) []string {
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	var cells []string
	var cell strings.Builder
	inCode := false
	for _, r := range row {
		switch {
		case r == '`':
			inCode = !inCode
		case r == '|' && !inCode:
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
			continue
		}
		cell.WriteRune(r)
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

// goCitation is one identifier an Implementation cell names in a Go file.
type goCitation struct {
	file   string   // the file path as the cell spells it, a suffix of a tracked path
	name   string   // the token as written: `name`, `Type.member` or `pkg.Name`
	names  []string // what it may resolve as: the token, and `Parent.token` in a member list
	parent string   // the `Type` a `Type` (`member`) list or `Type.a`/`b` chain names
}

func (c goCitation) display() string {
	if c.name == "" {
		return c.file
	}
	return c.file + ":" + c.name
}

var (
	goFileToken = regexp.MustCompile(`^((?:[\w.-]+/)*[\w-]+\.go)(?::(.+))?$`)
	identToken  = regexp.MustCompile(`^:?\*?([A-Za-z]\w*(?:\.[A-Za-z]\w*){0,2})$`)
)

// goReserved is what a backticked token can never cite: Go keywords and
// predeclared identifiers.
var goReserved = map[string]bool{"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true, "defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true, "goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true, "range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true, "var": true, "true": true, "false": true, "nil": true, "iota": true, "string": true, "int": true, "bool": true, "error": true, "any": true}

// goCitations tokenizes a cell and returns the identifiers listed under a Go
// file context, as the test's doc comment describes.
func goCitations(cell string) []goCitation {
	var out []goCitation
	file, last, listParent := "", "", ""
	members := -1 // index of the parenthesis closing the member list being read
	tokens := cellTokens(cell)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.kind == tokenOpen {
			end := matchingClose(tokens, i)
			if hasProse(tokens[i+1 : end]) {
				i = end
				continue
			}
			if members < 0 && last != "" && tokens[i-1].kind == tokenCode {
				members, listParent = end, last
			}
			continue
		}
		if i == members {
			members, listParent = -1, ""
		}
		if tok.kind == tokenWord {
			if !isSeparator(tok.text) {
				file, last = "", ""
			}
			continue
		}
		if tok.kind == tokenClose {
			continue
		}
		text := strings.TrimSuffix(tok.text, ":")
		if m := goFileToken.FindStringSubmatch(text); m != nil {
			file, last = m[1], ""
			if im := identToken.FindStringSubmatch(m[2]); im != nil {
				out = append(out, goCitation{file: file, name: im[1], names: []string{im[1]}})
				last = im[1]
			} else {
				out = append(out, goCitation{file: file})
			}
			continue
		}
		if strings.Contains(text, "_") {
			continue
		}
		m := identToken.FindStringSubmatch(text)
		if m == nil || goReserved[m[1]] {
			file, last = "", ""
			continue
		}
		if file != "" {
			c := goCitation{file: file, name: m[1], names: []string{m[1]}}
			parent := ""
			if members >= 0 {
				parent = listParent
			} else if i > 0 && tokens[i-1].kind == tokenWord && tokens[i-1].text == "/" {
				parent = typePrefix(last)
			}
			if parent != "" && !strings.Contains(m[1], ".") {
				c.parent = parent
				c.names = append(c.names, parent+"."+m[1])
			}
			out = append(out, c)
		}
		last = m[1]
	}
	return out
}

// typePrefix keeps `Type` of a `Type.member` parent for a `/`-joined sibling.
func typePrefix(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[:i]
	}
	return ""
}

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenCode
	tokenOpen
	tokenClose
	tokenEnd
)

type cellToken struct {
	kind tokenKind
	text string
}

// cellTokens splits a cell into code spans, parentheses, and whitespace- or
// punctuation-delimited bare words, ending with a sentinel.
func cellTokens(cell string) []cellToken {
	var tokens []cellToken
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, cellToken{kind: tokenWord, text: word.String()})
			word.Reset()
		}
	}
	for i := 0; i < len(cell); i++ {
		switch c := cell[i]; {
		case c == '`':
			flush()
			end := strings.IndexByte(cell[i+1:], '`')
			if end < 0 {
				i = len(cell)
				continue
			}
			tokens = append(tokens, cellToken{kind: tokenCode, text: strings.TrimSpace(cell[i+1 : i+1+end])})
			i += end + 1
		case c == '(':
			flush()
			tokens = append(tokens, cellToken{kind: tokenOpen})
		case c == ')':
			flush()
			tokens = append(tokens, cellToken{kind: tokenClose})
		case unicode.IsSpace(rune(c)), c == ',', c == ';':
			flush()
		default:
			word.WriteByte(c)
		}
	}
	flush()
	return append(tokens, cellToken{kind: tokenEnd})
}

// matchingClose returns the index of the parenthesis closing tokens[open], or
// of the end sentinel when it is unbalanced.
func matchingClose(tokens []cellToken, open int) int {
	depth := 0
	for i := open; i < len(tokens); i++ {
		switch tokens[i].kind {
		case tokenOpen:
			depth++
		case tokenClose:
			depth--
			if depth == 0 {
				return i
			}
		case tokenEnd:
			return i
		}
	}
	return len(tokens) - 1
}

func hasProse(tokens []cellToken) bool {
	for _, tok := range tokens {
		if tok.kind == tokenWord && !isSeparator(tok.text) {
			return true
		}
	}
	return false
}

// isSeparator accepts the punctuation that joins listed identifiers: slashes,
// dashes, colons, ampersands and `<br>` line breaks.
func isSeparator(word string) bool {
	return strings.Trim(word, "/<>br-–—:&") == ""
}

// goDeclIndex resolves citations against the tracked Go files, parsing each
// cited package once.
type goDeclIndex struct {
	t        *testing.T
	tracked  []string                 // every tracked .go path, repo-relative
	byDir    map[string]*packageDecls // parsed packages by directory
	pkgNames map[string][]string      // directory basename → directories
}

// packageDecls is what one directory of Go files declares: top-level names
// and `Type.member` names, the same per file, and each file's imports.
type packageDecls struct {
	names   map[string]bool
	types   map[string]bool
	top     map[string]map[string]bool // file → names declared at top level, members excluded
	byFile  map[string]map[string]bool
	imports map[string]map[string]string // file → import name → import path
}

func newGoDeclIndex(t *testing.T) *goDeclIndex {
	idx := &goDeclIndex{t: t, byDir: map[string]*packageDecls{}, pkgNames: map[string][]string{}}
	for _, p := range trackedFiles(t) {
		if strings.HasSuffix(p, ".go") {
			idx.tracked = append(idx.tracked, p)
			dir := path.Dir(p)
			base := path.Base(dir)
			if !contains(idx.pkgNames[base], dir) {
				idx.pkgNames[base] = append(idx.pkgNames[base], dir)
			}
		}
	}
	return idx
}

// resolve returns "" when the citation names a declaration in its file, else
// a message saying what is wrong and the nearest existing name.
func (idx *goDeclIndex) resolve(c goCitation) string {
	var files []string
	for _, p := range idx.tracked {
		if p == c.file || strings.HasSuffix(p, "/"+c.file) {
			files = append(files, p)
		}
	}
	if len(files) == 0 {
		return "names no tracked Go file"
	}
	if c.name == "" {
		return ""
	}
	var nearest []string
	inPackage, memberOf := "", ""
	for _, f := range files {
		pkg := idx.load(path.Dir(f))
		names := c.names
		if c.parent != "" && pkg.types[c.parent] && !pkg.top[f][c.name] {
			names, memberOf = names[1:], c.parent // a member list under a type names its members, not another type's
		}
		for _, name := range names {
			if pkg.byFile[f][name] || idx.qualified(pkg, f, name) || len(idx.pkgNames[name]) > 0 {
				return ""
			}
			if pkg.names[name] {
				inPackage = idx.declaringFile(pkg, name)
			}
		}
		for n := range pkg.names {
			nearest = append(nearest, n)
		}
	}
	if memberOf != "" {
		return "is not a member of " + memberOf + " in the cited file; nearest existing name: " + nearestName(memberOf+"."+c.name, nearest)
	}
	if inPackage != "" {
		return "is declared in " + inPackage + ", not in the cited file"
	}
	if elsewhere := idx.declaredElsewhere(c.names); elsewhere != "" {
		return "is declared in " + elsewhere + ", not in the cited file"
	}
	return "is not declared in the cited file; nearest existing name: " + nearestName(c.name, nearest)
}

// qualified accepts `pkg.Name` when pkg is an import of the cited file (any
// name of an external package; a declared name of a tree package), the cited
// package itself, or a tree package by directory name; and `pkg.Type.member`
// the same way.
func (idx *goDeclIndex) qualified(pkg *packageDecls, file, name string) bool {
	first, rest, ok := strings.Cut(name, ".")
	if !ok {
		return false
	}
	if importPath, imported := pkg.imports[file][first]; imported {
		dir, inTree := strings.CutPrefix(importPath, modulePath)
		if !inTree {
			return true
		}
		return idx.load(dir).names[rest]
	}
	if first == path.Base(path.Dir(file)) && pkg.names[rest] {
		return true
	}
	for _, dir := range idx.pkgNames[first] {
		if idx.load(dir).names[rest] {
			return true
		}
	}
	return false
}

// declaredElsewhere names the tracked files declaring any of names in any
// package.
func (idx *goDeclIndex) declaredElsewhere(names []string) string {
	var files []string
	seen := map[string]bool{}
	for _, p := range idx.tracked {
		dir := path.Dir(p)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		pkg := idx.load(dir)
		for _, name := range names {
			if pkg.names[name] {
				files = append(files, idx.declaringFile(pkg, name))
			}
		}
	}
	sort.Strings(files)
	return strings.Join(files, ", ")
}

func (idx *goDeclIndex) declaringFile(pkg *packageDecls, name string) string {
	var files []string
	for f, names := range pkg.byFile {
		if names[name] {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return strings.Join(files, ", ")
}

// load parses every tracked Go file of dir once.
func (idx *goDeclIndex) load(dir string) *packageDecls {
	if pkg, ok := idx.byDir[dir]; ok {
		return pkg
	}
	pkg := &packageDecls{names: map[string]bool{}, types: map[string]bool{}, top: map[string]map[string]bool{}, byFile: map[string]map[string]bool{}, imports: map[string]map[string]string{}}
	idx.byDir[dir] = pkg
	fset := token.NewFileSet()
	for _, p := range idx.tracked {
		if path.Dir(p) != dir {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot, p), nil, parser.SkipObjectResolution)
		if err != nil {
			idx.t.Fatalf("parse %s: %v", p, err)
		}
		names := map[string]bool{}
		for _, decl := range file.Decls {
			for _, n := range declaredNames(decl) {
				names[n] = true
				pkg.names[n] = true
			}
			for _, n := range declaredTypes(decl) {
				pkg.types[n] = true
			}
			for _, n := range topLevelNames(decl) {
				if pkg.top[p] == nil {
					pkg.top[p] = map[string]bool{}
				}
				pkg.top[p][n] = true
			}
		}
		pkg.byFile[p] = names
		imports := map[string]string{}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			name := path.Base(importPath)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			imports[name] = importPath
		}
		pkg.imports[p] = imports
	}
	return pkg
}

// topLevelNames lists the functions, types, constants and variables a
// declaration introduces by their own name: no methods, fields or `Type.name`.
func topLevelNames(decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil || len(d.Recv.List) == 0 {
			return []string{d.Name.Name}
		}
	case *ast.GenDecl:
		var names []string
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.ValueSpec:
				for _, n := range s.Names {
					names = append(names, n.Name)
				}
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
			}
		}
		return names
	}
	return nil
}

// declaredTypes lists the type names a declaration introduces.
func declaredTypes(decl ast.Decl) []string {
	d, ok := decl.(*ast.GenDecl)
	if !ok {
		return nil
	}
	var names []string
	for _, spec := range d.Specs {
		if s, ok := spec.(*ast.TypeSpec); ok {
			names = append(names, s.Name.Name)
		}
	}
	return names
}

// declaredNames lists what a top-level declaration names: functions; methods
// as `Type.method` and bare; types with their struct fields and interface
// methods as `Type.member` and bare; constants and variables, typed ones also
// as `Type.name`.
func declaredNames(decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil || len(d.Recv.List) == 0 {
			return []string{d.Name.Name}
		}
		return []string{receiverTypeName(d.Recv.List[0].Type) + "." + d.Name.Name, d.Name.Name}
	case *ast.GenDecl:
		var names []string
		typeName := ""
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.ValueSpec:
				if s.Type != nil {
					typeName = receiverTypeName(s.Type)
				} else if len(s.Values) > 0 {
					typeName = ""
				}
				for _, n := range s.Names {
					names = append(names, n.Name)
					if typeName != "" {
						names = append(names, typeName+"."+n.Name)
					}
				}
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
				names = append(names, memberNames(s.Name.Name, s.Type)...)
			}
		}
		return names
	}
	return nil
}

func memberNames(typeName string, expr ast.Expr) []string {
	var fields *ast.FieldList
	switch t := expr.(type) {
	case *ast.StructType:
		fields = t.Fields
	case *ast.InterfaceType:
		fields = t.Methods
	}
	if fields == nil {
		return nil
	}
	var names []string
	for _, f := range fields.List {
		for _, n := range f.Names {
			names = append(names, typeName+"."+n.Name, n.Name)
		}
	}
	return names
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// nearestName picks the candidate closest to name: a case-insensitive match
// first, else the smallest edit distance.
func nearestName(name string, candidates []string) string {
	best, bestDistance := "", -1
	for _, c := range candidates {
		if strings.EqualFold(c, name) {
			return c
		}
		if d := editDistance(name, c); bestDistance < 0 || d < bestDistance || d == bestDistance && c < best {
			best, bestDistance = c, d
		}
	}
	if best == "" {
		return "(none)"
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
