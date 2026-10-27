// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package examples

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan/markup"
)

// The surfaces the first self-model test does not reach: the MCP server, the
// Jupyter kernel, the JSON-RPC commands a WebAssembly host runs, the notebook
// packages, the VS Code extension's contributions, the markup parsers behind a
// prose body, the CLI's report formats and the FMI gate. Each fact is read from
// the analysed model and compared with the code or the build files.

// flagDefinitionPattern matches the definition of one flag in a command's
// usage, whether the variable it binds is a field or a local.
func flagDefinitionPattern(flag string) *regexp.Regexp {
	return regexp.MustCompile(`\w+\.\w*Var\(&[\w.]+, "` + regexp.QuoteMeta(strings.TrimPrefix(flag, "-")) + `"`)
}

// requireFlag fails unless the command at entrypoint defines the flag the
// model attributes to it.
func requireFlag(t *testing.T, part, attribute, flag, entrypoint string) {
	t.Helper()
	cmdSource := readGoPackage(t, filepath.Join("..", entrypoint))
	if !strings.HasPrefix(flag, "-") || !flagDefinitionPattern(flag).MatchString(cmdSource) {
		t.Errorf("surfaces.sysml says %s.%s = %q, %s defines no such flag", part, attribute, flag, entrypoint)
	}
}

// requireMakeTarget fails unless the Makefile defines the target the model
// attributes to a part.
func requireMakeTarget(t *testing.T, makefile, part, attribute, target string) {
	t.Helper()
	for _, match := range makeTargetPattern.FindAllStringSubmatch(makefile, -1) {
		if match[1] == target {
			return
		}
	}
	t.Errorf("surfaces.sysml says %s.%s = %q, which the Makefile does not define", part, attribute, target)
}

// readMakefile returns the text of the repository's Makefile.
func readMakefile(t *testing.T) string {
	t.Helper()
	makefile, err := os.ReadFile(filepath.Join("..", "Makefile"))
	if err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	return string(makefile)
}

// makeVariableWords returns the words a Makefile variable is assigned, with a
// $(COMMANDS) reference expanded.
func makeVariableWords(t *testing.T, makefile, variable string) []string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^` + variable + ` := (.*)$`).FindStringSubmatch(makefile)
	if match == nil {
		t.Fatalf("the Makefile does not assign %s", variable)
	}
	var words []string
	for _, word := range strings.Fields(match[1]) {
		if word == "$(COMMANDS)" {
			words = append(words, makeVariableWords(t, makefile, "COMMANDS")...)
			continue
		}
		words = append(words, word)
	}
	return words
}

// TestSelfModelCommandsMatchMakefile compares the toolchain's command counts
// and every surface's entrypoint with the Makefile: the commands it releases,
// the ones it builds for WebAssembly, and which binary each surface is.
func TestSelfModelCommandsMatchMakefile(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	toolchain := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "Toolchain")
	makefile := readMakefile(t)

	released := makeVariableWords(t, makefile, "COMMANDS")
	wasm := makeVariableWords(t, makefile, "WASM_COMMANDS")
	if declared := toolchain.integer("releasedCommandCount"); declared != len(released) {
		t.Errorf("surfaces.sysml says releasedCommandCount = %d, the Makefile releases %d: %v", declared, len(released), released)
	}
	if declared := toolchain.integer("wasmCommandCount"); declared != len(wasm) {
		t.Errorf("surfaces.sysml says wasmCommandCount = %d, the Makefile builds %d for WebAssembly: %v", declared, len(wasm), wasm)
	}

	entrypoints := map[string]bool{}
	for name, part := range toolchain.parts() {
		if !part.has("entrypoint") {
			continue
		}
		entrypoint := part.str("entrypoint")
		if _, err := os.Stat(filepath.Join("..", entrypoint)); err != nil {
			t.Errorf("surfaces.sysml says %s is served by %s, which does not exist", name, entrypoint)
		}
		entrypoints[filepath.Base(entrypoint)] = true
		if !part.has("released") {
			continue
		}
		command := filepath.Base(entrypoint)
		if part.boolean("released") != contains(released, command) {
			t.Errorf("surfaces.sysml says %s.released = %t, COMMANDS lists %v", name, part.boolean("released"), released)
		}
		if part.boolean("buildsForWasm") != contains(wasm, command) {
			t.Errorf("surfaces.sysml says %s.buildsForWasm = %t, WASM_COMMANDS lists %v", name, part.boolean("buildsForWasm"), wasm)
		}
		if part.has("releaseAsset") && !strings.Contains(makefile, "/release/"+part.str("releaseAsset")+" ./"+entrypoint) {
			t.Errorf("surfaces.sysml says a release ships %s as %s, the Makefile does not build it", name, part.str("releaseAsset"))
		}
	}
	for _, command := range wasm {
		if !entrypoints[command] {
			t.Errorf("the Makefile builds %s, which no surface in surfaces.sysml is served by", command)
		}
	}
}

// TestSelfModelMcpServerMatchesImplementation compares the modelled MCP server
// with the package and its command: the tool groups the catalog defines, the
// HTTP path and token variable, the flags, the conformance target, and the
// protocols the conformance suite knows.
func TestSelfModelMcpServerMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	server := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "McpServer")
	makefile := readMakefile(t)

	var groups []string
	for _, group := range catalog.Groups() {
		groups = append(groups, string(group))
	}
	if declared, actual := server.str("toolGroups"), strings.Join(groups, ", "); declared != actual {
		t.Errorf("surfaces.sysml says McpServer.toolGroups = %q, the catalog has %q", declared, actual)
	}
	if declared := server.str("httpPath"); declared != mcp.HTTPPath {
		t.Errorf("surfaces.sysml says McpServer.httpPath = %q, the implementation serves %q", declared, mcp.HTTPPath)
	}
	if declared := server.str("tokenEnv"); declared != mcp.TokenEnv {
		t.Errorf("surfaces.sysml says McpServer.tokenEnv = %q, the implementation reads %q", declared, mcp.TokenEnv)
	}
	for _, attribute := range []string{"groupSelector", "httpSetting"} {
		requireFlag(t, "McpServer", attribute, server.str(attribute), server.str("entrypoint"))
	}
	requireMakeTarget(t, makefile, "McpServer", "conformanceTarget", server.str("conformanceTarget"))

	suite := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "ConformanceSuite")
	conformance := readGoPackage(t, filepath.Join("..", suite.str("goPackage")))
	known := regexp.MustCompile(`knownProtocols = map\[string\]struct\{\}\{(.*)\}`).FindStringSubmatch(conformance)
	if known == nil {
		t.Fatalf("%s declares no knownProtocols", suite.str("goPackage"))
	}
	var protocols []string
	for _, match := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(known[1], -1) {
		protocols = append(protocols, match[1])
	}
	if declared, actual := suite.str("protocols"), strings.Join(protocols, ", "); declared != actual {
		t.Errorf("surfaces.sysml says ConformanceSuite.protocols = %q, the suite knows %q", declared, actual)
	}
}

// TestSelfModelJupyterKernelMatchesImplementation compares the modelled kernel
// with its command and package: the kernelspec names, the flags, the MIME
// forms it displays, and that it wraps the REPL rather than the service.
func TestSelfModelJupyterKernelMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	kernel := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "JupyterKernel")

	cmdSource := readGoPackage(t, filepath.Join("..", kernel.str("entrypoint")))
	for attribute, constant := range map[string]string{"kernelName": "defaultKernelName", "displayName": "defaultDisplayName"} {
		match := regexp.MustCompile(constant + `\s*=\s*"([^"]+)"`).FindStringSubmatch(cmdSource)
		if match == nil {
			t.Fatalf("%s defines no %s", kernel.str("entrypoint"), constant)
		}
		if declared := kernel.str(attribute); declared != match[1] {
			t.Errorf("surfaces.sysml says JupyterKernel.%s = %q, the command has %q", attribute, declared, match[1])
		}
	}
	for _, attribute := range []string{"connectionSetting", "installSetting"} {
		requireFlag(t, "JupyterKernel", attribute, kernel.str(attribute), kernel.str("entrypoint"))
	}

	pkg := kernel.str("goPackage")
	pkgSource := readGoPackage(t, filepath.Join("..", pkg))
	var mimeTypes []string
	for _, match := range regexp.MustCompile(`\bmime\w+\s*=\s*"([^"]+)"`).FindAllStringSubmatch(pkgSource, -1) {
		mimeTypes = append(mimeTypes, match[1])
	}
	if declared, actual := kernel.str("mimeTypes"), strings.Join(mimeTypes, ", "); declared != actual {
		t.Errorf("surfaces.sysml says JupyterKernel.mimeTypes = %q, %s displays %q", declared, pkg, actual)
	}
	wrapsRepl := packageImports(t, filepath.Join("..", pkg), `"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"`)
	if kernel.boolean("wrapsRepl") != wrapsRepl {
		t.Errorf("surfaces.sysml says JupyterKernel.wrapsRepl = %t, %s importing the REPL is %t", kernel.boolean("wrapsRepl"), pkg, wrapsRepl)
	}
}

// TestSelfModelJsonCommandsMatchImplementation compares each JSON-RPC command
// with its package: the methods its dispatcher serves, the shared transport it
// imports, and whether it loads the standard library.
func TestSelfModelJsonCommandsMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	toolchain := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "Toolchain")

	const library = `"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"`
	dispatchPattern := regexp.MustCompile(`(?s)\n\tswitch method \{\n(.*?)\n\t\}`)
	casePattern := regexp.MustCompile(`(?m)^\s*case ((?:"[A-Z]\w*",?\s*)+):`)
	namePattern := regexp.MustCompile(`"([^"]+)"`)
	commands := 0
	for name, part := range toolchain.parts() {
		if !part.has("serves") {
			continue
		}
		commands++
		pkg := part.str("goPackage")
		pkgSource := readGoPackage(t, filepath.Join("..", pkg))
		dispatch := dispatchPattern.FindStringSubmatch(pkgSource)
		if dispatch == nil {
			t.Errorf("%s has no switch over the method", pkg)
			continue
		}
		var methods []string
		for _, line := range casePattern.FindAllStringSubmatch(dispatch[1], -1) {
			for _, method := range namePattern.FindAllStringSubmatch(line[1], -1) {
				methods = append(methods, method[1])
			}
		}
		if declared, actual := part.str("serves"), strings.Join(methods, ", "); declared != actual {
			t.Errorf("surfaces.sysml says %s.serves = %q, %s dispatches %q", name, declared, pkg, actual)
		}
		transport := part.str("jsonTransport")
		if !packageImports(t, filepath.Join("..", pkg), `"github.com/Open-MBEE/OpenSysML/`+transport+`"`) {
			t.Errorf("surfaces.sysml says %s speaks over %s, which %s does not import", name, transport, pkg)
		}
		if part.has("stateless") && part.boolean("stateless") == packageImports(t, filepath.Join("..", pkg), library) {
			t.Errorf("surfaces.sysml says %s.stateless = %t, but %s loading the standard library is %t", name, part.boolean("stateless"), pkg, !part.boolean("stateless"))
		}
	}
	if commands == 0 {
		t.Fatal("surfaces.sysml models no JSON-RPC command")
	}
}

// TestSelfModelReportFormatsMatchImplementation compares the command line's
// report formats with the flag's usage text and the packages that render them.
func TestSelfModelReportFormatsMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	cli := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "CommandLine")

	requireFlag(t, "CommandLine", "reportSetting", cli.str("reportSetting"), cli.str("entrypoint"))
	cmdSource := readGoPackage(t, filepath.Join("..", cli.str("entrypoint")))
	match := regexp.MustCompile(`"` + regexp.QuoteMeta(strings.TrimPrefix(cli.str("reportSetting"), "-")) + `", "", "Report checks as ([a-z, ]+) or ([a-z]+);`).FindStringSubmatch(cmdSource)
	if match == nil {
		t.Fatalf("%s does not say which formats %s reports", cli.str("entrypoint"), cli.str("reportSetting"))
	}
	formats := append(strings.Split(match[1], ", "), match[2])
	if declared, actual := cli.str("reportFormats"), strings.Join(formats, ", "); declared != actual {
		t.Errorf("surfaces.sysml says CommandLine.reportFormats = %q, the usage says %q", declared, actual)
	}
	for _, format := range formats {
		if format == "text" || format == "json" {
			continue
		}
		if _, err := os.Stat(filepath.Join("..", cli.str("reportPackages"), format)); err != nil {
			t.Errorf("surfaces.sysml reports %s, but %s has no package for it", format, cli.str("reportPackages"))
		}
	}
}

// TestSelfModelEditorsMatchImplementation compares the VS Code extension's
// contributions with its manifest, and the notebook packages with theirs and
// with the Makefile targets that build, test and install them.
func TestSelfModelEditorsMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	makefile := readMakefile(t)

	vscode := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "VsCodeExtension")
	var manifest struct {
		Contributes struct {
			Commands  []struct{ Command string }
			Languages []struct{ ID string }
		}
	}
	readJSON(t, filepath.Join("..", vscode.str("goPackage"), "package.json"), &manifest)
	var commands, languages []string
	for _, command := range manifest.Contributes.Commands {
		commands = append(commands, command.Command)
	}
	for _, language := range manifest.Contributes.Languages {
		languages = append(languages, language.ID)
	}
	if declared, actual := vscode.str("commands"), strings.Join(commands, ", "); declared != actual {
		t.Errorf("surfaces.sysml says VsCodeExtension.commands = %q, package.json contributes %q", declared, actual)
	}
	if declared, actual := vscode.str("languages"), strings.Join(languages, ", "); declared != actual {
		t.Errorf("surfaces.sysml says VsCodeExtension.languages = %q, package.json contributes %q", declared, actual)
	}
	requireMakeTarget(t, makefile, "VsCodeExtension", "grammarTarget", vscode.str("grammarTarget"))
	if !strings.Contains(makefile, "./"+vscode.str("grammarGenerator")) && !strings.Contains(makefile, "/tools/gengrammar") {
		t.Errorf("the Makefile does not run %s", vscode.str("grammarGenerator"))
	}

	lab := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "JupyterLabExtension")
	var labManifest struct{ Name string }
	readJSON(t, filepath.Join("..", lab.str("goPackage"), "package.json"), &labManifest)
	if declared := lab.str("packageName"); declared != labManifest.Name {
		t.Errorf("surfaces.sysml says JupyterLabExtension.packageName = %q, package.json says %q", declared, labManifest.Name)
	}
	if table := lab.str("syntaxTable"); !strings.Contains(makefile, "-codemirror-out $(JUPYTERLAB_DIR)/"+strings.TrimPrefix(table, lab.str("goPackage")+"/")) {
		t.Errorf("the Makefile's grammar target does not write %s", table)
	}
	for _, attribute := range []string{"buildTarget", "testTarget"} {
		requireMakeTarget(t, makefile, "JupyterLabExtension", attribute, lab.str(attribute))
	}

	kernelPackage := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "JupyterKernelPackage")
	pyproject, err := os.ReadFile(filepath.Join("..", kernelPackage.str("goPackage"), "pyproject.toml"))
	if err != nil {
		t.Fatalf("read the kernel package's pyproject: %v", err)
	}
	match := regexp.MustCompile(`(?m)^name = "([^"]+)"`).FindStringSubmatch(string(pyproject))
	if match == nil {
		t.Fatal("the kernel package's pyproject names no project")
	}
	if declared := kernelPackage.str("packageName"); declared != match[1] {
		t.Errorf("surfaces.sysml says JupyterKernelPackage.packageName = %q, pyproject.toml says %q", declared, match[1])
	}
	for _, attribute := range []string{"installTarget", "testTarget"} {
		requireMakeTarget(t, makefile, "JupyterKernelPackage", attribute, kernelPackage.str(attribute))
	}
}

// TestSelfModelMarkupMatchesImplementation compares the markup parsers behind
// a prose body with the markup package: the languages it lists, which parser
// answers each, which one leaves the process, and the variable that names its tool.
func TestSelfModelMarkupMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	planner := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "DocumentPlanner")
	if declared, actual := planner.str("markupLanguages"), markup.Languages(); declared != actual {
		t.Errorf("surfaces.sysml says DocumentPlanner.markupLanguages = %q, the implementation has %q", declared, actual)
	}

	documents := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "DocumentPipeline")
	parsers := 0
	for name, part := range documents.parts() {
		if !part.has("markupLanguage") {
			continue
		}
		parsers++
		language := part.str("markupLanguage")
		parser, ok := markup.For(language)
		if !ok {
			t.Errorf("surfaces.sysml says %s parses %q, which the implementation does not", name, language)
			continue
		}
		if parser.Language() != language {
			t.Errorf("surfaces.sysml says %s parses %q, the parser calls it %q", name, language, parser.Language())
		}
		pkg := filepath.Join("..", part.str("goPackage"))
		if leaves := parserRunsTool(t, pkg, language); part.boolean("needsExternalTool") != leaves {
			t.Errorf("surfaces.sysml says %s.needsExternalTool = %t, the %s parser running a tool is %t", name, part.boolean("needsExternalTool"), language, leaves)
		}
		if part.has("toolEnv") && part.str("toolEnv") != markup.AsciiDocEnv {
			t.Errorf("surfaces.sysml says %s.toolEnv = %q, the implementation reads %q", name, part.str("toolEnv"), markup.AsciiDocEnv)
		}
	}
	if parsers != len(strings.Split(markup.Languages(), ", ")) {
		t.Errorf("surfaces.sysml models %d markup parsers, the implementation has %q", parsers, markup.Languages())
	}
}

// parserRunsTool reports whether the parser files for language in the markup
// package start an external process.
func parserRunsTool(t *testing.T, dir, language string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, language) || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(content), "exec.Command") {
			return true
		}
	}
	return false
}

// TestSelfModelFmiGateMatchesImplementation compares the modelled FMI gate
// with the suite that runs the Reference-FMUs: the variable that requires them,
// read by the test and set by CI, and the script that fetches them.
func TestSelfModelFmiGateMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	gate := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "ReferenceFmuGate")

	requireEnv := gate.str("requireEnv")
	if !strings.Contains(readGoTests(t, filepath.Join("..", gate.str("goPackage"))), `"`+requireEnv+`"`) {
		t.Errorf("surfaces.sysml says ReferenceFmuGate reads %s, %s does not", requireEnv, gate.str("goPackage"))
	}
	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "pr.yml"))
	if err != nil {
		t.Fatalf("read the PR workflow: %v", err)
	}
	if required := strings.Contains(string(workflow), requireEnv+`: "1"`); gate.boolean("gating") != required {
		t.Errorf("surfaces.sysml says ReferenceFmuGate.gating = %t, the PR workflow setting %s is %t", gate.boolean("gating"), requireEnv, required)
	}
	if _, err := os.Stat(filepath.Join("..", gate.str("baseline"))); err != nil {
		t.Errorf("surfaces.sysml says the Reference-FMUs come from %s, which does not exist", gate.str("baseline"))
	}
}

// readGoTests concatenates the test files of the suite at dir.
func readGoTests(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var text strings.Builder
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		text.Write(content)
	}
	return text.String()
}

// readJSON decodes one JSON file into v.
func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(content, v); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

// contains reports whether words holds word.
func contains(words []string, word string) bool {
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}
