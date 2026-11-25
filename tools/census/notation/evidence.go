package notation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A citation is `path:Name`: a Go file and the function or method it declares,
// a TypeScript file and the function it declares, or a golden file on its own.
var (
	goCite     = regexp.MustCompile(`^([A-Za-z0-9_./-]+\.go):([A-Za-z_][A-Za-z0-9_]*)$`)
	tsCite     = regexp.MustCompile(`^([A-Za-z0-9_./-]+\.ts):([A-Za-z_][A-Za-z0-9_]*)$`)
	goldenCite = regexp.MustCompile(`^[A-Za-z0-9_./-]+/testdata/[A-Za-z0-9_.-]+\.golden$`)
)

func sortStrings(s []string) { sort.Strings(s) }

// declarations parses each cited Go file once.
type declarations struct {
	root  string
	files map[string]map[string]bool
}

func newDeclarations(root string) *declarations {
	return &declarations{root: root, files: map[string]map[string]bool{}}
}

// declared reports whether file declares a function or method named name.
func (d *declarations) declared(file, name string) (bool, error) {
	names, ok := d.files[file]
	if !ok {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, filepath.Join(d.root, filepath.FromSlash(file)), nil, parser.SkipObjectResolution)
		if err != nil {
			return false, err
		}
		names = map[string]bool{}
		for _, decl := range parsed.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				names[fn.Name.Name] = true
			}
		}
		d.files[file] = names
	}
	return names[name], nil
}

// resolve checks one citation against the repository.
func (d *declarations) resolve(cite string, test bool) error {
	if m := goCite.FindStringSubmatch(cite); m != nil {
		if test != strings.HasSuffix(m[1], "_test.go") || test && !strings.HasPrefix(m[2], "Test") {
			return fmt.Errorf("%s: an implementation cites a non-test function, a test cites func TestX in a _test.go file", cite)
		}
		ok, err := d.declared(m[1], m[2])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s declares no function %s", m[1], m[2])
		}
		return nil
	}
	if m := tsCite.FindStringSubmatch(cite); m != nil && !test {
		content, err := os.ReadFile(filepath.Join(d.root, filepath.FromSlash(m[1]))) // #nosec G304 -- the path is a citation under the repository root
		if err != nil {
			return err
		}
		if !regexp.MustCompile(`\bfunction ` + m[2] + `\b`).Match(content) {
			return fmt.Errorf("%s declares no function %s", m[1], m[2])
		}
		return nil
	}
	if test && goldenCite.MatchString(cite) {
		if _, err := os.Stat(filepath.Join(d.root, filepath.FromSlash(cite))); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("%q is not a citation: want file.go:Func, file.ts:func, file_test.go:TestX or a testdata/*.golden path", cite)
}

// CheckEvidence resolves every citation of the drawn and approximate rows and
// checks their forms; the problems are sorted by production.
func CheckEvidence(root string, rows []Row) []string {
	decls := newDeclarations(root)
	known := map[string]bool{}
	for _, f := range Forms {
		known[f] = true
	}
	var problems []string
	for _, r := range rows {
		switch r.Verdict {
		case VerdictDrawn, VerdictApproximate:
		default:
			if r.Implementation != "" || r.Test != "" || len(r.Forms) > 0 {
				problems = append(problems, fmt.Sprintf("%s: a %s row carries evidence", r.Name, r.Verdict))
			}
			continue
		}
		if r.Implementation == "" || r.Test == "" || len(r.Forms) == 0 {
			problems = append(problems, fmt.Sprintf("%s: a %s row needs an implementation, forms and a test", r.Name, r.Verdict))
			continue
		}
		if err := decls.resolve(r.Implementation, false); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", r.Name, err))
		}
		if err := decls.resolve(r.Test, true); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", r.Name, err))
		}
		for _, f := range r.Forms {
			if !known[f] {
				problems = append(problems, fmt.Sprintf("%s: unknown form %q", r.Name, f))
			}
		}
	}
	sort.Strings(problems)
	return problems
}
