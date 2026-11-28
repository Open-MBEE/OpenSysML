package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFreePortsAreDistinct(t *testing.T) {
	port, healthPort, err := freePorts()
	if err != nil {
		t.Fatalf("freePorts: %v", err)
	}
	if port == healthPort {
		t.Fatalf("both ports are %d; the service cannot bind one on top of the other", port)
	}
}

func TestStartServiceReportsAProcessThatExits(t *testing.T) {
	// false lives in /bin on Linux but only in /usr/bin on macOS.
	exits, err := exec.LookPath("false")
	if err != nil {
		t.Fatalf("no false utility on PATH: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "service.log")
	start := time.Now()
	_, err = startService(context.Background(), exits, logPath, 4)
	if err == nil {
		t.Fatal("startService accepted a binary that exits immediately")
	}
	if !strings.Contains(err.Error(), "exited before answering") {
		t.Fatalf("error is %q, want it to report the exit", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("the exit was waited out for %s rather than reported", elapsed)
	}
}
