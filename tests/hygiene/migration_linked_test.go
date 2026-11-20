//go:build !sysml_re && !sysml_nov1

package hygiene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

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
		if !isAnnounced[name] {
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
