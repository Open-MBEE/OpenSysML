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
	"strings"
	"time"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// runGRPC serves the Service through grpc-go alone, as releases before the
// Connect default did, until shutdown asks for a graceful stop.
func runGRPC(lis net.Listener, svc *sysmlgrpc.Service, token guard.Token, shutdown <-chan os.Signal) {
	door := access{token: token}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			unaryAccessInterceptor(door),
			statusInterceptor(),
		),
		grpc.StreamInterceptor(streamAccessInterceptor(door)),
	)
	pb.RegisterSysMLServiceServer(grpcServer, svc)
	reflection.Register(grpcServer) // Enable server reflection for grpcurl/grpcui

	go func() {
		slog.Info("gRPC server listening", "addr", lis.Addr(), "auth", authDescription(token))
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

// unaryAccessInterceptor admits or refuses each unary call by its authorization
// metadata and logs it with its peer, as the Connect path does.
func unaryAccessInterceptor(door access) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		authenticated, refusal := door.admit(authorizationOf(ctx))
		if refusal != guard.Accepted {
			logRefusal(transportGRPC, peerOf(ctx), info.FullMethod, refusal, time.Since(start))
			_ = grpc.SetTrailer(ctx, challengeTrailer(refusal))
			return nil, status.Error(codes.Unauthenticated, refusal.Message())
		}
		resp, err := handler(ctx, req)
		logCall(transportGRPC, peerOf(ctx), info.FullMethod, grpcCodeOf(err), time.Since(start), authenticated)
		return resp, err
	}
}

// streamAccessInterceptor is unaryAccessInterceptor for streams, which carry
// the reflection service; this service's own RPCs are all unary.
func streamAccessInterceptor(door access) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		ctx := ss.Context()
		authenticated, refusal := door.admit(authorizationOf(ctx))
		if refusal != guard.Accepted {
			logRefusal(transportGRPC, peerOf(ctx), info.FullMethod, refusal, time.Since(start))
			ss.SetTrailer(challengeTrailer(refusal))
			return status.Error(codes.Unauthenticated, refusal.Message())
		}
		err := handler(srv, ss)
		logCall(transportGRPC, peerOf(ctx), info.FullMethod, grpcCodeOf(err), time.Since(start), authenticated)
		return err
	}
}

// authorizationOf is the call's authorization metadata, which gRPC carries in
// lower case; several values are one header the client repeated, refused as one.
func authorizationOf(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return strings.Join(md.Get("authorization"), ", ")
}

// peerOf is the caller's address, or unknown for a context without one.
func peerOf(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return "unknown"
}

// challengeTrailer carries the RFC 6750 challenge to a grpc-go client.
func challengeTrailer(refusal guard.Refusal) metadata.MD {
	return metadata.Pairs("www-authenticate", refusal.Challenge(realm))
}

// grpcCodeOf names how a call ended in the words the Connect path uses.
func grpcCodeOf(err error) string {
	if err == nil {
		return "ok"
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code().String()
	}
	return connect.Code(status.Code(err)).String()
}
