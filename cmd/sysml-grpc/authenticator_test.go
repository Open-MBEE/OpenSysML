// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// scoped is an authenticator of the tokens with its prefix: it accepts the one
// named good as its principal and refuses its others for a missing scope.
type scoped struct {
	prefix, good string
	principal    guard.Principal
}

func (s scoped) Verify(_ context.Context, authorization string, _ *http.Request) (guard.Principal, guard.Refusal) {
	token, refusal := guard.Bearer(authorization)
	switch {
	case refusal != guard.Accepted:
		return guard.Principal{}, refusal
	case !strings.HasPrefix(token, s.prefix):
		return guard.Principal{}, guard.NothingPresented
	case token != s.good:
		return guard.Principal{}, guard.InsufficientScope
	}
	return s.principal, guard.Accepted
}

func (scoped) Metadata() guard.ResourceMetadata { return guard.ResourceMetadata{} }

// testChain is a chain of a key-like authenticator ahead of the static token,
// as a service with both kinds is configured.
func testChain(t *testing.T) guard.Chain {
	t.Helper()
	token, err := guard.New(testToken)
	if err != nil {
		t.Fatal(err)
	}
	alice := guard.Principal{Subject: "k1", Name: "alice", Scopes: []string{"sysml:read"}, Source: "api-key"}
	return guard.Chain{scoped{prefix: "osk_", good: "osk_good", principal: alice}, guard.NewStaticToken(token)}
}

// The command line resolves to the chain the shared flags build: one static
// token, whose principal every admitted call carries.
func TestAuthenticatorChainFromFlags(t *testing.T) {
	fs := flag.NewFlagSet(grpcCommand, flag.ContinueOnError)
	opts := registerFlags(fs)
	if err := fs.Parse([]string{"-token", testToken}); err != nil {
		t.Fatal(err)
	}
	auth, err := opts.auth.Resolve(tokenEnv)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.Configured() || len(auth.Chain()) != 1 || auth.String() != "bearer token" {
		t.Fatalf("-token resolved to %s with %d authenticators", auth, len(auth.Chain()))
	}
	ctx, refusal := access{chain: auth.Chain()}.admit(context.Background(), "Bearer "+testToken)
	if refusal != guard.Accepted {
		t.Fatalf("the token was refused: %v", refusal)
	}
	principal, ok := guard.PrincipalFrom(ctx)
	if !ok || principal.Subject != guard.StaticTokenSubject || principal.Source != guard.SourceStaticToken {
		t.Errorf("principal = %+v, %v", principal, ok)
	}
	if opts.auth.Given() != "-token" {
		t.Errorf("Given() = %q", opts.auth.Given())
	}
	if msg := stdioRefusal(&options{auth: opts.auth}); !strings.Contains(msg, "-token is not for -transport stdio") {
		t.Errorf("stdio refusal = %q", msg)
	}
}

// A Connect handler, unary or streaming, reads the principal its call was
// admitted as from its context; an open door leaves none there.
func TestAuthenticatorPrincipalInConnectHandler(t *testing.T) {
	door := connectAccess{access{chain: testChain(t)}}
	var seen guard.Principal
	var found bool
	unary := door.WrapUnary(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		seen, found = guard.PrincipalFrom(ctx)
		return connect.NewResponse(&pb.ServerInfoResponse{}), nil
	})
	for _, test := range []struct {
		name, authorization, subject, source string
	}{
		{"api key", "Bearer osk_good", "k1", "api-key"},
		{"static token", "Bearer " + testToken, guard.StaticTokenSubject, guard.SourceStaticToken},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := connect.NewRequest(&pb.ServerInfoRequest{})
			req.Header().Set("Authorization", test.authorization)
			if _, err := unary(context.Background(), req); err != nil {
				t.Fatal(err)
			}
			if !found || seen.Subject != test.subject || seen.Source != test.source {
				t.Errorf("the handler saw %+v (%v), want %s from %s", seen, found, test.subject, test.source)
			}
		})
	}
	t.Run("streaming", func(t *testing.T) {
		found = false
		stream := door.WrapStreamingHandler(func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			seen, found = guard.PrincipalFrom(ctx)
			return nil
		})
		if err := stream(context.Background(), streamConn{authorization: "Bearer osk_good"}); err != nil {
			t.Fatal(err)
		}
		if !found || seen.Subject != "k1" {
			t.Errorf("the stream handler saw %+v (%v)", seen, found)
		}
	})
	t.Run("open door", func(t *testing.T) {
		found = true
		open := connectAccess{}.WrapUnary(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			seen, found = guard.PrincipalFrom(ctx)
			return connect.NewResponse(&pb.ServerInfoResponse{}), nil
		})
		if _, err := open(context.Background(), connect.NewRequest(&pb.ServerInfoRequest{})); err != nil {
			t.Fatal(err)
		}
		if found {
			t.Errorf("an unauthenticated call carries a principal: %+v", seen)
		}
	})
}

// streamConn is the least of a streaming handler connection: the request
// header the door reads and a peer for the log.
type streamConn struct {
	connect.StreamingHandlerConn
	authorization string
}

func (c streamConn) Spec() connect.Spec { return connect.Spec{Procedure: "/test/Stream"} }
func (c streamConn) Peer() connect.Peer {
	return connect.Peer{Addr: "127.0.0.1:1", Protocol: "connect"}
}
func (c streamConn) RequestHeader() http.Header {
	h := http.Header{}
	if c.authorization != "" {
		h.Set("Authorization", c.authorization)
	}
	return h
}

// The grpc-go interceptors put the principal where a handler reads it too,
// on a unary call and on a stream.
func TestAuthenticatorPrincipalInGRPCHandler(t *testing.T) {
	door := access{chain: testChain(t)}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+testToken))
	info := &grpc.UnaryServerInfo{FullMethod: "/sysml.SysMLService/GetServerInfo"}
	var seen guard.Principal
	var found bool
	_, err := unaryAccessInterceptor(door)(ctx, "request", info, func(ctx context.Context, req interface{}) (interface{}, error) {
		seen, found = guard.PrincipalFrom(ctx)
		return "response", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found || seen.Subject != guard.StaticTokenSubject {
		t.Errorf("the unary handler saw %+v (%v)", seen, found)
	}
	found = false
	err = streamAccessInterceptor(door)(nil, serverStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "/test/Stream"}, func(srv interface{}, ss grpc.ServerStream) error {
		seen, found = guard.PrincipalFrom(ss.Context())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !found || seen.Subject != guard.StaticTokenSubject {
		t.Errorf("the stream handler saw %+v (%v)", seen, found)
	}
	refusedCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer osk_other"))
	_, err = unaryAccessInterceptor(door)(refusedCtx, "request", info, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("a refused call reached the handler")
		return nil, nil
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("a token lacking a scope over grpc-go: %v, want PermissionDenied", err)
	}
}

// serverStream is the least of a grpc-go server stream: a context.
type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s serverStream) Context() context.Context { return s.ctx }
func (s serverStream) SetTrailer(metadata.MD)   {}

// A chain of several kinds answers each credential as its own authenticator
// does: a key is accepted, one lacking a scope is permission denied (HTTP
// 403, insufficient_scope), the static token is accepted, and a stranger is
// invalid_token; the access log names the principal and never the token.
func TestAuthenticatorChainOverConnect(t *testing.T) {
	url, logs := chainedServer(t)
	client := protoconnect.NewSysMLServiceClient(http.DefaultClient, url)
	for _, test := range []struct {
		name, authorization, challenge, subject, source string
		code                                            connect.Code
	}{
		{"api key", "Bearer osk_good", "", "k1", "api-key", 0},
		{"static token", "Bearer " + testToken, "", guard.StaticTokenSubject, guard.SourceStaticToken, 0},
		{"key lacking a scope", "Bearer osk_other", `error="insufficient_scope"`, "", "", connect.CodePermissionDenied},
		{"stranger", "Bearer stranger", `error="invalid_token"`, "", "", connect.CodeUnauthenticated},
		{"nothing", "", `Bearer realm="sysml-grpc"`, "", "", connect.CodeUnauthenticated},
	} {
		t.Run(test.name, func(t *testing.T) {
			logs.Reset()
			req := connect.NewRequest(&pb.ServerInfoRequest{})
			if test.authorization != "" {
				req.Header().Set("Authorization", test.authorization)
			}
			_, err := client.GetServerInfo(callContext(t), req)
			if test.code == 0 {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				wantLog(t, logs.String(), "level=INFO", "code=ok", "authenticated=true", "subject="+test.subject, "source="+test.source)
				return
			}
			var connectErr *connect.Error
			if !errors.As(err, &connectErr) || connectErr.Code() != test.code {
				t.Fatalf("error = %v, want code %s", err, test.code)
			}
			if got := connectErr.Meta().Get("WWW-Authenticate"); !strings.Contains(got, test.challenge) {
				t.Errorf("WWW-Authenticate = %q, want it to contain %q", got, test.challenge)
			}
			wantLog(t, logs.String(), "level=WARN", "call refused", "code="+test.code.String(), "authenticated=false")
			if strings.Contains(logs.String(), testToken) || strings.Contains(logs.String(), "osk_") {
				t.Errorf("the log carries a credential:\n%s", logs.String())
			}
		})
	}
	req, _ := http.NewRequest(http.MethodPost, url+"/sysml.SysMLService/GetServerInfo", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer osk_other")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusForbidden || !strings.Contains(string(body), `"code":"permission_denied"`) {
		t.Errorf("a key lacking a scope over HTTP: %d %s, want 403 permission_denied", res.StatusCode, body)
	}
}

// chainedServer is guardedServer behind testChain.
func chainedServer(t *testing.T) (string, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })
	svc, err := sysmlgrpc.NewService(4, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)
	srv := httptest.NewServer(connectHandler(svc, "test", nil, testChain(t)))
	t.Cleanup(srv.Close)
	return srv.URL, &logs
}
