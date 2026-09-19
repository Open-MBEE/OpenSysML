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
	"strings"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/proto"
)

// Transports this binary can serve.
const (
	transportGRPC    = "grpc"
	transportConnect = "connect"
	transportStdio   = "stdio"
)

// connectHandler builds the routes a Connect server answers: every RPC of
// SysMLService under /sysml.SysMLService/, gRPC server reflection, and the
// health endpoint that otherwise needs a port of its own.
func connectHandler(svc *sysmlgrpc.Service, ver string, origins map[string]struct{}) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(
		sysmlgrpc.NewConnectAdapter(svc),
		connect.WithInterceptors(connectLoggingInterceptor()),
	))

	// grpcurl and grpcui ask the reflection service for the schema, which
	// connect-go serves from a package of its own rather than in the handler.
	reflector := grpcreflect.NewStaticReflector(protoconnect.SysMLServiceName)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	mux.Handle("/health", healthHandler(ver))
	if len(origins) == 0 {
		return mux
	}
	return corsMiddleware(mux, origins)
}

// serveConnect serves gRPC, gRPC-Web and the Connect protocol on one port until
// the context is cancelled. h2c carries HTTP/2 in cleartext, which is how an
// existing gRPC client reaches an address that offers no TLS.
func serveConnect(ctx context.Context, lis net.Listener, svc *sysmlgrpc.Service, ver string, origins map[string]struct{}, certFile, keyFile string) error {
	handler := connectHandler(svc, ver, origins)
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if certFile != "" {
		srv.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2", "http/1.1"},
		}
	} else {
		srv.Handler = h2c.NewHandler(handler, &http2.Server{})
	}

	errs := make(chan error, 1)
	go func() {
		if certFile != "" {
			slog.Info("Connect server listening",
				"addr", lis.Addr(), "protocols", "grpc, grpc-web, connect", "security", "TLS")
			errs <- srv.ServeTLS(lis, certFile, keyFile)
			return
		}
		slog.Info("Connect server listening",
			"addr", lis.Addr(), "protocols", "grpc, grpc-web, connect", "security", "cleartext/h2c")
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

// connectLoggingInterceptor logs each call with its procedure and duration, as
// the grpc-go path's interceptor does.
func connectLoggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			res, err := next(ctx, req)
			if err == nil && strings.Contains(strings.ToLower(req.Header().Get("Content-Type")), "json") {
				if message, ok := res.Any().(proto.Message); ok {
					size := proto.Size(message)
					if size >= 256*1024 {
						slog.Warn("large JSON response; a protobuf body is several times cheaper for large answers",
							"procedure", req.Spec().Procedure,
							"protobuf_size_bytes", size)
					}
				}
			}
			dur := time.Since(start)
			if err != nil {
				slog.Error("Connect call failed",
					"procedure", req.Spec().Procedure,
					"protocol", req.Peer().Protocol,
					"duration_ms", dur.Milliseconds(),
					"error", err,
				)
			} else {
				slog.Debug("Connect call",
					"procedure", req.Spec().Procedure,
					"protocol", req.Peer().Protocol,
					"duration_ms", dur.Milliseconds(),
				)
			}
			return res, err
		}
	}
}
