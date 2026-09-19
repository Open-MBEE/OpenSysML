package hygiene

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const (
	exportPkg  = "github.com/Open-MBEE/OpenSysML/internal/core/export"
	convertPkg = "github.com/Open-MBEE/OpenSysML/internal/core/convert"
	migratePkg = "github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// packageDeps lists the transitive dependencies of a package, as go list -deps does.
func packageDeps(t *testing.T, pkg string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", pkg)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}
	return strings.Fields(string(out))
}

// directImports maps each package under internal/ and cmd/ to what it imports.
func directImports(t *testing.T) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./internal/...", "./cmd/...")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	imports := make(map[string][]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		imports[fields[0]] = fields[1:]
	}
	return imports
}

// The RDF mapping translates between a parsed tree and a graph; it does not
// migrate SysML v1 XMI, and the conversion entry point above it does.
func TestExportDoesNotDependOnMigration(t *testing.T) {
	deps := packageDeps(t, "./internal/core/export")
	if slices.Contains(deps, migratePkg) {
		t.Errorf("internal/core/export depends on internal/core/migrate; migration belongs to internal/core/convert")
	}
	if slices.Contains(deps, convertPkg) {
		t.Errorf("internal/core/export depends on internal/core/convert, which sits above it")
	}
}

// Conversions are driven from internal/core/convert: the mapping packages do
// not import it, and only it and the CLI (which prints the migration report)
// import the migration directly.
func TestConversionEntryPointOwnsTheOrchestration(t *testing.T) {
	imports := directImports(t)
	want := []string{"github.com/Open-MBEE/OpenSysML/cmd/sysml", convertPkg}
	var got []string
	for pkg, deps := range imports {
		if pkg != migratePkg && slices.Contains(deps, migratePkg) {
			got = append(got, pkg)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("packages importing internal/core/migrate = %v, want %v", got, want)
	}
	for _, pkg := range []string{exportPkg, migratePkg, "github.com/Open-MBEE/OpenSysML/internal/core/rdf"} {
		if slices.Contains(imports[pkg], convertPkg) {
			t.Errorf("%s imports internal/core/convert, which drives it", pkg)
		}
	}
}
