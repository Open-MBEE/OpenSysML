// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// sysml-grpc starts a gRPC server exposing SysML v2 parsing, resolution, and semantic services.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/buildinfo"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/usage"
)

// Build metadata, set by the linker: the names match the -X flags the Makefile
// and the release build pass, so a released binary reports what it was built from.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// build is what this binary reports about itself: the linker's stamps, or the
// module version and VCS metadata the toolchain recorded when none were passed.
func build() buildinfo.Info {
	return buildinfo.Resolve(buildinfo.Stamps{Version: Version, Commit: Commit, BuildTime: BuildTime})
}

func main() {
	opts := registerFlags(flag.CommandLine)
	flag.Usage = func() { doc().WriteText(flag.CommandLine.Output(), flag.CommandLine) }
	flag.Parse()

	// The page asked for is the result of the run, as the help is.
	if opts.showMan {
		doc().WriteRoff(os.Stdout, flag.CommandLine, usage.DefaultManMeta())
		return
	}

	switch opts.transport {
	case transportGRPC, transportConnect, transportStdio:
	default:
		fmt.Fprintf(os.Stderr, "sysml-grpc: unknown -transport %q; want grpc, connect or stdio\n", opts.transport)
		os.Exit(2)
	}
	if (opts.tlsCert == "") != (opts.tlsKey == "") {
		fmt.Fprintln(os.Stderr, "sysml-grpc: -tls-cert and -tls-key must be supplied together")
		os.Exit(2)
	}
	origins, err := parseCORSOrigins(opts.corsOrigins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml-grpc: %v\n", err)
		os.Exit(2)
	}

	token, _, err := guard.Read(opts.tokenFile, opts.token, tokenEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml-grpc: %v\n", err)
		os.Exit(2)
	}
	if token.Set() && opts.allowUnauthenticated {
		fmt.Fprintln(os.Stderr, "sysml-grpc: -allow-unauthenticated and a bearer token contradict each other; give one")
		os.Exit(2)
	}

	if opts.transport == transportStdio {
		for _, f := range []struct{ name, value string }{{"-host", opts.host}, {"-token", opts.token}, {"-token-file", opts.tokenFile}} {
			if f.value != "" {
				fmt.Fprintf(os.Stderr, "sysml-grpc: %s is not for -transport stdio, which binds no address\n", f.name)
				os.Exit(2)
			}
		}
		switch {
		case opts.allowUnauthenticated:
			fmt.Fprintln(os.Stderr, "sysml-grpc: -allow-unauthenticated is not for -transport stdio, which binds no address")
			os.Exit(2)
		case opts.allowInsecureToken:
			fmt.Fprintln(os.Stderr, "sysml-grpc: -allow-insecure-token-transport is not for -transport stdio, which binds no address")
			os.Exit(2)
		case opts.exitWithParent:
			fmt.Fprintln(os.Stderr,
				"sysml-grpc: -exit-with-parent is not for -transport stdio, whose stdin is "+
					"the session and already ends it at end of file")
			os.Exit(2)
		case opts.reportAddress:
			fmt.Fprintln(os.Stderr,
				"sysml-grpc: -report-address is not for -transport stdio, which binds no "+
					"address and speaks the protocol itself on stdout")
			os.Exit(2)
		}
	}

	info := build()
	if opts.showVersion {
		fmt.Printf("sysml-grpc version %s\n", info.Version)
		fmt.Printf("commit: %s\n", info.Commit)
		fmt.Printf("built: %s\n", info.BuildTime)
		os.Exit(0)
	}

	// Refused before the service is built and before anything listens, so a build
	// that cannot offer a reachable address says so instead of waiting on one.
	if err := serveRefusal(opts.transport); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// Configure logging
	var lvl slog.Level
	switch opts.logLevel {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)

	slog.Info("Starting sysml-grpc server",
		"version", info.Version,
		"commit", info.Commit,
		"buildTime", info.BuildTime,
		"transport", opts.transport,
	)

	// Create gRPC service (cache is internal to the service)
	unavailable := sysmlgrpc.WithheldCapabilitiesForTesting()
	var serving []sysmlgrpc.Option
	if names := splitNames(opts.serveExternal); len(names) > 0 {
		serving = append(serving, sysmlgrpc.ServeExternalEngines(names...))
	}
	var svc *sysmlgrpc.Service
	if len(unavailable) == 0 {
		svc, err = sysmlgrpc.NewService(opts.cacheSize, info.Version, serving...)
	} else {
		svc, err = sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(
			opts.cacheSize, info.Version, unavailable, serving...)
	}
	if err != nil {
		slog.Error("Invalid service configuration", "error", err)
		os.Exit(1)
	}

	// Build the standard library indexes in the background, so the first model to
	// arrive does not pay for the library and startup stays prompt.
	svc.Prewarm()

	if opts.transport == transportStdio {
		// The pipe is the session: there is no port to bind, nothing to poll
		// for readiness, and the client's exit closes stdin, which ends this.
		os.Exit(runStdio(svc))
	}
	if opts.allowInsecureToken && !token.Set() {
		fmt.Fprintln(os.Stderr, "sysml-grpc: -allow-insecure-token-transport is for a bearer token served without TLS; there is no token")
		os.Exit(2)
	}
	if opts.allowInsecureToken && opts.tlsCert != "" {
		fmt.Fprintln(os.Stderr, "sysml-grpc: -allow-insecure-token-transport is for a port without -tls-cert and -tls-key; this one serves TLS")
		os.Exit(2)
	}

	// Start gRPC server
	addr, err := guard.ListenAddress(net.JoinHostPort(opts.host, strconv.Itoa(opts.port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml-grpc: -host %s -port %d: %v\n", opts.host, opts.port, err)
		os.Exit(2)
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("Failed to listen", "addr", addr, "port", opts.port, "error", err)
		os.Exit(1)
	}
	// Refused before anything is served or reported: an address other machines
	// reach is not offered open by accident.
	if !guard.IsLoopbackAddr(lis.Addr()) {
		if !token.Set() && !opts.allowUnauthenticated {
			_ = lis.Close()
			fmt.Fprintf(os.Stderr, "sysml-grpc: %s is an address other machines reach: %v; -allow-unauthenticated serves it open\n", lis.Addr(), guard.NoToken(tokenEnv))
			os.Exit(2)
		}
		if token.Set() && opts.tlsCert == "" && !opts.allowInsecureToken {
			_ = lis.Close()
			fmt.Fprintf(os.Stderr, "sysml-grpc: %s is an address other machines reach and the bearer token would cross it in clear: serve TLS with -tls-cert and -tls-key, or -allow-insecure-token-transport behind a proxy that terminates TLS\n", lis.Addr())
			os.Exit(2)
		}
		switch {
		case opts.allowUnauthenticated:
			slog.Warn("serving an address other machines reach with no authentication; every host that opens the port may use this service", "addr", lis.Addr())
		case opts.allowInsecureToken:
			slog.Warn("the bearer token travels in clear; anyone observing this address can replay it, so only a proxy terminating TLS in front makes this safe", "addr", lis.Addr())
		}
	}

	// Start health check server, on the same host as the service.
	var healthSrv *http.Server
	if opts.healthPort != 0 {
		healthAddr, err := guard.ListenAddress(net.JoinHostPort(opts.host, strconv.Itoa(opts.healthPort)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "sysml-grpc: -host %s -health-port %d: %v\n", opts.host, opts.healthPort, err)
			os.Exit(2)
		}
		healthSrv = &http.Server{
			Addr:              healthAddr,
			Handler:           healthHandler(info.Version),
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("Health check server listening", "addr", healthSrv.Addr)
			if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("Health check server failed", "error", err)
			}
		}()
		if opts.transport == transportConnect {
			slog.Warn("the separate health listener is deprecated; /health is served on the main port and -health-port 0 disables it",
				"health_port", opts.healthPort)
		}
	}

	// The port is the kernel's under -port 0, so report the bound one before
	// serving: a client waiting on this line cannot miss the address it must dial.
	if opts.reportAddress {
		if _, err := fmt.Fprintln(os.Stdout, dialAddress(lis.Addr())); err != nil {
			slog.Error("Failed to report the listening address", "error", err)
			os.Exit(1)
		}
	}

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	if opts.exitWithParent {
		go exitWhenStdinCloses(shutdown)
	}

	if opts.transport == transportConnect {
		serveCtx, cancelServe := context.WithCancel(context.Background())
		served := make(chan error, 1)
		go func() {
			served <- serveConnect(serveCtx, lis, svc, info.Version, origins, token, opts.tlsCert, opts.tlsKey)
		}()

		select {
		case err := <-served:
			cancelServe()
			if err != nil {
				slog.Error("Connect server failed", "error", err)
				stopHealthServer(healthSrv)
				svc.Close()
				os.Exit(1)
			}
		case <-shutdown:
			slog.Info("Shutting down gracefully...")
			cancelServe()
			if err := <-served; err != nil {
				slog.Warn("Connect server shutdown error", "error", err)
			}
		}
		svc.Close()
		stopHealthServer(healthSrv)
		return
	}

	runGRPC(lis, svc, token, shutdown)
	stopHealthServer(healthSrv)
}

// authDescription says in a startup line what guards the service.
func authDescription(token guard.Token) string {
	if token.Set() {
		return "bearer token"
	}
	return "none"
}

// dialAddress renders a listening address as one a client can dial: a wildcard
// host answers on the loopback address, which is where a private child is wanted.
func dialAddress(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	switch host {
	case "", "::", "0.0.0.0":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// exitWhenStdinCloses asks for shutdown once stdin reaches end of file, which is
// what the pipe on it does when the process holding the other end dies, however
// it dies. Reading also discards anything sent, so a writer cannot block.
func exitWhenStdinCloses(shutdown chan<- os.Signal) {
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		slog.Debug("stdin read ended", "error", err)
	}
	slog.Info("stdin closed, exiting with the process that started this service")
	shutdown <- syscall.SIGTERM
}

// stopHealthServer shuts the health check server down, allowing it a moment to
// finish the requests it has in hand.
func stopHealthServer(healthSrv *http.Server) {
	if healthSrv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := healthSrv.Shutdown(ctx); err != nil {
		slog.Warn("Health check server shutdown error", "error", err)
	} else {
		slog.Info("Health check server stopped")
	}
}

// healthHandler returns an HTTP handler that responds to /health with JSON status.
func healthHandler(ver string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "sysml-grpc",
			"version": ver,
		}); err != nil {
			slog.Warn("health response write failed", "error", err)
		}
	})
	return mux
}

// splitNames reads a comma-separated list, dropping blanks.
func splitNames(list string) []string {
	var names []string
	for _, name := range strings.Split(list, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}
