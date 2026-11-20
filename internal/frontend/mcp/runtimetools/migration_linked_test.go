//go:build !sysml_re && !sysml_nov1

package runtimetools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

func TestConvertMigrateAndApplyEditsAnswerText(t *testing.T) {
	h := attach(t, newService(t))
	hash := h.parse()

	var convert pb.ConvertResponse
	_, out := h.ok("convert", map[string]any{"modelHash": hash, "toFormat": "ttl"}, &convert)
	if convert.GetError() != "" || !strings.Contains(convert.GetContent(), "@prefix") || !convert.GetExperimental() {
		t.Errorf("to ttl: error %q, experimental %v\n%s", convert.GetError(), convert.GetExperimental(), out)
	}
	if !strings.Contains(out, "notice: ") || !strings.Contains(out, strings.TrimRight(convert.GetContent(), "\n")) {
		t.Errorf("text:\n%s", out)
	}
	h.ok("convert", map[string]any{"filePath": "demo.sysml", "toFormat": "sysml"}, &convert)
	if convert.GetFromFormat() != "sysml" || !strings.Contains(convert.GetContent(), "part def Vehicle") {
		t.Errorf("file to notation: from %q\n%s", convert.GetFromFormat(), convert.GetContent())
	}
	h.ok("convert", map[string]any{"content": "package P { part def Q; }", "fromFormat": "sysml", "toFormat": "api-json"}, &convert)
	if !strings.Contains(convert.GetContent(), "PartDefinition") {
		t.Errorf("content to api-json:\n%s", convert.GetContent())
	}
	h.ok("convert", map[string]any{"content": "", "fromFormat": "sysml", "toFormat": "sysml"}, &convert)
	if convert.GetFromFormat() != "sysml" {
		t.Errorf("an empty inline document was not converted: %v", &convert)
	}
	if msg := h.fails("convert", map[string]any{"toFormat": "ttl"}); !strings.Contains(msg, "exactly one source") {
		t.Errorf("convert of nothing: %s", msg)
	}
	if msg := h.fails("convert", map[string]any{"modelHash": hash, "filePath": "demo.sysml", "toFormat": "ttl"}); !strings.Contains(msg, "exactly one source") {
		t.Errorf("convert of two sources: %s", msg)
	}
	if msg := h.fails("convert", map[string]any{"modelHash": hash}); !strings.Contains(msg, "toFormat") {
		t.Errorf("convert without a target format: %s", msg)
	}

	xmi, err := os.ReadFile(migrateFixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.root, "v1.xmi"), xmi, 0o600); err != nil {
		t.Fatal(err)
	}
	var migrate pb.MigrateResponse
	_, out = h.ok("migrate", map[string]any{"filePath": "v1.xmi", "toFormat": "sysml", "report": true}, &migrate)
	if migrate.GetError() != "" || !strings.Contains(migrate.GetContent(), "occurrence def A") || migrate.GetReport().GetMapped() == 0 {
		t.Errorf("migrate: error %q, mapped %d\n%s", migrate.GetError(), migrate.GetReport().GetMapped(), migrate.GetContent())
	}
	if !strings.Contains(out, migrate.GetReport().GetSummary()) || !strings.Contains(out, strings.TrimRight(migrate.GetContent(), "\n")) {
		t.Errorf("text:\n%s", out)
	}
	h.ok("migrate", map[string]any{"content": base64.StdEncoding.EncodeToString(xmi), "fromFormat": "xmi", "toFormat": "sysml"}, &migrate)
	if migrate.GetError() != "" || !strings.Contains(migrate.GetContent(), "occurrence def B") {
		t.Errorf("migrate inline: error %q\n%s", migrate.GetError(), migrate.GetContent())
	}
	if msg := h.fails("migrate", map[string]any{"toFormat": "sysml"}); !strings.Contains(msg, "exactly one source") {
		t.Errorf("migrate of nothing: %s", msg)
	}

	var edits pb.ApplyEditsResponse
	_, out = h.ok("apply_edits", map[string]any{
		"modelHash":  hash,
		"operations": []any{map[string]any{"setValue": map[string]any{"target": "Demo::sedan::mass", "value": "1300.0"}}},
	}, &edits)
	if edits.GetError() != "" || !strings.Contains(edits.GetContent(), "mass = 1300.0") || len(edits.GetApplied()) != 1 {
		t.Errorf("set value: error %q, %d applied\n%s", edits.GetError(), len(edits.GetApplied()), edits.GetContent())
	}
	if !strings.Contains(out, `"1200.0" -> "1300.0"`) || !strings.Contains(out, strings.TrimRight(edits.GetContent(), "\n")) {
		t.Errorf("text:\n%s", out)
	}
	if data, err := os.ReadFile(filepath.Join(h.root, "demo.sysml")); err != nil || string(data) != demoModel {
		t.Error("apply_edits wrote to the workspace")
	}
	_, out = h.ok("apply_edits", map[string]any{
		"modelHash":  hash,
		"operations": []any{map[string]any{"setValue": map[string]any{"target": "Demo::nowhere", "value": "1"}}},
	}, &edits)
	if edits.GetError() == "" || edits.GetFailure() != pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET || !strings.Contains(out, "EDIT_FAILURE_UNKNOWN_TARGET") {
		t.Errorf("unknown target: %q %v\n%s", edits.GetError(), edits.GetFailure(), out)
	}
	if msg := h.fails("apply_edits", map[string]any{"modelHash": hash, "operations": []any{map[string]any{"setvalue": map[string]any{}}}}); !strings.Contains(msg, "ApplyEditsRequest") {
		t.Errorf("a misspelled operation arm: %s", msg)
	}
}

// Every capability the service can report is served by a registered tool, so
// server_info never advertises what no tool acts on; a capability the service
// withholds is refused by that tool (TestAWithheldCapabilityIsUnimplemented).
func TestEveryCapabilityIsServedByATool(t *testing.T) {
	h := attach(t, newService(t))
	res, err := h.client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, tool := range res.Tools {
		registered[tool.Name] = true
	}
	for _, capability := range sysmlgrpc.Capabilities() {
		tool, mapped := CapabilityTools[capability]
		if !mapped {
			t.Errorf("capability %q is served by no tool", capability)
			continue
		}
		if !registered[tool] {
			t.Errorf("capability %q names tool %q, which is not registered", capability, tool)
		}
	}
	for capability := range CapabilityTools {
		if !slices.Contains(sysmlgrpc.Capabilities(), capability) {
			t.Errorf("CapabilityTools names %q, which the service does not report", capability)
		}
	}
	var info pb.ServerInfoResponse
	resp := h.call("server_info", map[string]any{})
	raw, _ := json.Marshal(resp.StructuredContent)
	if err := protojson.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	for _, capability := range info.GetCapabilities() {
		if !registered[CapabilityTools[capability]] {
			t.Errorf("server_info advertises %q without a tool behind it", capability)
		}
	}
}
