package specexamples

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Fixture is one example completed into a file.
type Fixture struct {
	Block Block
	// Imports are the standard-library members the completion imports, as
	// `Package::Name`, because the example uses the bare name without declaring it.
	Imports []string
	Content string
}

// PackageName is the package a fixture wraps its example in, unique across the
// corpus so one batch can hold every fixture.
func (b Block) PackageName() string {
	return fmt.Sprintf("'%s %s example %d'", strings.ToUpper(b.Document[:1])+b.Document[1:], b.Clause, b.Index)
}

// actionBodyKeywords open the members the grammar admits only in an action body
// (SysML 8.2.2.17.1 ActionBodyItem): action nodes and the initial node.
var actionBodyKeywords = map[string]bool{
	"merge": true, "decide": true, "join": true, "fork": true, "then": true,
	"send": true, "accept": true, "assign": true, "terminate": true,
	"if": true, "while": true, "loop": true, "for": true,
}

// IsActionBody reports whether the example is a fragment of an action body: a
// top-level statement opens with an action-node keyword or a `then` target
// succession, or is `first x;` naming an initial node rather than the
// `first x then y;` succession that a package may own.
func (b Block) IsActionBody() bool {
	depth := 0
	var stmt strings.Builder
	var comments commentStripper
	opens := func() bool {
		fields := strings.Fields(stmt.String())
		stmt.Reset()
		if depth != 0 || len(fields) == 0 {
			return false
		}
		return actionBodyKeywords[fields[0]] || fields[0] == "first" && !slices.Contains(fields, "then")
	}
	for _, l := range b.Lines {
		for _, r := range comments.strip(l) {
			switch r {
			case '{':
				if opens() {
					return true
				}
				depth++
			case '}':
				stmt.Reset()
				depth--
			case ';':
				if opens() {
					return true
				}
			default:
				stmt.WriteRune(r)
			}
		}
		stmt.WriteByte(' ')
	}
	return false
}

// Wrapper is the declaration the completion wraps the example in.
func (b Block) Wrapper() string {
	if b.IsActionBody() {
		return "action def"
	}
	return "package"
}

// Render writes the fixture text: a two-line header stating provenance and the
// completion, then the example wrapped in its package (or, for an action body
// fragment, an action definition) behind the imports.
func Render(title, release string, b Block, imports []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "// %s (release %s), clause %s, page %s, example %d.\n", title, release, b.Clause, b.Page, b.Index)
	wrapper := "a package"
	if b.IsActionBody() {
		wrapper = "an action definition (the example is an action body)"
	}
	if len(imports) == 0 {
		fmt.Fprintf(&sb, "// Completion: wrapped in %s; no import added.\n", wrapper)
	} else {
		fmt.Fprintf(&sb, "// Completion: wrapped in %s; imports %s for names the example leaves undeclared.\n",
			wrapper, strings.Join(imports, ", "))
	}
	fmt.Fprintf(&sb, "%s %s {\n", b.Wrapper(), b.PackageName())
	for _, imp := range imports {
		fmt.Fprintf(&sb, "    private import %s;\n", imp)
	}
	if len(imports) > 0 {
		sb.WriteString("\n")
	}
	for _, l := range b.Lines {
		if l == "" {
			sb.WriteString("\n")
			continue
		}
		sb.WriteString("    " + l + "\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

// Complete renders every block, then adds, for each bare name the example uses
// without declaring, a private import of the one standard-library member of that
// name when exactly one exists. The names come from the name-resolution pass
// over the wrapped examples opened together as one batch, the way the gate
// reads them; a name no library member or more than one declares stays as the
// specification wrote it.
func Complete(title, release string, blocks []Block) []Fixture {
	fixtures := make([]Fixture, len(blocks))
	ws := model.NewWorkspace()
	contents := make(map[string][]byte, len(blocks))
	for i, b := range blocks {
		fixtures[i].Block = b
		fixtures[i].Content = Render(title, release, b, nil)
		contents[b.Path()] = []byte(fixtures[i].Content)
		ws.Open(b.Path(), contents[b.Path()], 1)
	}
	lib, packages := libraryPackages()
	for i := range fixtures {
		path := fixtures[i].Block.Path()
		text := contents[path]
		seen := map[string]bool{}
		for _, d := range ws.Diagnostics(path) {
			if d.Code != "unresolved" || d.Span.End() > len(text) {
				continue
			}
			name := string(text[d.Span.Offset:d.Span.End()])
			if !source.IsIdentifier(name) || seen[name] {
				continue
			}
			seen[name] = true
			if fqn, ok := libraryMember(lib, packages, name, inUnitBrackets(text, d.Span.Offset)); ok {
				fixtures[i].Imports = append(fixtures[i].Imports, fqn)
			}
		}
		sort.Strings(fixtures[i].Imports)
		if len(fixtures[i].Imports) > 0 {
			fixtures[i].Content = Render(title, release, fixtures[i].Block, fixtures[i].Imports)
		}
	}
	return fixtures
}

// inUnitBrackets reports whether the reference at offset follows a `[`, the
// position of a unit in a quantity expression such as `0[kg]`.
func inUnitBrackets(text []byte, offset int) bool {
	for i := offset - 1; i >= 0; i-- {
		switch text[i] {
		case ' ', '\t':
			continue
		case '[':
			return true
		default:
			return false
		}
	}
	return false
}

// extensionLibraries holds the OpenSysML extension libraries, which the
// specification's examples cannot assume.
const extensionLibraries = "OpenSysML Libraries/"

// libraryPackages returns the shared standard-library index with the names of
// its top-level standard packages, read from the library text it was built from.
func libraryPackages() (*symbols.Index, []string) {
	idx, src := libs.SharedLibrary()
	var packages []string
	for _, name := range src.List() {
		if strings.HasPrefix(name, extensionLibraries) {
			continue
		}
		content, err := src.Read(name)
		if err != nil {
			continue
		}
		for _, m := range libs.TopMembers(parser.New(source.New(name, content)).ParseFile()) {
			if strings.HasSuffix(m.Kind, "package") && m.Name != "" {
				packages = append(packages, m.Name)
			}
		}
	}
	sort.Strings(packages)
	return idx, packages
}

// libraryMember returns `Package::name` when exactly one top-level library
// package declares a direct member called name; a member whose short name is
// name counts only for a unit reference, where `0[kg]` and `5[min]` are read.
func libraryMember(lib *symbols.Index, packages []string, name string, unit bool) (string, bool) {
	var found []string
	for _, pkg := range packages {
		for _, sym := range lib.LookupDirectChildrenNamed(pkg, name) {
			if sym.Name != name && !unit {
				continue
			}
			// Visible through an import is not declared there.
			if lib.Declaring(pkg+"::"+sym.Name) == sym {
				found = append(found, pkg+"::"+name)
				break
			}
		}
	}
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}
