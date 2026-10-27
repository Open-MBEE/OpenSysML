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

// access is the door every call passes: the bearer token it must carry, when
// one is configured, and the log line each call leaves.
type access struct {
	token guard.Token
}

// admit classifies a call's Authorization value: whether it is let in and was
// authenticated doing so, or why it is refused.
func (a access) admit(authorization string) (authenticated bool, refusal guard.Refusal) {
	if !a.token.Set() {
		return false, guard.Accepted
	}
	refusal = a.token.Check(authorization)
	return refusal == guard.Accepted, refusal
}

// logCall writes the one line a call leaves: who called what over which
// protocol, how it ended, how long it took, and whether it was authenticated.
func logCall(protocol, peer, method, code string, dur time.Duration, authenticated bool) {
	slog.Info("call",
		"peer", peer,
		"method", method,
		"protocol", protocol,
		"code", code,
		"duration_ms", dur.Milliseconds(),
		"authenticated", authenticated,
	)
}

// logRefusal records a call turned away at the door and who knocked; what
// they presented is never written.
func logRefusal(protocol, peer, method string, refusal guard.Refusal) {
	slog.Warn("call refused: "+refusal.String(),
		"peer", peer,
		"method", method,
		"protocol", protocol,
		"code", connect.CodeUnauthenticated.String(),
		"authenticated", false,
	)
}

// unauthenticated is the error a refused call is answered with: the code the
// wire contract reserves, carrying the RFC 6750 challenge.
func unauthenticated(refusal guard.Refusal) *connect.Error {
	err := connect.NewError(connect.CodeUnauthenticated, errors.New(refusal.Message()))
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
		authenticated, refusal := i.admit(req.Header().Get("Authorization"))
		if refusal != guard.Accepted {
			logRefusal(protocol, peer, procedure, refusal)
			return nil, unauthenticated(refusal)
		}
		res, err := next(ctx, req)
		if err == nil {
			warnLargeJSON(req, res)
		}
		logCall(protocol, peer, procedure, codeOf(err), time.Since(start), authenticated)
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
		authenticated, refusal := i.admit(conn.RequestHeader().Get("Authorization"))
		if refusal != guard.Accepted {
			logRefusal(protocol, peer, procedure, refusal)
			return unauthenticated(refusal)
		}
		err := next(ctx, conn)
		logCall(protocol, peer, procedure, codeOf(err), time.Since(start), authenticated)
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
