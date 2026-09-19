package hygiene

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/Open-MBEE/OpenSysML"

// dependencies lists the import paths of pkg and everything it links.
func dependencies(t *testing.T, pkg string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", pkg)
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}
	return strings.Fields(string(out))
}

// TestREPLDoesNotDependOnTheService keeps the terminal REPL off the RPC stack: the
// instance graph it serializes for `features` comes from protoconv, which both
// frontends share, not from the Connect service layer.
func TestREPLDoesNotDependOnTheService(t *testing.T) {
	for _, pkg := range []string{"./internal/repl", "./cmd/sysml"} {
		for _, dep := range dependencies(t, pkg) {
			switch {
			case dep == module+"/internal/grpc",
				dep == module+"/api/proto/protoconnect",
				strings.HasPrefix(dep, "connectrpc.com/"):
				t.Errorf("%s depends on %s", pkg, dep)
			}
		}
	}
}

// TestProtoconvImportsOnlyTheMessages keeps the proto conversion package a leaf of the
// frontends: it reads the API's messages and the core packages, never a transport.
func TestProtoconvImportsOnlyTheMessages(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{join .Imports \"\\n\"}}", "./internal/protoconv")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	belowFrontend := []string{
		"/internal/core/",
		"/internal/syntax/",
		"/internal/semantic/",
		"/internal/ir/",
		"/internal/check/",
		"/internal/exec/",
		"/internal/translate/",
		"/internal/doc/",
		"/internal/workspace/",
	}
	for _, imp := range strings.Fields(string(out)) {
		core := false
		for _, prefix := range belowFrontend {
			if strings.HasPrefix(imp, module+prefix) {
				core = true
				break
			}
		}
		switch {
		case !strings.Contains(imp, "."),
			imp == module+"/api/proto",
			core,
			strings.HasPrefix(imp, "google.golang.org/protobuf/"):
		default:
			t.Errorf("internal/protoconv imports %s", imp)
		}
	}
	for _, dep := range dependencies(t, "./internal/protoconv") {
		if dep == module+"/internal/grpc" || strings.HasPrefix(dep, "connectrpc.com/") {
			t.Errorf("internal/protoconv depends on %s", dep)
		}
	}
}
