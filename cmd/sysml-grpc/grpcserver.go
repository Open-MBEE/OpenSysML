// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// The -transport grpc path: the only non-test file here that may import
// grpc-go; scripts/check-grpc-imports.sh holds that line.

package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// runGRPC serves the Service through grpc-go alone, as releases before the
// Connect default did, until shutdown asks for a graceful stop.
func runGRPC(lis net.Listener, svc *sysmlgrpc.Service, shutdown <-chan os.Signal) {
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(),
			statusInterceptor(),
		),
	)
	pb.RegisterSysMLServiceServer(grpcServer, svc)
	reflection.Register(grpcServer) // Enable server reflection for grpcurl/grpcui

	go func() {
		slog.Info("gRPC server listening", "addr", lis.Addr())
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdown
	slog.Info("Shutting down gracefully...")
	svc.Close()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		slog.Info("gRPC server stopped")
	case <-time.After(30 * time.Second):
		slog.Warn("Forcing gRPC server shutdown after timeout")
		grpcServer.Stop()
	}
}

// statusInterceptor carries the *connect.Error a handler refuses with out as
// the gRPC status of the same code, so a grpc-go client reads the code and
// message unchanged.
func statusInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		resp, err := handler(ctx, req)
		var ce *connect.Error
		if errors.As(err, &ce) {
			return resp, status.Error(codes.Code(ce.Code()), ce.Message())
		}
		return resp, err
	}
}

// loggingInterceptor logs each gRPC call with method name and duration.
func loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		if err != nil {
			slog.Error("gRPC call failed",
				"method", info.FullMethod,
				"duration_ms", dur.Milliseconds(),
				"error", err,
			)
		} else {
			slog.Debug("gRPC call",
				"method", info.FullMethod,
				"duration_ms", dur.Milliseconds(),
			)
		}
		return resp, err
	}
}
