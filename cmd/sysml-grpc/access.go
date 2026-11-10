// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"google.golang.org/protobuf/proto"
)

// tokenEnv is the environment variable the bearer token is read from when no
// flag names one.
const tokenEnv = "OPENSYSML_GRPC_TOKEN" // #nosec G101 -- a variable name, not a credential

// realm names this service in a WWW-Authenticate challenge.
const realm = "sysml-grpc"

// access is the door every call passes: the authenticators a call's credential
// must satisfy, when any is configured, and the log line each call leaves.
type access struct {
	chain guard.Chain
}

// admit classifies a call's Authorization value: the context to serve it in,
// carrying the principal it was authenticated as, or why it is refused.
func (a access) admit(ctx context.Context, authorization string) (context.Context, guard.Refusal) {
	if len(a.chain) == 0 {
		return ctx, guard.Accepted
	}
	principal, refusal := a.chain.Verify(ctx, authorization, nil)
	if refusal != guard.Accepted {
		return ctx, refusal
	}
	return guard.WithPrincipal(ctx, principal), guard.Accepted
}

// logCall writes the one line a call leaves: who called what over which
// protocol, how it ended, how long it took, and who it was authenticated as.
func logCall(ctx context.Context, protocol, peer, method, code string, dur time.Duration) {
	attrs := []any{
		"peer", peer,
		"method", method,
		"protocol", protocol,
		"code", code,
		"duration_ms", dur.Milliseconds(),
	}
	if principal, ok := guard.PrincipalFrom(ctx); ok {
		attrs = append(attrs, "authenticated", true, "subject", principal.Subject, "source", principal.Source)
	} else {
		attrs = append(attrs, "authenticated", false)
	}
	slog.Info("call", attrs...)
}

// logRefusal records a call turned away at the door and who knocked; what
// they presented is never written.
func logRefusal(protocol, peer, method string, refusal guard.Refusal, dur time.Duration) {
	slog.Warn("call refused: "+refusal.String(),
		"peer", peer,
		"method", method,
		"protocol", protocol,
		"code", codeFor(refusal).String(),
		"duration_ms", dur.Milliseconds(),
		"authenticated", false,
	)
}

// codeFor is the code a refusal is answered with: permission denied for a
// credential lacking a scope (RFC 6750 § 3.1, 403), unauthenticated otherwise.
func codeFor(refusal guard.Refusal) connect.Code {
	if refusal == guard.InsufficientScope {
		return connect.CodePermissionDenied
	}
	return connect.CodeUnauthenticated
}

// refused is the error a refused call is answered with: the code the wire
// contract reserves for it, carrying the RFC 6750 challenge.
func refused(refusal guard.Refusal) *connect.Error {
	err := connect.NewError(codeFor(refusal), errors.New(refusal.Message()))
	err.Meta().Set("WWW-Authenticate", refusal.Challenge(realm))
	return err
}

// codeOf names how a call ended: ok, or the code of its error.
func codeOf(err error) string {
	if err == nil {
		return "ok"
	}
	return connect.CodeOf(err).String()
}

// connectAccess is the access policy as a Connect interceptor, on unary calls
// and on the streams reflection answers alike.
type connectAccess struct{ access }

func (i connectAccess) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		peer, procedure, protocol := req.Peer().Addr, req.Spec().Procedure, req.Peer().Protocol
		ctx, refusal := i.admit(ctx, req.Header().Get("Authorization"))
		if refusal != guard.Accepted {
			logRefusal(protocol, peer, procedure, refusal, time.Since(start))
			return nil, refused(refusal)
		}
		res, err := next(ctx, req)
		if err == nil {
			warnLargeJSON(req, res)
		}
		logCall(ctx, protocol, peer, procedure, codeOf(err), time.Since(start))
		return res, err
	}
}

func (i connectAccess) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i connectAccess) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		peer, procedure, protocol := conn.Peer().Addr, conn.Spec().Procedure, conn.Peer().Protocol
		ctx, refusal := i.admit(ctx, conn.RequestHeader().Get("Authorization"))
		if refusal != guard.Accepted {
			logRefusal(protocol, peer, procedure, refusal, time.Since(start))
			return refused(refusal)
		}
		err := next(ctx, conn)
		logCall(ctx, protocol, peer, procedure, codeOf(err), time.Since(start))
		return err
	}
}

// warnLargeJSON notes a JSON answer large enough that protobuf would have
// been several times cheaper.
func warnLargeJSON(req connect.AnyRequest, res connect.AnyResponse) {
	if !strings.Contains(strings.ToLower(req.Header().Get("Content-Type")), "json") {
		return
	}
	message, ok := res.Any().(proto.Message)
	if !ok {
		return
	}
	if size := proto.Size(message); size >= 256*1024 {
		slog.Warn("large JSON response; a protobuf body is several times cheaper for large answers",
			"procedure", req.Spec().Procedure,
			"protobuf_size_bytes", size)
	}
}
