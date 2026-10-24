package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// The binary is what an agent host starts, so the session is tested through
// it: the transport an MCP host speaks and the exit it waits for are only
// observable from outside the process.
var (
	buildOnce   sync.Once
	builtServer string
	buildErr    error
)

// serverBinary builds cmd/sysml-mcp once per test binary and returns its path.
func serverBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "sysml-mcp-build")
		if err != nil {
			buildErr = err
			return
		}
		builtServer = filepath.Join(dir, "sysml-mcp")
		build := exec.Command("go", gobuild.Args(builtServer)...)
		if out, err := build.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building the server: %v", buildErr)
	}
	return builtServer
}

func TestTheBinaryServesTheProtocolOverStdio(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "m.sysml"), []byte("package M { part def P; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(serverBinary(t), "-stdio", "-root", root)
	ctx := protocolBudget(t)
	cmd.Stderr = os.Stderr
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if info := cs.InitializeResult().ServerInfo; info.Name != "sysml-mcp" {
		t.Errorf("server info = %+v", info)
	}
	if cs.InitializeResult().Instructions == "" {
		t.Error("the server gave the client no instructions")
	}

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := catalog.Names(catalog.Groups()...)
	if !slices.Equal(names, want) {
		t.Errorf("tools = %v, want the whole catalog %v", names, want)
	}

	cat, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "tool_catalog", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("tool_catalog: %v", err)
	}
	if cat.IsError {
		t.Fatalf("tool_catalog failed: %+v", cat.Content)
	}
	var listed struct {
		Groups []string `json:"groups"`
		Tools  []struct {
			Name      string `json:"name"`
			Operation string `json:"operation"`
			Summary   string `json:"summary"`
		} `json:"tools"`
	}
	raw, _ := json.Marshal(cat.StructuredContent)
	if err := json.Unmarshal(raw, &listed); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(listed.Groups, []string{"lsp", "runtime", "session"}) {
		t.Errorf("tool_catalog groups = %v", listed.Groups)
	}
	var catalogued []string
	for _, tool := range listed.Tools {
		catalogued = append(catalogued, tool.Name)
		if tool.Operation == "" || tool.Summary == "" {
			t.Errorf("tool_catalog describes %s without an operation or summary: %+v", tool.Name, tool)
		}
	}
	slices.Sort(catalogued)
	if !slices.Equal(catalogued, names) {
		t.Errorf("tool_catalog lists %v\ntools/list answers %v", catalogued, names)
	}

	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "workspace_info", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("workspace_info: %v", err)
	}
	if res.IsError {
		t.Fatalf("workspace_info failed: %+v", res.Content)
	}
	var info struct {
		Documents []string `json:"documents"`
	}
	raw, _ = json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(info.Documents, []string{filepath.Join(root, "m.sysml")}) {
		t.Errorf("documents = %v", info.Documents)
	}

	// Closing the client's end of the stream ends the session; the server exits
	// on its own, with the status of a session served to its end.
	if err := cs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if code := cmd.ProcessState.ExitCode(); code != exitServed {
		t.Errorf("exit status = %d, want %d", code, exitServed)
	}
}
