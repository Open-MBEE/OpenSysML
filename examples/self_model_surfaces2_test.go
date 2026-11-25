// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package examples

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// The surfaces the first two self-model tests do not reach; each fact is read
// from the analysed model and compared with the code, build files or documents.

// readRepoFile returns the text of one file of the repository.
func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

// requireRepoPath fails unless the repository holds what a path-valued attribute names.
func requireRepoPath(t *testing.T, part, attribute, path string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join("..", path)); err != nil {
		t.Errorf("surfaces.sysml points %s.%s at %s, which does not exist", part, attribute, path)
	}
}

// goConstant returns the string a Go source assigns to a named constant or variable.
func goConstant(t *testing.T, source, name string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^\s*(?:const\s+|var\s+)?` + regexp.QuoteMeta(name) + `\s*(?::\s*\w+\s*)?=\s*"([^"]*)"`).FindStringSubmatch(source)
	if match == nil {
		t.Fatalf("the implementation assigns no string to %s", name)
	}
	return match[1]
}

// commaList splits a comma-separated attribute into its words.
func commaList(value string) []string {
	return strings.Split(value, ", ")
}

func TestSelfModelServiceAccessMatchesCommand(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	service := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "ModelService")
	entrypoint := service.str("entrypoint")
	command := readGoPackage(t, filepath.Join("..", entrypoint))

	requireFlag(t, "ModelService", "hostSetting", service.str("hostSetting"), entrypoint)
	for _, flag := range commaList(service.str("tokenSettings")) {
		requireFlag(t, "ModelService", "tokenSettings", flag, entrypoint)
	}
	requireFlag(t, "ModelService", "unauthenticatedSetting", service.str("unauthenticatedSetting"), entrypoint)
	requireFlag(t, "ModelService", "insecureTokenSetting", service.str("insecureTokenSetting"), entrypoint)

	if declared, actual := service.str("bearerTokenEnv"), goConstant(t, command, "tokenEnv"); declared != actual {
		t.Errorf("surfaces.sysml says ModelService.bearerTokenEnv = %q, %s reads %q", declared, entrypoint, actual)
	}
	loopback := strings.Contains(command, `host = "127.0.0.1"`) && strings.Contains(command, "empty binds loopback")
	if service.boolean("loopbackByDefault") != loopback {
		t.Errorf("surfaces.sysml says ModelService.loopbackByDefault = %v, %s binds loopback for an empty -host: %v",
			service.boolean("loopbackByDefault"), entrypoint, loopback)
	}

	access := service.str("accessControl")
	requireRepoPath(t, "ModelService", "accessControl", access)
	if filepath.Dir(access) != entrypoint {
		t.Errorf("surfaces.sysml puts ModelService.accessControl in %s, outside %s", filepath.Dir(access), entrypoint)
	}
	accessSource := readRepoFile(t, access)
	logsEveryCall := strings.Contains(accessSource, `slog.Info("call",`) && strings.Contains(accessSource, "func logCall(") &&
		strings.Contains(accessSource, "func (i connectAccess) WrapUnary(") && strings.Contains(accessSource, "func (i connectAccess) WrapStreamingHandler(")
	if service.boolean("logsEveryCall") != logsEveryCall {
		t.Errorf("surfaces.sysml says ModelService.logsEveryCall = %v, %s logs unary and streaming calls: %v",
			service.boolean("logsEveryCall"), access, logsEveryCall)
	}
}

func TestSelfModelLanguageServerNotebookAndSettings(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	server := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "LanguageServer")
	lspDir := filepath.Join("..", server.str("goPackage"))
	entrypoint := server.str("entrypoint")

	result, err := lsp.NewServer(model.NewWorkspace()).Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("initialize the language server: %v", err)
	}
	if advertised := result.Capabilities.NotebookDocumentSync != nil; server.boolean("notebookCells") != advertised {
		t.Errorf("surfaces.sysml says LanguageServer.notebookCells = %v, the server advertises notebook synchronization: %v",
			server.boolean("notebookCells"), advertised)
	}

	notebook := readRepoFile(t, filepath.Join(server.str("goPackage"), "notebook.go"))
	match := regexp.MustCompile(`cellLanguages = \[\]string\{([^}]*)\}`).FindStringSubmatch(notebook)
	if match == nil {
		t.Fatal("notebook.go declares no cellLanguages")
	}
	var languages []string
	for _, quoted := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(match[1], -1) {
		languages = append(languages, quoted[1])
	}
	if declared, actual := server.str("notebookCellLanguages"), strings.Join(languages, ", "); declared != actual {
		t.Errorf("surfaces.sysml says LanguageServer.notebookCellLanguages = %q, the server syncs %q", declared, actual)
	}

	settings := readRepoFile(t, filepath.Join(server.str("goPackage"), "conformance.go"))
	for attribute, constant := range map[string]string{
		"settingsSection":       "settingsSection",
		"keepLineBreaksSetting": "keepLineBreaksKey",
		"libraryRootsSetting":   "libraryRootsKey",
	} {
		if declared, actual := server.str(attribute), goConstant(t, settings, constant); declared != actual {
			t.Errorf("surfaces.sysml says LanguageServer.%s = %q, the server reads %q", attribute, declared, actual)
		}
	}

	requireFlag(t, "LanguageServer", "keepLineBreaksFlag", server.str("keepLineBreaksFlag"), entrypoint)
	requireFlag(t, "LanguageServer", "strictFlag", server.str("strictFlag"), entrypoint)
	if !strings.Contains(readGoPackage(t, filepath.Join("..", entrypoint)), "lsp.WithKeepLineBreaks(") {
		t.Errorf("%s does not hand its %s flag to the server", entrypoint, server.str("keepLineBreaksFlag"))
	}
	if declared := server.str("importDiagramMethod"); declared != lsp.MethodImportDiagram {
		t.Errorf("surfaces.sysml says LanguageServer.importDiagramMethod = %q, the server dispatches %q", declared, lsp.MethodImportDiagram)
	}
	if !strings.Contains(readGoPackage(t, lspDir), "req.Method() != MethodImportDiagram") {
		t.Errorf("%s dispatches no handler for %s", server.str("goPackage"), lsp.MethodImportDiagram)
	}
}

func TestSelfModelRenderingFormsMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	forms := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "RenderingForms")

	type modelled struct{ diagram, runOnly, editable bool }
	declared := map[string]modelled{}
	for _, part := range forms.parts() {
		if !part.has("form") {
			continue
		}
		form := part.str("form")
		if _, dup := declared[form]; dup {
			t.Errorf("RenderingForms declares form %q twice", form)
		}
		declared[form] = modelled{part.boolean("diagram"), part.boolean("runOnly"), part.boolean("editable")}
	}

	diagramForms := map[view.Form]bool{}
	for _, form := range view.DiagramForms() {
		diagramForms[form] = true
	}
	viewForms := map[view.Form]bool{}
	for _, form := range view.ViewForms() {
		viewForms[form] = true
	}
	for _, form := range view.Forms() {
		got, ok := declared[string(form)]
		if !ok {
			t.Errorf("the view package writes %q, which RenderingForms does not model", form)
			continue
		}
		if got.diagram != diagramForms[form] {
			t.Errorf("RenderingForms says %q diagram = %v, the implementation says %v", form, got.diagram, diagramForms[form])
		}
		if got.runOnly == viewForms[form] {
			t.Errorf("RenderingForms says %q runOnly = %v, the implementation renders views in it: %v", form, got.runOnly, viewForms[form])
		}
		if got.editable != (form == view.FormDrawio) {
			t.Errorf("RenderingForms says %q editable = %v, only %q is read back", form, got.editable, view.FormDrawio)
		}
	}
	if len(declared) != len(view.Forms()) {
		t.Errorf("RenderingForms models %d forms, the view package writes %d", len(declared), len(view.Forms()))
	}
	if declared := forms.integer("formCount"); declared != len(view.Forms()) {
		t.Errorf("surfaces.sysml says formCount = %d, the implementation has %d", declared, len(view.Forms()))
	}
	if declared := forms.integer("viewFormCount"); declared != len(view.ViewForms()) {
		t.Errorf("surfaces.sysml says viewFormCount = %d, the implementation has %d", declared, len(view.ViewForms()))
	}

	// The editable form is read back on every surface: the CLI's verb, the
	// language server's request and the MCP tool over it.
	cli := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "CommandLine")
	requireFlag(t, "CommandLine", "importDiagramSetting", cli.str("importDiagramSetting"), cli.str("entrypoint"))
	if !contains(commaList(cli.str("verbs")), cli.str("importDiagramSetting")) {
		t.Errorf("CommandLine.verbs omits %s", cli.str("importDiagramSetting"))
	}
	server := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "McpServer")
	var tool *catalog.Entry
	for _, entry := range catalog.Entries(catalog.GroupLSP) {
		if entry.Name == server.str("importDiagramTool") {
			tool = &entry
			break
		}
	}
	if tool == nil {
		t.Fatalf("surfaces.sysml says McpServer.importDiagramTool = %q, which the lsp tool group does not serve", server.str("importDiagramTool"))
	}
	if !contains(tool.Flags, strings.TrimPrefix(cli.str("importDiagramSetting"), "-")) {
		t.Errorf("the %s tool carries flags %v, not the CLI's %s", tool.Name, tool.Flags, cli.str("importDiagramSetting"))
	}
	if !strings.Contains(tool.Summary, string(view.FormDrawio)) {
		t.Errorf("the %s tool's summary does not name the %q form it reads back", tool.Name, view.FormDrawio)
	}
}

func TestSelfModelCommandLineVerbsMatchFlags(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	cli := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "CommandLine")
	usage := readGoPackage(t, filepath.Join("..", cli.str("entrypoint")))

	verbs := commaList(cli.str("verbs"))
	seen := map[string]bool{}
	for _, verb := range verbs {
		if seen[verb] {
			t.Errorf("CommandLine.verbs names %s twice", verb)
		}
		seen[verb] = true
		requireFlag(t, "CommandLine", "verbs", verb, cli.str("entrypoint"))
	}
	// A verb is documented: the usage lists it among the options of a group.
	for _, verb := range verbs {
		if !strings.Contains(usage, `usage.Opt("`+strings.TrimPrefix(verb, "-")+`"`) {
			t.Errorf("CommandLine.verbs names %s, which %s's usage does not document", verb, cli.str("entrypoint"))
		}
	}
	for _, required := range []string{"-metamodel-table", "-run-query", "-import-diagram", "-convert", "-migrate", "-render-document"} {
		if !seen[required] {
			t.Errorf("CommandLine.verbs omits %s", required)
		}
	}
}

func TestSelfModelProjectDiscoveryMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	projects := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "ProjectDiscovery")

	if declared := projects.str("projectFile"); declared != project.ProjectFile {
		t.Errorf("surfaces.sysml says ProjectDiscovery.projectFile = %q, the implementation reads %q", declared, project.ProjectFile)
	}
	if declared := projects.str("libraryRootsEnv"); declared != project.LibraryRootsEnvVar {
		t.Errorf("surfaces.sysml says ProjectDiscovery.libraryRootsEnv = %q, the implementation reads %q", declared, project.LibraryRootsEnvVar)
	}

	source := readGoPackage(t, filepath.Join("..", projects.str("goPackage")))
	match := regexp.MustCompile(`wellKnownLibraryDirs = \[\]string\{\n((?:.*\n)*?)\s*\}`).FindStringSubmatch(source)
	if match == nil {
		t.Fatal("the project package declares no wellKnownLibraryDirs")
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(match[1]), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		var segments []string
		for _, quoted := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(line, -1) {
			segments = append(segments, quoted[1])
		}
		if len(segments) > 0 {
			dirs = append(dirs, strings.Join(segments, "/"))
		}
	}
	if declared, actual := projects.str("wellKnownLibraryDirs"), strings.Join(dirs, ", "); declared != actual {
		t.Errorf("surfaces.sysml says ProjectDiscovery.wellKnownLibraryDirs = %q, the implementation knows %q", declared, actual)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, project.ProjectFile), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		if !project.IsLibraryDir(root, strings.SplitN(dir, "/", 2)[0]) {
			t.Errorf("the implementation does not recognize %s beside %s as a library directory", dir, project.ProjectFile)
		}
	}
}

func TestSelfModelClientsCarryTheServiceToken(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	toolchain := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "Toolchain")
	service := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "ModelService")

	clients := 0
	for name, part := range toolchain.parts() {
		if !part.has("connectionCode") {
			continue
		}
		clients++
		code := part.str("connectionCode")
		requireRepoPath(t, name, "connectionCode", code)
		if !strings.HasPrefix(code, part.str("goPackage")+"/") {
			t.Errorf("%s.connectionCode %s lies outside %s", name, code, part.str("goPackage"))
			continue
		}
		source := readRepoFile(t, code)
		if env := part.str("bearerTokenEnv"); env != service.str("bearerTokenEnv") || !strings.Contains(source, `"`+env+`"`) && !strings.Contains(source, `'`+env+`'`) {
			t.Errorf("surfaces.sysml says %s reads the token from %s; the service reads %s and %s mentions it: %v",
				name, env, service.str("bearerTokenEnv"), code, strings.Contains(source, env))
		}
		option := part.str("tokenOption")
		if !regexp.MustCompile(`\b` + regexp.QuoteMeta(option) + `\b`).MatchString(source) {
			t.Errorf("surfaces.sysml says %s takes the token as %q, which %s does not define", name, option, code)
		}
	}
	if clients != 7 {
		t.Errorf("the toolchain models %d clients with connection code, not the seven", clients)
	}
}

func TestSelfModelEditorsBeyondVsCode(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	makefile := readMakefile(t)

	emacs := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "EmacsMode")
	dir := emacs.str("goPackage")
	for _, file := range commaList(emacs.str("lispFiles")) {
		requireRepoPath(t, "EmacsMode", "lispFiles", filepath.Join(dir, file))
	}
	mode := readRepoFile(t, filepath.Join(dir, "sysml-mode.el"))
	for _, language := range commaList(emacs.str("languages")) {
		if !strings.Contains(mode, "(define-derived-mode "+language+"-mode ") {
			t.Errorf("surfaces.sysml says EmacsMode serves %s, sysml-mode.el defines no %s-mode", language, language)
		}
	}
	client := readRepoFile(t, filepath.Join(dir, "sysml-lsp.el"))
	if !strings.Contains(client, "("+emacs.str("lspClient")+" ") || !strings.Contains(client, "Package-Requires:") {
		t.Errorf("surfaces.sysml says EmacsMode.lspClient = %q, which sysml-lsp.el does not require", emacs.str("lspClient"))
	}
	if !strings.Contains(client, `"`+emacs.str("speaksTo")+`"`) {
		t.Errorf("surfaces.sysml says EmacsMode speaks to %s, which sysml-lsp.el does not start", emacs.str("speaksTo"))
	}
	requireRepoPath(t, "EmacsMode", "keywordTable", emacs.str("keywordTable"))
	requireRepoPath(t, "EmacsMode", "keywordGenerator", emacs.str("keywordGenerator"))
	for _, attribute := range []string{"keywordTarget", "compileTarget", "testTarget"} {
		requireMakeTarget(t, makefile, "EmacsMode", attribute, emacs.str(attribute))
	}
	emacsDir := makeVariableWords(t, makefile, "EMACS_DIR")
	if len(emacsDir) != 1 || emacsDir[0] != dir {
		t.Errorf("the Makefile's EMACS_DIR is %v, surfaces.sysml puts EmacsMode at %s", emacsDir, dir)
	}
	if generator := strings.TrimPrefix(emacs.str("keywordGenerator"), dir+"/"); !strings.Contains(makefile, "go run ./$(EMACS_DIR)/"+generator+" ") {
		t.Errorf("the Makefile's %s target does not run %s", emacs.str("keywordTarget"), emacs.str("keywordGenerator"))
	}

	treeSitter := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "TreeSitterGrammars")
	requireRepoPath(t, "TreeSitterGrammars", "guide", treeSitter.str("guide"))
	guide := readRepoFile(t, treeSitter.str("guide"))
	if !strings.Contains(guide, "("+treeSitter.str("repository")+")") {
		t.Errorf("%s does not link %s", treeSitter.str("guide"), treeSitter.str("repository"))
	}
	heading := ""
	for _, line := range strings.Split(guide, "\n") {
		if strings.HasPrefix(line, "## ") && strings.Contains(line, "Helix") {
			heading = line
		}
	}
	for _, editor := range commaList(treeSitter.str("editors")) {
		if !strings.Contains(heading, editor) {
			t.Errorf("surfaces.sysml lists %s under TreeSitterGrammars, the guide's section %q does not", editor, heading)
		}
	}
	if !strings.Contains(guide, treeSitter.str("speaksTo")) {
		t.Errorf("%s does not wire the Tree-sitter editors to %s", treeSitter.str("guide"), treeSitter.str("speaksTo"))
	}

	readme := readRepoFile(t, filepath.Join("editors", "README.md"))
	for _, def := range []string{"MagicMDK", "SysonMDK"} {
		plugin := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", def)
		pom := readRepoFile(t, filepath.Join(plugin.str("goPackage"), "pom.xml"))
		for attribute, tag := range map[string]string{"groupId": "groupId", "artifactId": "artifactId", "pluginName": "name"} {
			match := regexp.MustCompile(`<` + tag + `>([^<]+)</` + tag + `>`).FindStringSubmatch(pom)
			if match == nil {
				t.Errorf("%s/pom.xml declares no <%s>", plugin.str("goPackage"), tag)
				continue
			}
			if declared := plugin.str(attribute); declared != match[1] {
				t.Errorf("surfaces.sysml says %s.%s = %q, the pom says %q", def, attribute, declared, match[1])
			}
		}
		if !strings.Contains(readme, plugin.str("pluginName")) {
			t.Errorf("editors/README.md does not describe %s", plugin.str("pluginName"))
		}
		if !plugin.has("javaPackage") {
			continue
		}
		packageDir := filepath.Join(strings.Split(plugin.str("javaPackage"), ".")...)
		found := false
		_ = filepath.WalkDir(filepath.Join("..", plugin.str("goPackage")), func(path string, entry os.DirEntry, err error) error {
			if err == nil && entry.IsDir() && strings.HasSuffix(path, string(filepath.Separator)+packageDir) {
				found = true
			}
			return nil
		})
		if !found {
			t.Errorf("surfaces.sysml says %s.javaPackage = %q, no such package under %s", def, plugin.str("javaPackage"), plugin.str("goPackage"))
		}
	}
}

func TestSelfModelMcpNpmPackageMatchesPackaging(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	pkg := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "McpNpmPackage")
	dir := pkg.str("goPackage")

	var manifest struct {
		Name string `json:"name"`
	}
	readJSON(t, filepath.Join("..", dir, "package.json"), &manifest)
	if declared := pkg.str("packageName"); declared != manifest.Name {
		t.Errorf("surfaces.sysml says McpNpmPackage.packageName = %q, package.json says %q", declared, manifest.Name)
	}
	readme := readRepoFile(t, filepath.Join(dir, "README.md"))
	if alias := pkg.str("npmAlias"); !strings.Contains(readme, "`"+alias+"`") || alias == manifest.Name {
		t.Errorf("surfaces.sysml says McpNpmPackage.npmAlias = %q, which %s/README.md does not document as a second name", alias, dir)
	}
	requireMakeTarget(t, readMakefile(t), "McpNpmPackage", "coverageTarget", pkg.str("coverageTarget"))

	toolchain := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "Toolchain")
	for _, part := range []string{"emacs", "treeSitter", "mcpPackage", "projects", "forms", "apiJson", "flexo", "transformation", "formal", "notation", "metaclassMap", "specExamples", "structure"} {
		if _, ok := toolchain.parts()[part]; !ok {
			t.Errorf("the Toolchain has no part %s", part)
		}
	}
}

func TestSelfModelGainedOraclesMatchTools(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	oracle := func(def string) *modelInstance {
		return instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", def)
	}
	readTool := func(dir string) string { return readGoPackage(t, filepath.Join("..", dir)) }

	for _, def := range []string{"ApiJsonDifferential", "FlexoInterop", "TransformationCensus", "FormalConstraintCensus", "NotationCensus", "MetaclassMap", "SpecExamplesGate", "StructureDifferential", "DocCounts"} {
		part := oracle(def)
		for _, attribute := range []string{"baseline", "report", "extractor", "fetchScript", "combiner"} {
			if part.has(attribute) {
				requireRepoPath(t, def, attribute, part.str(attribute))
			}
		}
	}

	apiJSON := oracle("ApiJsonDifferential")
	if source := readTool(apiJSON.str("goPackage")); !strings.Contains(source, `"`+apiJSON.str("baseline")+`"`) {
		t.Errorf("%s does not record its baseline at %s", apiJSON.str("goPackage"), apiJSON.str("baseline"))
	}
	if source := readTool("tools/cmd/pilot-apijson-diff"); !strings.Contains(source, apiJSON.str("report")) {
		t.Errorf("tools/cmd/pilot-apijson-diff does not write %s", apiJSON.str("report"))
	}

	flexo := oracle("FlexoInterop")
	if declared, actual := flexo.str("gateEnv"), goConstant(t, readTool(flexo.str("goPackage")), "EnvGate"); declared != actual {
		t.Errorf("surfaces.sysml says FlexoInterop.gateEnv = %q, the referee reads %q", declared, actual)
	}
	if tests := readGoTests(t, filepath.Join("..", flexo.str("goPackage"))); !strings.Contains(tests, `"`+strings.TrimPrefix(flexo.str("baseline"), flexo.str("goPackage")+"/")+`"`) {
		t.Errorf("%s's tests do not hold %s", flexo.str("goPackage"), flexo.str("baseline"))
	}

	for _, def := range []string{"TransformationCensus", "NotationCensus"} {
		census := oracle(def)
		source := readTool(census.str("goPackage"))
		if declared, actual := census.str("baseline"), goConstant(t, source, "baselinePath"); declared != actual {
			t.Errorf("surfaces.sysml says %s.baseline = %q, the census records %q", def, declared, actual)
		}
		if declared, actual := census.str("report"), goConstant(t, source, "censusDocPath"); declared != actual {
			t.Errorf("surfaces.sysml says %s.report = %q, the census checks %q", def, declared, actual)
		}
	}

	formal := oracle("FormalConstraintCensus")
	manifest := readTool("tools/census/manifest")
	if declared, actual := formal.str("baseline"), goConstant(t, manifest, "Path"); declared != actual {
		t.Errorf("surfaces.sysml says FormalConstraintCensus.baseline = %q, the manifest lives at %q", declared, actual)
	}
	if !packageImports(t, filepath.Join("..", formal.str("goPackage")), `"github.com/Open-MBEE/OpenSysML/tools/census/manifest"`) {
		t.Errorf("%s does not write through the manifest package", formal.str("goPackage"))
	}

	metaclassMap := oracle("MetaclassMap")
	requireRepoPath(t, "MetaclassMap", "command", metaclassMap.str("command"))
	if report := readRepoFile(t, metaclassMap.str("report")); !strings.Contains(report, "("+filepath.Base(metaclassMap.str("baseline"))+")") ||
		!strings.Contains(report, "./cmd/"+filepath.Base(metaclassMap.str("command"))) {
		t.Errorf("%s does not name its baseline %s and the command %s", metaclassMap.str("report"), metaclassMap.str("baseline"), metaclassMap.str("command"))
	}
	if !packageImports(t, filepath.Join("..", metaclassMap.str("command")), `"github.com/Open-MBEE/OpenSysML/`+metaclassMap.str("goPackage")+`"`) {
		t.Errorf("%s does not run %s", metaclassMap.str("command"), metaclassMap.str("goPackage"))
	}
	if !strings.Contains(readTool(metaclassMap.str("goPackage")), "func writeBaseline(") {
		t.Errorf("%s writes no baseline", metaclassMap.str("goPackage"))
	}

	specExamples := oracle("SpecExamplesGate")
	gate := readGoTests(t, filepath.Join("..", "tests", "corpus"))
	for attribute, want := range map[string]string{
		"baseline":    `"` + strings.TrimPrefix(specExamples.str("baseline"), "tests/corpus/") + `"`,
		"requireEnv":  `"` + specExamples.str("requireEnv") + `"`,
		"fetchScript": "./" + specExamples.str("fetchScript"),
	} {
		if !strings.Contains(gate, want) {
			t.Errorf("surfaces.sysml says SpecExamplesGate.%s = %q, which the corpus gate does not name", attribute, specExamples.str(attribute))
		}
	}
	if !packageImports(t, filepath.Join("..", specExamples.str("extractor")), `"github.com/Open-MBEE/OpenSysML/`+specExamples.str("goPackage")+`"`) {
		t.Errorf("%s does not run %s", specExamples.str("extractor"), specExamples.str("goPackage"))
	}

	structure := oracle("StructureDifferential")
	exportTests := readGoTests(t, filepath.Join("..", structure.str("goPackage")))
	if !strings.Contains(exportTests, "func "+structure.str("testName")+"(t *testing.T)") {
		t.Errorf("%s defines no %s", structure.str("goPackage"), structure.str("testName"))
	}
	if !strings.Contains(exportTests, `"`+strings.TrimPrefix(structure.str("baseline"), structure.str("goPackage")+"/")+`"`) {
		t.Errorf("%s's tests do not hold %s", structure.str("goPackage"), structure.str("baseline"))
	}

	counts := oracle("DocCounts")
	countsSource := readTool(counts.str("goPackage"))
	if declared, actual := counts.str("combinedPage"), goConstant(t, countsSource, "SpecCompliancePath"); declared != actual {
		t.Errorf("surfaces.sysml says DocCounts.combinedPage = %q, the census serves %q", declared, actual)
	}
	if !strings.Contains(countsSource, counts.str("combiner")) {
		t.Errorf("%s does not name %s as the combiner", counts.str("goPackage"), counts.str("combiner"))
	}
	ignored := strings.Contains(readRepoFile(t, ".gitignore"), "\n/"+counts.str("combinedPage")+"\n")
	if counts.boolean("combinedPageTracked") == ignored {
		t.Errorf("surfaces.sysml says DocCounts.combinedPageTracked = %v, .gitignore lists %s: %v", counts.boolean("combinedPageTracked"), counts.str("combinedPage"), ignored)
	}
	if info, err := os.Stat(filepath.Join("..", counts.str("baseline"))); err != nil || !info.IsDir() {
		t.Errorf("surfaces.sysml says DocCounts.baseline = %q, which is not the sources directory", counts.str("baseline"))
	}

	snapshotGate := oracle("StdlibSnapshotGate")
	skips := strings.Contains(readGoTests(t, filepath.Join("..", snapshotGate.str("goPackage"))), `t.Skip("no snapshot embedded`)
	if snapshotGate.boolean("skipsWithoutSnapshot") != skips {
		t.Errorf("surfaces.sysml says StdlibSnapshotGate.skipsWithoutSnapshot = %v, the gate skips an absent snapshot: %v", snapshotGate.boolean("skipsWithoutSnapshot"), skips)
	}
}
