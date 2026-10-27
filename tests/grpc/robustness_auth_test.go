package grpc_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// buildService builds the sysml-grpc command once per test binary.
func buildService(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sysml-grpc")
	build := exec.Command("go", gobuild.Args(path)...)
	build.Dir = filepath.Join("..", "..", "cmd", "sysml-grpc")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building sysml-grpc: %v\n%s", err, out)
	}
	return path
}

// TestGRPCRobustnessAuthBinary drives the built command as an operator would:
// a non-loopback bind without a token is refused, and a token-protected port
// admits only the token, on the Connect path a client outside this module uses.
func TestGRPCRobustnessAuthBinary(t *testing.T) {
	binary := buildService(t)

	t.Run("non-loopback without a token is refused at startup", func(t *testing.T) {
		cmd := exec.Command(binary, "-host", "0.0.0.0", "-port", "0", "-health-port", "0")
		cmd.Env = append(os.Environ(), "OPENSYSML_GRPC_TOKEN=")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("the service started without a token on 0.0.0.0:\n%s", out)
		}
		for _, want := range []string{"-token-file", "-token", "OPENSYSML_GRPC_TOKEN", "-allow-unauthenticated"} {
			if !strings.Contains(string(out), want) {
				t.Errorf("refusal does not name %s:\n%s", want, out)
			}
		}
	})

	t.Run("a token in clear on every interface is refused at startup", func(t *testing.T) {
		cmd := exec.Command(binary, "-host", "0.0.0.0", "-port", "0", "-health-port", "0", "-token", "conformance-token_1")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("the service served a bearer token in clear on 0.0.0.0:\n%s", out)
		}
		for _, want := range []string{"in clear", "-tls-cert and -tls-key", "-allow-insecure-token-transport"} {
			if !strings.Contains(string(out), want) {
				t.Errorf("the refusal does not say %q:\n%s", want, out)
			}
		}
	})

	t.Run("a token-protected port admits only the token", func(t *testing.T) {
		const token = "conformance-token_1"
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "-port", "0", "-health-port", "0", "-report-address", "-token", token)
		cmd.Stderr = os.Stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() { cancel(); _ = cmd.Wait() }()
		address, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			t.Fatalf("the service reported no address: %v", err)
		}
		url := fmt.Sprintf("http://%s/sysml.SysMLService/GetServerInfo", strings.TrimSpace(address))
		client := &http.Client{Timeout: 10 * time.Second}
		call := func(authorization string) *http.Response {
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString("{}"))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			if authorization != "" {
				req.Header.Set("Authorization", authorization)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { resp.Body.Close() })
			return resp
		}
		for _, tc := range []struct {
			name, authorization string
			status              int
		}{
			{"nothing presented", "", http.StatusUnauthorized},
			{"the wrong token", "Bearer " + token + "x", http.StatusUnauthorized},
			{"the right token", "Bearer " + token, http.StatusOK},
		} {
			resp := call(tc.authorization)
			if resp.StatusCode != tc.status {
				t.Errorf("%s: status %d, want %d", tc.name, resp.StatusCode, tc.status)
			}
			if tc.status == http.StatusUnauthorized && !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), `Bearer realm="sysml-grpc"`) {
				t.Errorf("%s: WWW-Authenticate %q", tc.name, resp.Header.Get("WWW-Authenticate"))
			}
		}
		health := fmt.Sprintf("http://%s/health", strings.TrimSpace(address))
		resp, err := client.Get(health)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("/health without a token: status %d", resp.StatusCode)
		}
	})
}
