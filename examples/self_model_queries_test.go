package examples

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/modeldoc"
)

// TestSelfModelQueryLibraryMatchesImplementation checks the modelled library
// against the library file and the compiler's bindings.
func TestSelfModelQueryLibraryMatchesImplementation(t *testing.T) {
	text := readModelFile(t, "queries.sysml")
	idx, ctx := analyseSelfModel(t)
	library := instantiateSelfModel(t, idx, ctx, "queries.sysml", "OpenSysMLQueries", "QueryLibrary")

	file, ok := declaredString(text, "libraryFile")
	if !ok {
		t.Fatal("queries.sysml names no library file")
	}
	libraryText, err := os.ReadFile(filepath.Join("..", file))
	if err != nil {
		t.Fatalf("read the library file: %v", err)
	}
	calcDefs := regexp.MustCompile(`(?m)^\s*(?:abstract )?calc def (\w+)`).FindAllStringSubmatch(string(libraryText), -1)
	names := make([]string, 0, len(calcDefs))
	for _, m := range calcDefs {
		names = append(names, m[1])
	}
	if got, want := library.integer("calcDefCount"), len(names); got != want {
		t.Errorf("queries.sysml says the library has %d calc defs, the file has %d", got, want)
	}
	if !strings.Contains(string(libraryText), "NON-NORMATIVE") || library.boolean("normative") {
		t.Error("queries.sysml must state the library non-normative, as the library file does")
	}
	pkg := regexp.MustCompile(`library package (\w+)`).FindStringSubmatch(string(libraryText))
	if pkg == nil || pkg[1] != library.str("libraryPackage") {
		t.Errorf("queries.sysml says libraryPackage = %q, the file declares %v", library.str("libraryPackage"), pkg)
	}

	compilerText, err := os.ReadFile(filepath.Join("..", "internal", "ir", "queryplan", "compiler.go"))
	if err != nil {
		t.Fatalf("read the compiler: %v", err)
	}
	bindings := map[string]string{}
	for _, m := range regexp.MustCompile(`"DocumentQueries::(\w+)":\s+\{Operation(\w+)`).FindAllStringSubmatch(string(compilerText), -1) {
		bindings[m[1]] = operationConstant(t, m[2])
	}
	if got, want := library.integer("operationCount"), len(bindings); got != want {
		t.Errorf("queries.sysml says the compiler binds %d calc defs, compiler.go binds %d", got, want)
	}
	base := regexp.MustCompile(`queryBaseFQN\s*=\s*"([^"]+)"`).FindStringSubmatch(string(compilerText))
	if base == nil || base[1] != library.str("queryBase") {
		t.Errorf("queries.sysml says queryBase = %q, compiler.go has %v", library.str("queryBase"), base)
	}

	modelled := map[string]string{}
	for _, part := range library.parts() {
		modelled[part.str("calcDef")] = part.str("operation")
	}
	if !reflect.DeepEqual(modelled, bindings) {
		t.Errorf("queries.sysml binds %v, compiler.go binds %v", modelled, bindings)
	}
	var unbound []string
	for _, name := range names {
		if _, bound := bindings[name]; !bound {
			unbound = append(unbound, name)
		}
	}
	if got, want := library.str("unboundCalcDefs"), strings.Join(unbound, ", "); got != want {
		t.Errorf("queries.sysml says unboundCalcDefs = %q, the library and compiler give %q", got, want)
	}
}

// operationConstant is the value of one queryplan.Operation constant, read from plan.go.
func operationConstant(t *testing.T, name string) string {
	t.Helper()
	constants := operationConstants(t)
	value, ok := constants[name]
	if !ok {
		t.Fatalf("plan.go declares no Operation%s", name)
	}
	return value
}

// operationConstants are the queryplan.Operation constants of plan.go by name.
func operationConstants(t *testing.T) map[string]string {
	t.Helper()
	planText, err := os.ReadFile(filepath.Join("..", "internal", "ir", "queryplan", "plan.go"))
	if err != nil {
		t.Fatalf("read plan.go: %v", err)
	}
	constants := map[string]string{}
	for _, m := range regexp.MustCompile(`Operation(\w+)\s+Operation = "([^"]+)"`).FindAllStringSubmatch(string(planText), -1) {
		constants[m[1]] = m[2]
	}
	return constants
}

// TestSelfModelQueryCompilerMatchesImplementation checks the compiler's figures
// against queryplan's operation constants.
func TestSelfModelQueryCompilerMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	compiler := instantiateSelfModel(t, idx, ctx, "queries.sysml", "OpenSysMLQueries", "QueryCompiler")
	library := instantiateSelfModel(t, idx, ctx, "queries.sysml", "OpenSysMLQueries", "QueryLibrary")

	if got, want := compiler.str("goPackage"), "internal/ir/queryplan"; got != want {
		t.Errorf("QueryCompiler.goPackage = %q, want %q", got, want)
	}
	if got, want := compiler.integer("operationCount"), len(operationConstants(t)); got != want {
		t.Errorf("queries.sysml says the plan has %d operations, plan.go declares %d", got, want)
	}
	if got, want := compiler.integer("libraryOperationCount"), library.integer("operationCount"); got != want {
		t.Errorf("the compiler binds %d library operations, the library says %d", got, want)
	}
	if !compiler.boolean("immutablePlans") {
		t.Error("queries.sysml must state the plans immutable")
	}
}

// TestSelfModelQueryOutcomesMatchImplementation checks the step kinds,
// constraints, outcome kinds and record fields against queryexec.
func TestSelfModelQueryOutcomesMatchImplementation(t *testing.T) {
	text := readModelFile(t, "queries.sysml")
	idx, ctx := analyseSelfModel(t)
	executor := instantiateSelfModel(t, idx, ctx, "queries.sysml", "OpenSysMLQueries", "QueryExecutor")
	if got, want := executor.str("goPackage"), "internal/doc/queryexec"; got != want {
		t.Errorf("QueryExecutor.goPackage = %q, want %q", got, want)
	}

	outcomeText, err := os.ReadFile(filepath.Join("..", "internal", "doc", "queryexec", "outcome.go"))
	if err != nil {
		t.Fatalf("read outcome.go: %v", err)
	}
	stepKinds := literalsOf(string(outcomeText), "StepKind")
	if got := enumMembers(text, "StepKind"); !reflect.DeepEqual(got, stepKinds) {
		t.Errorf("queries.sysml lists step kinds %v, outcome.go declares %v", got, stepKinds)
	}
	if got, want := executor.integer("stepKindCount"), len(stepKinds); got != want {
		t.Errorf("queries.sysml says %d step kinds, outcome.go declares %d", got, want)
	}
	constraints := literalsOf(string(outcomeText), "Constraint")
	if got := enumMembers(text, "ModelConstraint"); !reflect.DeepEqual(got, constraints) {
		t.Errorf("queries.sysml lists constraints %v, outcome.go declares %v", got, constraints)
	}
	if got, want := executor.integer("constraintCount"), len(constraints); got != want {
		t.Errorf("queries.sysml says %d constraints, outcome.go declares %d", got, want)
	}

	pathError := queryexec.Outcome{Path: &queryexec.PathError{}}
	modelError := queryexec.Outcome{Model: &queryexec.ModelError{}}
	both := queryexec.Outcome{Path: pathError.Path, Model: modelError.Model}
	if got := enumMembers(text, "OutcomeKind"); !reflect.DeepEqual(got, []string{"value", pathError.Kind(), modelError.Kind()}) {
		t.Errorf("queries.sysml lists outcome kinds %v, want value, %s, %s", got, pathError.Kind(), modelError.Kind())
	}
	// A model error outranks a path error from the same fan-out, as StepOutcome's
	// first transition states.
	if both.Kind() != modelError.Kind() {
		t.Errorf("an outcome with findings and failures is %q, the model says %q", both.Kind(), modelError.Kind())
	}
	if !strings.Contains(text, "transition first evaluating if violated then modelError;") {
		t.Error("StepOutcome must go to modelError whenever a constraint is violated")
	}

	records := map[string]reflect.Type{}
	collectRecordTypes(reflect.TypeOf(queryexec.Outcome{}), records)
	for name, typ := range records {
		var fields, typed []string
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			fields = append(fields, strings.ToLower(f.Name[:1])+f.Name[1:])
			typed = append(typed, attributeTyping(f.Type))
		}
		if got := attributeNames(text, name); !reflect.DeepEqual(got, fields) {
			t.Errorf("attribute def %s has %v, queryexec.%s has %v", name, got, name, fields)
		}
		if got := attributeTypings(text, name); !reflect.DeepEqual(got, typed) {
			t.Errorf("attribute def %s types its attributes %v, queryexec.%s has %v", name, got, name, typed)
		}
	}

	tolerating := regexp.MustCompile(`in tolerate : Boolean`).FindAllStringIndex(executorLibrary(t), -1)
	if got, want := strings.Count(executor.str("toleratingOperations"), ",")+1, len(tolerating); got != want {
		t.Errorf("queries.sysml names %d tolerating operations, the library declares tolerate on %d", got, want)
	}
	for _, name := range strings.Split(executor.str("toleratingOperations"), ", ") {
		if !regexp.MustCompile(`calc def ` + name + ` \{[^}]*in tolerate : Boolean`).MatchString(executorLibrary(t)) {
			t.Errorf("queries.sysml says %s tolerates, the library declares no tolerate on it", name)
		}
	}
}

// executorLibrary is the text of the DocumentQueries library file.
func executorLibrary(t *testing.T) string {
	t.Helper()
	file, ok := declaredString(readModelFile(t, "queries.sysml"), "libraryFile")
	if !ok {
		t.Fatal("queries.sysml names no library file")
	}
	content, err := os.ReadFile(filepath.Join("..", file))
	if err != nil {
		t.Fatalf("read the library file: %v", err)
	}
	return string(content)
}

// literalsOf are the string constants of the named Go type, in declaration order.
func literalsOf(text, typ string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`\w+\s+`+typ+` = "([^"]+)"`).FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// enumMembers are the members of an enum def of the model text, in order.
func enumMembers(text, name string) []string {
	body := regexp.MustCompile(`enum def ` + name + ` \{([^}]*)\}`).FindStringSubmatch(text)
	if body == nil {
		return nil
	}
	var out []string
	for _, m := range regexp.MustCompile(`(\w+);`).FindAllStringSubmatch(body[1], -1) {
		out = append(out, m[1])
	}
	return out
}

// collectRecordTypes gathers a struct type and every struct type its fields
// reach, by name, as the model's attribute defs mirror them.
func collectRecordTypes(typ reflect.Type, into map[string]reflect.Type) {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || into[typ.Name()] != nil {
		return
	}
	into[typ.Name()] = typ
	for i := 0; i < typ.NumField(); i++ {
		collectRecordTypes(typ.Field(i).Type, into)
	}
}

// attributeTyping is the model's typing of a Go field: the element type's name
// (String for a string kind) and a multiplicity when the field is a slice or pointer.
func attributeTyping(typ reflect.Type) string {
	switch typ.Kind() {
	case reflect.Slice:
		return attributeTyping(typ.Elem()) + "[0..*]"
	case reflect.Pointer:
		return attributeTyping(typ.Elem()) + "[0..1]"
	case reflect.String:
		if typ.Name() != "string" {
			return typ.Name()
		}
		return "String"
	}
	return typ.Name()
}

// attributeNames are the attributes of an attribute def of the model text, in order.
func attributeNames(text, name string) []string {
	var out []string
	for _, m := range attributeDeclarations(text, name) {
		out = append(out, m[1])
	}
	return out
}

// attributeTypings are the attributes' types and multiplicities, in order, as
// a Go field states them: a non-empty list is a slice, an optional scalar a string.
func attributeTypings(text, name string) []string {
	var out []string
	for _, m := range attributeDeclarations(text, name) {
		typing := m[2] + strings.ReplaceAll(m[3], "[1..*]", "[0..*]")
		if m[3] == "[0..1]" && m[2] == "String" {
			typing = "String"
		}
		out = append(out, modelTypeToGo(typing))
	}
	return out
}

// attributeDeclarations are the `attribute 'name' : Type[mult]` declarations
// of an attribute def, as name, type and multiplicity.
func attributeDeclarations(text, name string) [][]string {
	body := regexp.MustCompile(`attribute def ` + name + ` \{([^}]*)\}`).FindStringSubmatch(text)
	if body == nil {
		return nil
	}
	return regexp.MustCompile(`attribute '?(\w+)'? : (\w+)(\[[^\]]*\])?`).FindAllStringSubmatch(body[1], -1)
}

// modelTypeToGo renames the model's enum defs to the Go types they stand for.
func modelTypeToGo(typing string) string {
	return strings.NewReplacer("ModelConstraint", "Constraint").Replace(typing)
}

// TestSelfModelQuerySurfacesMatchImplementation checks the flag, the meta
// command, the RPC and its error detail against where each is defined.
func TestSelfModelQuerySurfacesMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	path := instantiateSelfModel(t, idx, ctx, "queries.sysml", "OpenSysMLQueries", "QueryPath")
	surfaces := path.parts()

	cli := surfaces["cli"]
	usage, err := os.ReadFile(filepath.Join("..", cli.str("goPackage"), "usage.go"))
	if err != nil {
		t.Fatalf("read the CLI usage: %v", err)
	}
	for _, flag := range []string{cli.str("command"), cli.str("inlineCommand")} {
		if !strings.Contains(string(usage), `"`+strings.TrimPrefix(flag, "-")+`"`) {
			t.Errorf("cmd/sysml defines no %s flag", flag)
		}
	}
	if cliText := readGoPackage(t, filepath.Join("..", cli.str("goPackage"))); !strings.Contains(cliText, "queryexec.") || !strings.Contains(cliText, "outcome") {
		t.Error("cmd/sysml does not report the query outcome")
	}
	if got, want := cli.integer("failureExitStatus"), 2; got != want {
		t.Errorf("queries.sysml says a failed query exits %d, the guide records %d", got, want)
	}
	guide, err := os.ReadFile(filepath.Join("..", surfaces["oracle"].str("guide")))
	if err != nil {
		t.Fatalf("read the guide: %v", err)
	}
	for _, want := range []string{cli.str("outcomeShown"), cli.str("jsonFlag"), "exit status 2", surfaces["repl"].str("command")} {
		if !strings.Contains(string(guide), want) {
			t.Errorf("the guide does not mention %q", want)
		}
	}

	repl := surfaces["repl"]
	meta, err := os.ReadFile(filepath.Join("..", repl.str("goPackage"), "meta.go"))
	if err != nil {
		t.Fatalf("read the REPL meta commands: %v", err)
	}
	if !regexp.MustCompile(`cmdRunQuery\s+= "` + regexp.QuoteMeta(repl.str("command")) + `"`).MatchString(string(meta)) {
		t.Errorf("the REPL defines no %s command", repl.str("command"))
	}
	if !regexp.MustCompile(`name: cmdRunQuery,[^\n]*args: "[^"]*\| ` + regexp.QuoteMeta(repl.str("inlineCommand")) + ` <expr>`).MatchString(string(meta)) {
		t.Errorf("the REPL's %s does not take %s <expr>", repl.str("command"), repl.str("inlineCommand"))
	}

	service := surfaces["service"]
	if got, want := service.str("goPackage"), "internal/frontend/grpc"; got != want {
		t.Errorf("ServiceQuerySurface.goPackage = %q, want %q", got, want)
	}
	services := pb.File_sysml_proto.Services()
	var rpc protoreflect.MethodDescriptor
	for i := 0; i < services.Len(); i++ {
		if m := services.Get(i).Methods().ByName(protoreflect.Name(service.str("command"))); m != nil {
			rpc = m
		}
	}
	if rpc == nil {
		t.Errorf("the schema declares no %s RPC", service.str("command"))
	}
	outcome := pb.File_sysml_proto.Messages().ByName(protoreflect.Name(service.str("outcomeShown")))
	if outcome == nil {
		t.Fatalf("the schema declares no %s message", service.str("outcomeShown"))
	}
	if rpc != nil && rpc.Input().Fields().ByName(protoreflect.Name(service.str("inlineCommand"))) == nil {
		t.Errorf("%s takes no %s", service.str("command"), service.str("inlineCommand"))
	}
	for _, capability := range []string{service.str("capability"), service.str("inlineCapability")} {
		if !slices.Contains(sysmlgrpc.Capabilities(), capability) {
			t.Errorf("the service declares no %s capability", capability)
		}
	}
	docquery, err := os.ReadFile(filepath.Join("..", service.str("goPackage"), "docquery.go"))
	if err != nil {
		t.Fatalf("read the service's query handler: %v", err)
	}
	for _, limit := range []string{"failureLimit", "frontierLimit"} {
		m := regexp.MustCompile(`const ` + limit + ` = (\d+)`).FindStringSubmatch(string(docquery))
		if m == nil {
			t.Errorf("docquery.go declares no %s", limit)
			continue
		}
		if got, want := strconv.Itoa(service.integer(limit)), m[1]; got != want {
			t.Errorf("queries.sysml says %s = %s, docquery.go says %s", limit, got, want)
		}
	}
	totals := 0
	for _, message := range []protoreflect.MessageDescriptor{outcome, pb.File_sysml_proto.Messages().ByName("QueryFrontier")} {
		if message == nil {
			continue
		}
		for i := 0; i < message.Fields().Len(); i++ {
			if strings.HasSuffix(string(message.Fields().Get(i).Name()), "_total") {
				totals++
			}
		}
	}
	if service.boolean("reportsTotals") != (totals > 0) {
		t.Errorf("queries.sysml says reportsTotals = %v, the schema has %d *_total fields", service.boolean("reportsTotals"), totals)
	}
	catalog, err := os.ReadFile(filepath.Join("..", "internal", "frontend", "mcp", "catalog", "catalog.go"))
	if err != nil {
		t.Fatalf("read the MCP tool catalog: %v", err)
	}
	if !regexp.MustCompile(`Name: "` + service.str("mcpTool") + `",[^\n]*RPCs: \[\]string\{"` + service.str("command") + `"\}`).Match(catalog) {
		t.Errorf("the MCP catalog has no %s tool over %s", service.str("mcpTool"), service.str("command"))
	}
	docgen, err := os.ReadFile(filepath.Join("..", "editors", "mdk", "plugin", "src", "main", "java", "org", "openmbee", "mdk", "magic", "docgen", "DocGenExtensions.java"))
	if err != nil {
		t.Fatalf("read the MDK docgen extensions: %v", err)
	}
	if !strings.Contains(string(docgen), `Operation.RUN_QUERY, "`+service.str("mdkOperation")+`"`) {
		t.Errorf("the MDK plugin registers no %s docgen operation", service.str("mdkOperation"))
	}
	if _, err := os.Stat(filepath.Join("..", service.str("schema"))); err != nil {
		t.Errorf("queries.sysml points at %s, which does not exist", service.str("schema"))
	}
	if !strings.Contains(string(docquery), "pb."+service.str("outcomeShown")) {
		t.Errorf("the service does not attach a %s detail", service.str("outcomeShown"))
	}

	if got, want := surfaces["tables"].str("goPackage"), "internal/ir/docplan"; got != want {
		t.Errorf("TableQuerySurface.goPackage = %q, want %q", got, want)
	}
	for name, surface := range surfaces {
		if pkg := surface.str("goPackage"); pkg != "" {
			if _, err := os.Stat(filepath.Join("..", pkg)); err != nil {
				t.Errorf("%s points at %s, which does not exist", name, pkg)
			}
		}
	}
}

// TestSelfModelQvtsOracleMatchesRecordings checks the oracle's examples,
// recordings, guide and design note exist as the model names them.
func TestSelfModelQvtsOracleMatchesRecordings(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	oracle := instantiateSelfModel(t, idx, ctx, "queries.sysml", "OpenSysMLQueries", "QvtsOracle")

	for _, attr := range []string{"goPackage", "examples", "baseline", "guide", "designNote"} {
		if _, err := os.Stat(filepath.Join("..", oracle.str(attr))); err != nil {
			t.Errorf("queries.sysml points at %s = %s, which does not exist", attr, oracle.str(attr))
		}
	}
	entries, err := os.ReadDir(filepath.Join("..", oracle.str("baseline")))
	if err != nil {
		t.Fatalf("read the recordings: %v", err)
	}
	if got, want := oracle.integer("recordingCount"), len(entries); got != want {
		t.Errorf("queries.sysml says %d recordings, %s holds %d", got, oracle.str("baseline"), want)
	}
	if !oracle.boolean("gating") {
		t.Error("the QVTS oracle gates the build: tests/qvts fails on a moved recording")
	}
	note, err := os.ReadFile(filepath.Join("..", oracle.str("designNote")))
	if err != nil {
		t.Fatalf("read the design note: %v", err)
	}
	if !strings.Contains(string(note), oracle.str("reference")) {
		t.Errorf("the design note does not name the reference %s", oracle.str("reference"))
	}
}

// TestSelfModelQueryInvariantHolds evaluates the query path's invariant like
// the twenty of the other packages.
func TestSelfModelQueryInvariantHolds(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	scope := packageScope(t, idx, "queries.sysml", "OpenSysMLQueries")
	sym, ok := scope.LookupLocal("outcomesAreDistinct")
	if !ok {
		t.Fatal("requirement outcomesAreDistinct is not declared in OpenSysMLQueries")
	}
	satisfied, err := ctx.EvaluateRequirement(sym, scope)
	if err != nil {
		t.Fatalf("evaluate outcomesAreDistinct: %v", err)
	}
	if !satisfied {
		t.Error("outcomesAreDistinct does not hold")
	}
}

// TestSelfModelQueriesSectionRenders renders the document and checks every
// table of the Queries and outcomes section has rows.
func TestSelfModelQueriesSectionRenders(t *testing.T) {
	ws := model.NewWorkspace()
	for _, name := range selfModelFiles(t) {
		content, err := os.ReadFile(filepath.Join(selfModelDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		ws.Open(name, content, 1)
	}
	markdown, err := modeldoc.RenderDocumentMarkdown(ws, "OpenSysMLDocument::ArchitectureDocument", docrender.MarkdownOptions{})
	if err != nil {
		t.Fatalf("render the architecture document: %v", err)
	}
	_, section, found := strings.Cut(markdown, "## Queries and outcomes")
	if !found {
		t.Fatal("the document has no Queries and outcomes section")
	}
	for _, want := range []string{
		"| OwnedElements | owned-elements |",
		"| Tree | tree |",
		"| Differ |",
		"| cli | cmd/sysml | -run-query | checks\\[\\].outcome |",
		"| repl | internal/frontend/repl | %run-query | outcome lines |",
		"| service | internal/frontend/grpc | RunDocumentQuery | QueryOutcome |",
		"| OpenSysMLArtifacts::CodeUnit |",
		"| cli | internal/frontend/repl | cmd/sysml |",
		"| satisfaction | OpenSysMLSurfaces::opensysml.pipeline.tree | holds |",
		"| satisfaction | OpenSysMLQueries::queryPath.executor | holds |",
		"OpenSysMLViews::queryPath",
		"OpenSysMLViews::stepOutcome",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the Queries and outcomes section does not contain %q", want)
		}
	}
	var captions []string
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "*") && strings.HasSuffix(line, "*") && !strings.HasPrefix(line, "**") {
			captions = append(captions, line)
		}
	}
	sort.Strings(captions)
	if len(captions) < 9 {
		t.Errorf("the section renders %d captions, want its two diagrams and seven tables", len(captions))
	}
	if strings.Contains(section, "| --- |\n\n") {
		t.Error("a table of the section rendered without rows")
	}
}
