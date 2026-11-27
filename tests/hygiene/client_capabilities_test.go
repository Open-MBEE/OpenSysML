package hygiene

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// capabilityChecklistDoc holds the table of service capabilities against client
// bindings, under the section capabilityChecklistHeading.
const (
	capabilityChecklistDoc     = "docs/reference/client-parity.md"
	capabilityChecklistHeading = "## Capabilities"
)

// capabilityClient is one column of the checklist: the client's capabilities
// module, and how a declaration of one capability reads there.
type capabilityClient struct {
	name string
	path string
	// declares reports whether the module declares the capability, given its wire
	// name and the suffix of the service's Go identifier (InstancePaths).
	declares func(source, name, ident string) bool
	// usedIn is the package directory whose other sources must gate or decode by a
	// declared capability for it to be bound; empty holds the client to the declaration alone.
	usedIn string
}

func quotedLiteral(source, name, _ string) bool {
	return strings.Contains(source, `"`+name+`"`)
}

func singleQuotedLiteral(source, name, _ string) bool {
	return strings.Contains(source, `'`+name+`'`)
}

func reExportedConstant(source, _, ident string) bool {
	return regexp.MustCompile(`=\s*sysmlgrpc\.Capability` + ident + `\b`).MatchString(source)
}

// stringConstant matches a `const string X = "name";` declaration, so a name the
// module's documentation quotes is not taken for a declaration.
func stringConstant(source, name, _ string) bool {
	return regexp.MustCompile(`\bconst\s+string\s+\w+\s*=\s*"` + regexp.QuoteMeta(name) + `"\s*;`).MatchString(source)
}

var capabilityClients = []capabilityClient{
	{"Python", "client/python/opensysml/capabilities.py", quotedLiteral, ""},
	{"Go", "client/opensysml/types.go", reExportedConstant, "client/opensysml"},
	{"Node", "client/node/src/core/capabilities.ts", quotedLiteral, ""},
	{"Java", "client/java/opensysml-client/src/main/java/org/openmbee/opensysml/Capabilities.java", quotedLiteral, ""},
	{"Rust", "client/rust/opensysml/src/capabilities.rs", quotedLiteral, ""},
	{"Julia", "client/julia/OpenSysML/src/capabilities.jl", quotedLiteral, ""},
	{"MATLAB", "client/matlab/+opensysml/capabilities.m", singleQuotedLiteral, ""},
	{".NET", "client/dotnet/src/OpenMBEE.OpenSysML/Capabilities.cs", stringConstant, ""},
}

// capabilityConstPattern reads a service capability declaration,
// `CapabilityInstancePaths = "instance_paths"`, into its identifier and name.
var capabilityConstPattern = regexp.MustCompile(`\bCapability(\w+)\s*=\s*"([a-z][a-z_0-9]*)"`)

// TestClientCapabilityChecklist holds the capability table of client-parity.md to the code: one row per
// announced capability, each cell `bound` exactly when the client declares the name and, for Go, uses it.
func TestClientCapabilityChecklist(t *testing.T) {
	identifiers := serviceCapabilityIdentifiers(t)
	announced := sysmlgrpc.Capabilities()
	if len(announced) == 0 {
		t.Fatal("the service announces no capabilities")
	}
	sources := make(map[string]string, len(capabilityClients))
	for _, client := range capabilityClients {
		source, err := os.ReadFile(filepath.Join(repoRoot, client.path))
		if err != nil {
			t.Fatalf("%s client: %v", client.name, err)
		}
		sources[client.name] = string(source)
	}
	uses := make(map[string]string, len(capabilityClients))
	for _, client := range capabilityClients {
		if client.usedIn != "" {
			uses[client.name] = packageSourcesBeside(t, client.usedIn, client.path)
		}
	}

	rows, columns := readCapabilityChecklist(t)
	wantColumns := make([]string, 0, len(capabilityClients))
	for _, client := range capabilityClients {
		wantColumns = append(wantColumns, client.name)
	}
	if strings.Join(columns, ",") != strings.Join(wantColumns, ",") {
		t.Fatalf("%s: table columns are %v, want %v", capabilityChecklistDoc, columns, wantColumns)
	}

	listed := make(map[string]bool, len(rows))
	for name := range rows {
		listed[name] = true
	}
	for _, name := range announced {
		if !listed[name] {
			t.Errorf("%s: the service announces %q but the table has no row for it", capabilityChecklistDoc, name)
		}
	}
	isAnnounced := make(map[string]bool, len(announced))
	for _, name := range announced {
		isAnnounced[name] = true
	}
	for _, name := range sortedKeys(rows) {
		// The table is the full build's: a build without the migration still holds its row to the clients.
		if !isAnnounced[name] && (migrationLinked || name != sysmlgrpc.CapabilityMigrate) {
			t.Errorf("%s: the table lists %q, which the service does not announce", capabilityChecklistDoc, name)
			continue
		}
		ident, ok := identifiers[name]
		if !ok {
			t.Fatalf("no CapabilityX constant declares %q under internal/frontend/grpc", name)
		}
		cells := rows[name]
		for i, client := range capabilityClients {
			cell := cells[i]
			if cell != "bound" && cell != "not bound" {
				t.Errorf("%s: %s/%s reads %q, want bound or not bound", capabilityChecklistDoc, name, client.name, cell)
				continue
			}
			declared := client.declares(sources[client.name], name, ident)
			used := client.usedIn == "" || capabilityUsed(ident, uses[client.name])
			switch {
			case declared && used && cell != "bound":
				t.Errorf("%s declares %q but the table says %s does not bind it", client.path, name, client.name)
			case declared && !used && cell != "not bound":
				t.Errorf("%s: the table says %s binds %q, which %s declares but nothing else under %s gates or decodes by",
					capabilityChecklistDoc, client.name, name, client.path, client.usedIn)
			case !declared && cell != "not bound":
				t.Errorf("%s: the table says %s binds %q but %s does not declare it", capabilityChecklistDoc, client.name, name, client.path)
			}
		}
	}
}

// capabilityDecodes names, for each capability that only populates a response and so has
// nothing to gate, the read of the generated message that decodes it and thereby binds it.
var capabilityDecodes = map[string]*regexp.Regexp{
	"CoordinateFrameValues":    regexp.MustCompile(`\*pb\.Value_CoordinateFrame\b`),
	"ConvertLibraryReferences": regexp.MustCompile(`\bresp\.LibraryReferences\b`),
	"DiagnosticCodes":          regexp.MustCompile(`\bdiag\.Code\b`),
	"EnumValues":               regexp.MustCompile(`\*pb\.Value_EnumLiteral\b`),
	"FeatureValues":            regexp.MustCompile(`\binst\.FeatureValues\b`),
	"SymbolAnnotatingMetadata": regexp.MustCompile(`\bsym\.AnnotatingMetadataIds\b`),
	"SymbolAttributes":         regexp.MustCompile(`\bsym\.Attributes\b`),
	"TypeFacts":                regexp.MustCompile(`\bsym\.TypeInfo\b`),
	"VerdictNotes":             regexp.MustCompile(`\bverdict\.Notes\b`),
	"UndeterminedValue":        regexp.MustCompile(`\*pb\.Value_Undetermined\b`),
	"UnsetValue":               regexp.MustCompile(`\*pb\.Value_Unset\b`),
}

// capabilityUsed reports whether sources gate by the capability's constant or decode what it populates.
func capabilityUsed(ident, sources string) bool {
	if regexp.MustCompile(`\bCapability` + ident + `\b`).MatchString(sources) {
		return true
	}
	decodes, ok := capabilityDecodes[ident]
	return ok && decodes.MatchString(sources)
}

// TestCapabilityDecodesAreRead keeps capabilityDecodes to reads the Go package still makes of
// capabilities the service still announces, so a removed decoder cannot leave a cell bound.
func TestCapabilityDecodesAreRead(t *testing.T) {
	sources := packageSourcesBeside(t, "client/opensysml", "client/opensysml/types.go")
	announced := map[string]bool{}
	for _, ident := range serviceCapabilityIdentifiers(t) {
		announced[ident] = true
	}
	for ident, decodes := range capabilityDecodes {
		if !announced[ident] {
			t.Errorf("capabilityDecodes names Capability%s, which the service does not declare", ident)
		}
		if !decodes.MatchString(sources) {
			t.Errorf("capabilityDecodes binds Capability%s by %s, which client/opensysml no longer reads", ident, decodes)
		}
	}
}

// packageSourcesBeside reads the Go sources of a package directory other than its tests and
// the capabilities module itself: where a declared capability has to be gated or decoded by.
func packageSourcesBeside(t *testing.T, dir, module string) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot, dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var sources strings.Builder
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == filepath.Join(repoRoot, module) {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		sources.Write(source)
	}
	if sources.Len() == 0 {
		t.Fatalf("%s holds no Go source beside %s", dir, module)
	}
	return sources.String()
}

// serviceCapabilityIdentifiers maps every capability name declared under
// internal/frontend/grpc to the suffix of its Go identifier.
func serviceCapabilityIdentifiers(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot, "internal/frontend/grpc/*.go"))
	if err != nil {
		t.Fatal(err)
	}
	identifiers := make(map[string]string)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range capabilityConstPattern.FindAllStringSubmatch(string(source), -1) {
			identifiers[match[2]] = match[1]
		}
	}
	return identifiers
}

// readCapabilityChecklist reads the first table under the Capabilities heading:
// the rows by capability name, with one cell per client column, and the columns.
func readCapabilityChecklist(t *testing.T) (map[string][]string, []string) {
	t.Helper()
	file, err := os.Open(filepath.Join(repoRoot, capabilityChecklistDoc))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows := make(map[string][]string)
	var columns []string
	inSection, inTable := false, false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "## ") {
			if inTable {
				break
			}
			inSection = line == capabilityChecklistHeading
			continue
		}
		if !inSection {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			if inTable {
				break
			}
			continue
		}
		cells := tableCells(line)
		switch {
		case columns == nil:
			if len(cells) < 2 || cells[0] != "Capability" {
				t.Fatalf("%s: the table header is %q, want a Capability column first", capabilityChecklistDoc, line)
			}
			columns = cells[1:]
			inTable = true
		case strings.HasPrefix(cells[0], "---"):
			continue
		default:
			name := strings.Trim(cells[0], "`")
			if len(cells) != len(columns)+1 {
				t.Fatalf("%s: row %s has %d cells, want %d", capabilityChecklistDoc, name, len(cells)-1, len(columns))
			}
			if _, dup := rows[name]; dup {
				t.Errorf("%s: %q is listed twice", capabilityChecklistDoc, name)
			}
			rows[name] = cells[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if columns == nil {
		t.Fatalf("%s: no table under %q", capabilityChecklistDoc, capabilityChecklistHeading)
	}
	return rows, columns
}

func tableCells(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

func sortedKeys(rows map[string][]string) []string {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
