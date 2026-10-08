// Package jupyter serves the Jupyter messaging protocol over ZeroMQ for a
// kernel: the five channels a connection file names, the signing of every
// message, and the request kinds a front end sends, each answered by an
// Engine. The engine over the REPL session is in this package too.
package jupyter

import (
	"encoding/json"
	"fmt"
	"os"
)

// ConnectionInfo is what a Jupyter connection file holds: where each channel
// listens, and the key messages are signed with.
type ConnectionInfo struct {
	Transport       string `json:"transport"`
	IP              string `json:"ip"`
	ShellPort       int    `json:"shell_port"`
	IOPubPort       int    `json:"iopub_port"`
	StdinPort       int    `json:"stdin_port"`
	ControlPort     int    `json:"control_port"`
	HBPort          int    `json:"hb_port"`
	Key             string `json:"key"`
	SignatureScheme string `json:"signature_scheme"`
	KernelName      string `json:"kernel_name,omitempty"`
}

// ReadConnectionFile reads the connection file a front end wrote for the kernel.
func ReadConnectionFile(path string) (ConnectionInfo, error) {
	// #nosec G304 -- the front end names the connection file on the command line.
	raw, err := os.ReadFile(path)
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("read connection file: %w", err)
	}
	var info ConnectionInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return ConnectionInfo{}, fmt.Errorf("connection file %s: %w", path, err)
	}
	if err := info.Validate(); err != nil {
		return ConnectionInfo{}, fmt.Errorf("connection file %s: %w", path, err)
	}
	return info, nil
}

// Validate reports a connection the kernel cannot serve: a transport other than
// TCP or IPC, a channel without a port, or a signature scheme other than
// HMAC-SHA256 when a key is set.
func (c ConnectionInfo) Validate() error {
	switch c.Transport {
	case "tcp", "ipc":
	case "":
		return fmt.Errorf("no transport")
	default:
		return fmt.Errorf("transport %q is not tcp or ipc", c.Transport)
	}
	if c.IP == "" {
		return fmt.Errorf("no ip")
	}
	for _, port := range []struct {
		name string
		port int
	}{{"shell_port", c.ShellPort}, {"iopub_port", c.IOPubPort}, {"stdin_port", c.StdinPort}, {"control_port", c.ControlPort}, {"hb_port", c.HBPort}} {
		if port.port <= 0 && c.Transport == "tcp" {
			return fmt.Errorf("no %s", port.name)
		}
	}
	if c.Key != "" && c.SignatureScheme != "hmac-sha256" {
		return fmt.Errorf("signature scheme %q is not hmac-sha256", c.SignatureScheme)
	}
	return nil
}

// endpoint is the ZeroMQ endpoint of one channel.
func (c ConnectionInfo) endpoint(port int) string {
	if c.Transport == "ipc" {
		return fmt.Sprintf("ipc://%s-%d", c.IP, port)
	}
	return fmt.Sprintf("tcp://%s:%d", c.IP, port)
}
