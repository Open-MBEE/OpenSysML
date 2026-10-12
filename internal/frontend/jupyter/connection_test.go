package jupyter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConnection(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kernel.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const sampleConnection = `{
  "shell_port": 50001, "iopub_port": 50002, "stdin_port": 50003, "control_port": 50004, "hb_port": 50005,
  "ip": "127.0.0.1", "key": "abc", "transport": "tcp", "signature_scheme": "hmac-sha256", "kernel_name": "sysml"
}`

func TestAConnectionFileReadsAsJupyterWritesIt(t *testing.T) {
	info, err := ReadConnectionFile(writeConnection(t, sampleConnection))
	if err != nil {
		t.Fatal(err)
	}
	want := ConnectionInfo{Transport: "tcp", IP: "127.0.0.1", ShellPort: 50001, IOPubPort: 50002, StdinPort: 50003,
		ControlPort: 50004, HBPort: 50005, Key: "abc", SignatureScheme: "hmac-sha256", KernelName: "sysml"}
	if info != want {
		t.Errorf("read %+v, want %+v", info, want)
	}
	if got := info.endpoint(info.ShellPort); got != "tcp://127.0.0.1:50001" {
		t.Errorf("endpoint = %q", got)
	}
}

func TestConnectionFilesThatCannotBeServedAreRefused(t *testing.T) {
	cases := map[string]string{
		"missing file":        "",
		"not JSON":            `{`,
		"unknown transport":   strings.Replace(sampleConnection, `"tcp"`, `"udp"`, 1),
		"no ip":               strings.Replace(sampleConnection, `"127.0.0.1"`, `""`, 1),
		"port zero":           strings.Replace(sampleConnection, `"hb_port": 50005`, `"hb_port": 0`, 1),
		"unknown signing":     strings.Replace(sampleConnection, `"hmac-sha256"`, `"hmac-md5"`, 1),
		"negative shell port": strings.Replace(sampleConnection, `"shell_port": 50001`, `"shell_port": -1`, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "absent.json")
			if body != "" {
				path = writeConnection(t, body)
			}
			if _, err := ReadConnectionFile(path); err == nil {
				t.Error("read without error")
			}
		})
	}
}

func TestAnIPCConnectionNeedsNoPorts(t *testing.T) {
	info := ConnectionInfo{Transport: "ipc", IP: "/tmp/kernel", Key: "k", SignatureScheme: "hmac-sha256"}
	if err := info.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
	if got := info.endpoint(3); got != "ipc:///tmp/kernel-3" {
		t.Errorf("endpoint = %q", got)
	}
}

func TestAConnectionWithoutAKeyIsAllowed(t *testing.T) {
	info := ConnectionInfo{Transport: "tcp", IP: "127.0.0.1", ShellPort: 1, IOPubPort: 2, StdinPort: 3, ControlPort: 4, HBPort: 5}
	if err := info.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}
