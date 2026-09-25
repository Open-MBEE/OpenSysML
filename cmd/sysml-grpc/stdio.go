// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"log/slog"
	"os"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/stdiorpc"
)

// runStdio serves one client over stdin/stdout and reports the exit status the
// session earned. Logging already goes to stderr, so it does not corrupt the
// frames on stdout.
func runStdio(svc *sysmlgrpc.Service) int {
	slog.Info("Serving over stdin/stdout", "framing", "Content-Length")
	err := stdiorpc.NewServer(svc).Serve(context.Background(), os.Stdin, os.Stdout)
	svc.Close()
	if err != nil {
		slog.Error("stdio session ended in a protocol error", "error", err)
		return 1
	}
	return 0
}
