package hygiene

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/enginecontract"
)

// engineContractPkgs loads the extension package set from the vendored
// library: every file stem under the vendored directory names one package.
func engineContractPkgs(t *testing.T) []string {
	t.Helper()
	pkgs, err := enginecontract.Packages(libs.DefaultSource())
	if err != nil {
		t.Fatalf("%v", err)
	}
	return pkgs
}

// stringConsts collects the package-level string constants of every file so
// identifiers naming them fold too: `"Pkg::" + suffix` folds when Pkg's
// prefix is a const declared beside it.
func stringConsts(files map[string][]*ast.File) map[*ast.File]map[string]string {
	out := map[*ast.File]map[string]string{}
	for _, fset := range files {
		consts := map[string]string{}
		for _, file := range fset {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								consts[name.Name] = unquote(lit.Value)
							}
						}
					}
				}
			}
			out[file] = consts
		}
	}
	return out
}

func unquote(lit string) string {
	if len(lit) >= 2 && lit[0] == '"' {
		return lit[1 : len(lit)-1]
	}
	return lit
}

// foldString reduces a string expression to its value when it is foldable:
// BasicLits, parenthesized forms, `+` of foldable operands, and identifiers
// naming a package-level string const of the same package.
func foldString(expr ast.Expr, consts map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return unquote(e.Value), true
		}
	case *ast.ParenExpr:
		return foldString(e.X, consts)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			left, lok := foldString(e.X, consts)
			right, rok := foldString(e.Y, consts)
			if lok && rok {
				return left + right, true
			}
		}
	case *ast.Ident:
		if v, ok := consts[e.Name]; ok {
			return v, true
		}
	}
	return "", false
}

// derivedNames extracts every qualified name a string binds under one of the
// extension packages: full `Pkg::Name` chains, a string equal to a package
// name (a `package` binding), and `Pkg::` prefix operands of a concatenation
// (a composed prefix, whose composed names cannot be checked one by one).
func derivedNames(value string, packages map[string]bool, names, prefixes map[string]bool) {
	for _, m := range qnameRe.FindAllString(value, -1) {
		pkg, _, _ := strings.Cut(m, "::")
		if packages[pkg] {
			names[m] = true
		}
	}
	if packages[value] {
		names[value] = true
	}
	for pkg := range packages {
		if strings.HasSuffix(value, pkg+"::") {
			prefixes[pkg] = true
		}
	}
}

var qnameRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)(::[A-Za-z_][A-Za-z0-9_]*)+`)

// TestEngineContractCoversEveryBoundName derives, from every non-test Go file
// under internal/, cmd/ and tools/, the extension-library qualified names the
// code binds by string, and requires each to be a manifest entry; a manifest
// entry no code derives is stale unless its package composes names under a
// `Pkg::` prefix, which the derivation cannot enumerate.
func TestEngineContractCoversEveryBoundName(t *testing.T) {
	packageList := engineContractPkgs(t)
	packages := map[string]bool{}
	for _, p := range packageList {
		packages[p] = true
	}
	manifest, err := enginecontract.Load(libs.DefaultSource())
	if err != nil {
		t.Fatalf("%v", err)
	}
	entries := manifest.ByName()

	// Parse every non-test .go file under internal/, cmd/ and tools/,
	// collecting package-level string consts before folding.
	roots := []string{"../../internal", "../../cmd", "../../tools"}
	files := map[string][]*ast.File{}
	fset := token.NewFileSet()
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				return err
			}
			files[filepath.Dir(p)] = append(files[filepath.Dir(p)], file)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	constsByFile := stringConsts(files)

	names := map[string]bool{}
	prefixes := map[string]bool{}
	for _, fsetFiles := range files {
		for _, file := range fsetFiles {
			consts := constsByFile[file]
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if ok && lit.Kind == token.STRING {
					derivedNames(unquote(lit.Value), packages, names, prefixes)
					return true
				}
				bin, ok := n.(*ast.BinaryExpr)
				if ok && bin.Op == token.ADD {
					if v, folded := foldString(bin, consts); folded {
						derivedNames(v, packages, names, prefixes)
					}
					// Non-foldable concatenations still contribute their
					// foldable operands as prefixes and strings.
					for _, side := range []ast.Expr{bin.X, bin.Y} {
						if v, folded := foldString(side, consts); folded {
							derivedNames(v, packages, names, prefixes)
						}
					}
				}
				ident, ok := n.(*ast.Ident)
				if ok {
					if v, c := consts[ident.Name]; c {
						derivedNames(v, packages, names, prefixes)
					}
				}
				return true
			})
		}
	}

	update := "update engine-contract.json upstream in Open-MBEE/OpenSysML-Extensions-Library, bump scripts/extension-libraries-pin.sh, and run scripts/sync-extension-libraries.sh"

	sortedNames := make([]string, 0, len(names))
	for n := range names {
		sortedNames = append(sortedNames, n)
	}
	sort.Strings(sortedNames)
	for _, n := range sortedNames {
		if _, ok := entries[n]; !ok {
			t.Errorf("OpenSysML binds %s but engine-contract.json does not list it; %s", n, update)
		}
	}
	prefixList := make([]string, 0, len(prefixes))
	for p := range prefixes {
		prefixList = append(prefixList, p)
	}
	sort.Strings(prefixList)
	t.Logf("packages composing names under a Pkg:: prefix: %v", prefixList)
	for _, e := range manifest.Entries {
		if names[e.Name] {
			continue
		}
		pkg, _, _ := strings.Cut(e.Name, "::")
		if prefixes[pkg] {
			continue
		}
		t.Errorf("engine-contract.json lists %s but no code derives it; stale entries come out upstream; %s", e.Name, update)
	}
}
