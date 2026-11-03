// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// Transports this binary can serve.
const (
	transportGRPC    = "grpc"
	transportConnect = "connect"
	transportStdio   = "stdio"
)

// connectHandler builds the routes a Connect server answers: every RPC of
// SysMLService under /sysml.SysMLService/, gRPC server reflection, and the
// health endpoint that otherwise needs a port of its own. Every RPC passes the
// access policy; health alone is open to probes.
func connectHandler(svc *sysmlgrpc.Service, ver string, origins map[string]struct{}, token guard.Token) http.Handler {
	door := connect.WithInterceptors(connectAccess{access{token: token}})
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc), door))

	// grpcurl and grpcui ask the reflection service for the schema, which
	// connect-go serves from a package of its own rather than in the handler.
	reflector := grpcreflect.NewStaticReflector(protoconnect.SysMLServiceName)
	mux.Handle(grpcreflect.NewHandlerV1(reflector, door))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector, door))

	// Health answers probes, which carry no token; it names the build and no more.
	mux.Handle("/health", healthHandler(ver))
	if len(origins) == 0 {
		return mux
	}
	return corsMiddleware(mux, origins)
}

// certificateFiles names the PEM certificate and private key a server serves
// TLS with; with no certificate the server speaks cleartext.
type certificateFiles struct{ cert, key string }

func (c certificateFiles) set() bool { return c.cert != "" }

// serveConnect serves gRPC, gRPC-Web and the Connect protocol on one port until
// the context is cancelled. h2c carries HTTP/2 in cleartext, which is how an
// existing gRPC client reaches an address that offers no TLS.
func serveConnect(ctx context.Context, lis net.Listener, svc *sysmlgrpc.Service, ver string, origins map[string]struct{}, token guard.Token, certs certificateFiles) error {
	handler := connectHandler(svc, ver, origins, token)
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if certs.set() {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2", "http/1.1"},
		}
	} else {
		srv.Protocols = new(http.Protocols)
		srv.Protocols.SetHTTP1(true)
		srv.Protocols.SetUnencryptedHTTP2(true)
	}

	errs := make(chan error, 1)
	go func() {
		if certs.set() {
			slog.Info("Connect server listening",
				"addr", lis.Addr(), "protocols", "grpc, grpc-web, connect", "security", "TLS", "auth", authDescription(token))
			errs <- srv.ServeTLS(lis, certs.cert, certs.key)
			return
		}
		slog.Info("Connect server listening",
			"addr", lis.Addr(), "protocols", "grpc, grpc-web, connect", "security", "cleartext/h2c", "auth", authDescription(token))
		errs <- srv.Serve(lis)
	}()

	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	// ctx is already cancelled here, so shutdown runs on a copy that keeps its
	// values but not its cancellation.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("connect server shutdown: %w", err)
	}
	slog.Info("Connect server stopped")
	return nil
}
