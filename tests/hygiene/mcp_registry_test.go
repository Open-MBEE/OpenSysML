package hygiene

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
)

// The MCP Registry listing of sysml-mcp (packaging/mcp/server.json) is metadata
// over the npm package @openmbee/sysml-mcp; scripts/mcp-registry-publish.sh fills
// the tag's version in and publishes it on each release tag.
const (
	mcpRegistryServerJSON  = "packaging/mcp/server.json"
	mcpRegistrySchema      = "tests/hygiene/testdata/mcp-registry/server.schema.json"
	mcpRegistryWorkflow    = ".github/workflows/mcp-registry.yml"
	mcpRegistryName        = "org.opensysml/sysml-mcp"
	mcpRegistryPackage     = "@openmbee/sysml-mcp"
	mcpRegistryPlaceholder = "0.0.0-unreleased"
	mcpRegistryMetaKey     = "io.modelcontextprotocol.registry/publisher-provided"
)

type mcpRegistryServer struct {
	Schema      string `json:"$schema"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	WebsiteURL  string `json:"websiteUrl"`
	Repository  struct {
		URL       string `json:"url"`
		Source    string `json:"source"`
		Subfolder string `json:"subfolder"`
	} `json:"repository"`
	Version  string `json:"version"`
	Packages []struct {
		RegistryType string `json:"registryType"`
		Identifier   string `json:"identifier"`
		Version      string `json:"version"`
		RuntimeHint  string `json:"runtimeHint"`
		Transport    struct {
			Type string `json:"type"`
		} `json:"transport"`
		PackageArguments []struct {
			Type        string `json:"type"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Default     string `json:"default"`
		} `json:"packageArguments"`
	} `json:"packages"`
	Meta map[string]struct {
		ToolGroups []string `json:"toolGroups"`
		Tools      int      `json:"tools"`
	} `json:"_meta"`
}

func loadMCPRegistryServer(t *testing.T) (mcpRegistryServer, string) {
	t.Helper()
	raw := readRepoFile(t, mcpRegistryServerJSON)
	var server mcpRegistryServer
	if err := json.Unmarshal([]byte(raw), &server); err != nil {
		t.Fatalf("parse %s: %v", mcpRegistryServerJSON, err)
	}
	return server, raw
}

func TestMCPRegistryServerJSONMatchesTheSchemaItNames(t *testing.T) {
	server, raw := loadMCPRegistryServer(t)
	schemaRaw := readRepoFile(t, mcpRegistrySchema)
	var schemaID struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal([]byte(schemaRaw), &schemaID); err != nil {
		t.Fatalf("parse %s: %v", mcpRegistrySchema, err)
	}
	if server.Schema != schemaID.ID {
		t.Fatalf("server.json names schema %q but the vendored copy is %q; update both together", server.Schema, schemaID.ID)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaRaw))
	if err != nil {
		t.Fatalf("decode %s: %v", mcpRegistrySchema, err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaID.ID, schemaDoc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(schemaID.ID)
	if err != nil {
		t.Fatalf("compile %s: %v", mcpRegistrySchema, err)
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("decode %s: %v", mcpRegistryServerJSON, err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("%s does not satisfy %s:\n%v", mcpRegistryServerJSON, server.Schema, err)
	}
}

func TestMCPRegistryServerJSONListsTheNpmPackageWithItsRootFlag(t *testing.T) {
	server, _ := loadMCPRegistryServer(t)
	if server.Name != mcpRegistryName {
		t.Errorf("name is %q, not %s", server.Name, mcpRegistryName)
	}
	if server.Version != mcpRegistryPlaceholder {
		t.Errorf("version is %q; the committed file carries the placeholder %s, which the publish script replaces with the tag's version", server.Version, mcpRegistryPlaceholder)
	}
	if server.Title == "" || server.Description == "" || server.WebsiteURL == "" {
		t.Error("title, description and websiteUrl must all be set; galleries render them")
	}
	if server.Repository.URL != "https://github.com/Open-MBEE/OpenSysML" || server.Repository.Source != "github" || server.Repository.Subfolder != "cmd/sysml-mcp" {
		t.Errorf("repository is %+v, not this repository's cmd/sysml-mcp", server.Repository)
	}
	if len(server.Packages) != 1 {
		t.Fatalf("server.json lists %d packages; the npm package is the one artefact", len(server.Packages))
	}
	pkg := server.Packages[0]
	if pkg.RegistryType != "npm" || pkg.Identifier != mcpRegistryPackage {
		t.Errorf("package is %s %q, not npm %s", pkg.RegistryType, pkg.Identifier, mcpRegistryPackage)
	}
	if pkg.Version != mcpRegistryPlaceholder {
		t.Errorf("package version is %q, not the placeholder %s the server version carries", pkg.Version, mcpRegistryPlaceholder)
	}
	if pkg.Transport.Type != "stdio" {
		t.Errorf("transport is %q; sysml-mcp speaks stdio only", pkg.Transport.Type)
	}
	if pkg.RuntimeHint != "npx" {
		t.Errorf("runtimeHint is %q; hosts run the package with npx", pkg.RuntimeHint)
	}
	var root *struct {
		Type        string `json:"type"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Default     string `json:"default"`
	}
	for i := range pkg.PackageArguments {
		if pkg.PackageArguments[i].Name == "-root" {
			root = &pkg.PackageArguments[i]
		}
	}
	if root == nil {
		t.Fatal("packageArguments does not declare -root; galleries render a named argument with its description and default")
	}
	if root.Type != "named" || root.Description == "" || root.Default == "" {
		t.Errorf("-root is %+v; it must be a named argument with a description and a default", *root)
	}
	usage := readRepoFile(t, "docs/reference/sysml-mcp.md")
	if !strings.Contains(usage, "| `-root DIR` | `"+root.Default+"` |") {
		t.Errorf("the -root default %q in server.json is not the one docs/reference/sysml-mcp.md's command-line table states", root.Default)
	}
}

func TestMCPRegistryServerJSONAgreesWithTheToolCatalogue(t *testing.T) {
	server, _ := loadMCPRegistryServer(t)
	meta, ok := server.Meta[mcpRegistryMetaKey]
	if !ok {
		t.Fatalf("server.json has no _meta.%q block recording the shipped tool catalogue", mcpRegistryMetaKey)
	}
	groups := make([]string, 0, len(catalog.Groups()))
	for _, group := range catalog.Groups() {
		groups = append(groups, string(group))
	}
	if !reflect.DeepEqual(meta.ToolGroups, groups) {
		t.Errorf("toolGroups is %v; the catalogue serves %v", meta.ToolGroups, groups)
	}
	if want := len(catalog.Names(catalog.Groups()...)); meta.Tools != want {
		t.Errorf("tools is %d; the catalogue ships %d", meta.Tools, want)
	}
}

// The npm package's package.json must declare the server name as mcpName: the
// registry verifies npm ownership by it. The package ships in its own tree;
// until a package.json names it, only the server.json side can be checked.
func TestMCPRegistryServerJSONAgreesWithTheNpmPackage(t *testing.T) {
	server, _ := loadMCPRegistryServer(t)
	var found []string
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "build", "bin", "site":
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "package.json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var pkg struct {
			Name    string `json:"name"`
			MCPName string `json:"mcpName"`
		}
		if err := json.Unmarshal(raw, &pkg); err != nil {
			return nil
		}
		if pkg.Name != mcpRegistryPackage {
			return nil
		}
		found = append(found, path)
		if pkg.MCPName != server.Name {
			t.Errorf("%s declares mcpName %q; the registry requires %q, the server.json name", path, pkg.MCPName, server.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the tree: %v", err)
	}
	if len(found) == 0 {
		t.Skipf("no package.json in the tree names %s yet; the mcpName check starts once the npm package ships", mcpRegistryPackage)
	}
	if len(found) > 1 {
		t.Errorf("%d package.json files name %s: %v", len(found), mcpRegistryPackage, found)
	}
}

func TestMCPRegistryWorkflowPublishesReleasesThroughThePinnedScript(t *testing.T) {
	raw := readRepoFile(t, mcpRegistryWorkflow)
	var workflow struct {
		On struct {
			Push struct {
				Tags []string `yaml:"tags"`
			} `yaml:"push"`
		} `yaml:"on"`
		actionsWorkflow `yaml:",inline"`
	}
	if err := yaml.Unmarshal([]byte(raw), &workflow); err != nil {
		t.Fatalf("parse %s: %v", mcpRegistryWorkflow, err)
	}
	if !reflect.DeepEqual(workflow.On.Push.Tags, []string{"v*"}) {
		t.Errorf("the workflow runs on tags %v, not v*", workflow.On.Push.Tags)
	}
	if workflow.Permissions["contents"] != "read" {
		t.Errorf("the workflow's contents permission is %q, not read; it publishes nothing to this repository", workflow.Permissions["contents"])
	}
	publish, ok := workflow.Jobs["publish"]
	if !ok {
		t.Fatalf("%s has no publish job", mcpRegistryWorkflow)
	}
	if stepIndex(publish.runSteps(), `scripts/mcp-registry-publish.sh "$TAG"`) < 0 {
		t.Error("the publish job does not run scripts/mcp-registry-publish.sh on the tag; the script holds the npm wait, the pinned publisher and the failure messages")
	}
	requireFragments(t, mcpRegistryWorkflow, raw,
		"secrets.MCP_REGISTRY_PRIVATE_KEY",
		"https://staging.registry.modelcontextprotocol.io",
	)
	script := readRepoFile(t, "scripts/mcp-registry-publish.sh")
	requireFragments(t, "scripts/mcp-registry-publish.sh", script,
		`SERVER_JSON="`+mcpRegistryServerJSON+`"`,
		`DOMAIN="opensysml.org"`,
		"registry.npmjs.org",
		"scripts/install-mcp-publisher.sh",
		"login dns --domain",
		"MCP_REGISTRY_PRIVATE_KEY is not set",
		"is not on registry.npmjs.org after",
		"DNS login for",
	)
	installer := readRepoFile(t, "scripts/install-mcp-publisher.sh")
	requireFragments(t, "scripts/install-mcp-publisher.sh", installer, "MCP_PUBLISHER_VERSION=", "sha256sum", "refusing to install")
	if bytes.Count([]byte(installer), []byte("MCP_PUBLISHER_VERSION=")) != 1 {
		t.Error("the installer pins mcp-publisher more than once")
	}
}
