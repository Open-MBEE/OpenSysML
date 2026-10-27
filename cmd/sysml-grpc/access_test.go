// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

const testToken = "s3cret-token_0"

// guardedServer is connectServer with a bearer token every RPC must carry,
// and the log the server writes while the test runs.
func guardedServer(t *testing.T) (string, *bytes.Buffer) {
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
	token, err := guard.New(testToken)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h2c.NewHandler(connectHandler(svc, "test", nil, token), &http2.Server{}))
	t.Cleanup(srv.Close)
	return srv.URL, &logs
}

// TestGRPCRobustnessAuthConnect: with a token configured, every protocol the
// Connect server speaks refuses a call without it, with the wrong one, and
// with a malformed header, and admits the right one.
func TestGRPCRobustnessAuthConnect(t *testing.T) {
	url, logs := guardedServer(t)
	protocols := []struct {
		name    string
		client  *http.Client
		options []connect.ClientOption
	}{
		{"connect", h2cClient(), nil},
		{"grpc", h2cClient(), []connect.ClientOption{connect.WithGRPC()}},
		{"grpc-web", http.DefaultClient, []connect.ClientOption{connect.WithGRPCWeb()}},
	}
	headers := []struct {
		name, authorization, challenge string
		code                           connect.Code
	}{
		{"missing", "", `Bearer realm="sysml-grpc"`, connect.CodeUnauthenticated},
		{"wrong", "Bearer " + testToken + "x", `error="invalid_token"`, connect.CodeUnauthenticated},
		{"not-bearer", "Basic " + testToken, `error="invalid_request"`, connect.CodeUnauthenticated},
		{"malformed", "Bearer two words", `error="invalid_request"`, connect.CodeUnauthenticated},
		{"right", "Bearer " + testToken, "", 0},
	}
	for _, protocol := range protocols {
		client := protoconnect.NewSysMLServiceClient(protocol.client, url, protocol.options...)
		for _, header := range headers {
			t.Run(protocol.name+"/"+header.name, func(t *testing.T) {
				logs.Reset()
				req := connect.NewRequest(&pb.ServerInfoRequest{})
				if header.authorization != "" {
					req.Header().Set("Authorization", header.authorization)
				}
				res, err := client.GetServerInfo(callContext(t), req)
				if header.code == 0 {
					if err != nil {
						t.Fatalf("the right token was refused: %v", err)
					}
					if res.Msg.Version != "test" {
						t.Errorf("version = %q, want test", res.Msg.Version)
					}
					wantLog(t, logs.String(), "level=INFO", "peer=127.0.0.1:", "method=/sysml.SysMLService/GetServerInfo", "code=ok", "authenticated=true")
					return
				}
				var connectErr *connect.Error
				if !errors.As(err, &connectErr) || connectErr.Code() != header.code {
					t.Fatalf("error = %v, want code %s", err, header.code)
				}
				// Over gRPC the challenge rides the trailers, which connect-go's
				// client does not surface; the grpc-go test reads them.
				if got := connectErr.Meta().Get("WWW-Authenticate"); protocol.name != "grpc" && !strings.Contains(got, header.challenge) {
					t.Errorf("WWW-Authenticate = %q, want it to contain %q", got, header.challenge)
				}
				wantLog(t, logs.String(), "level=WARN", "call refused", "peer=127.0.0.1:", "method=/sysml.SysMLService/GetServerInfo", "authenticated=false")
				if strings.Contains(logs.String(), testToken) {
					t.Errorf("the log carries the token:\n%s", logs.String())
				}
			})
		}
	}
}

// wantLog fails unless one log line carries every fragment.
func wantLog(t *testing.T, logs string, fragments ...string) {
	t.Helper()
	for _, line := range strings.Split(logs, "\n") {
		found := true
		for _, f := range fragments {
			found = found && strings.Contains(line, f)
		}
		if found {
			return
		}
	}
	t.Errorf("no log line carries all of %q:\n%s", fragments, logs)
}

// TestGRPCRobustnessAuthHTTPStatus: the plain-HTTP face of a refusal is the
// 401 the wire contract reserves, with the challenge in the response header.
func TestGRPCRobustnessAuthHTTPStatus(t *testing.T) {
	url, _ := guardedServer(t)
	req, _ := http.NewRequest(http.MethodPost, url+"/sysml.SysMLService/GetServerInfo", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401: %s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), `"code":"unauthenticated"`) {
		t.Errorf("body = %s, want the unauthenticated code", body)
	}
	if got := res.Header.Get("WWW-Authenticate"); !strings.HasPrefix(got, `Bearer realm="sysml-grpc"`) {
		t.Errorf("WWW-Authenticate = %q", got)
	}
}

// TestGRPCRobustnessAuthReflectionAndHealth: the reflection stream is behind
// the same door; health answers a probe that carries nothing.
func TestGRPCRobustnessAuthReflectionAndHealth(t *testing.T) {
	url, _ := guardedServer(t)
	addr := strings.TrimPrefix(url, "http://")
	reflector := grpc_reflection_v1.NewServerReflectionClient(dialConn(t, addr))
	for _, test := range []struct {
		name  string
		token string
		code  codes.Code
	}{{"without", "", codes.Unauthenticated}, {"with", testToken, codes.OK}} {
		t.Run(test.name, func(t *testing.T) {
			ctx := callContext(t)
			if test.token != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+test.token)
			}
			stream, err := reflector.ServerReflectionInfo(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
				MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
			}); err != nil && err != io.EOF {
				t.Fatal(err)
			}
			_, err = stream.Recv()
			if status.Code(err) != test.code {
				t.Errorf("reflection %s a token: %v, want %s", test.name, err, test.code)
			}
		})
	}
	res, err := http.Get(url + "/health")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("/health without a token = %d, want 200", res.StatusCode)
	}
}

// TestGRPCRobustnessAuthGRPCTransport: under -transport grpc the grpc-go
// interceptors read the authorization metadata and answer in kind.
func TestGRPCRobustnessAuthGRPCTransport(t *testing.T) {
	s := startService(t, "-transport", "grpc", "-port", "0", "-health-port", "0", "-token", testToken)
	addr := s.address(30 * time.Second)
	client := dial(t, addr)
	for _, test := range []struct {
		name, authorization string
		code                codes.Code
	}{
		{"missing", "", codes.Unauthenticated},
		{"wrong", "Bearer nope", codes.Unauthenticated},
		{"right", "Bearer " + testToken, codes.OK},
	} {
		ctx := callContext(t)
		if test.authorization != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", test.authorization)
		}
		var trailer metadata.MD
		_, err := client.GetServerInfo(ctx, &pb.ServerInfoRequest{}, grpc.Trailer(&trailer))
		if status.Code(err) != test.code {
			t.Errorf("%s token: %v, want %s", test.name, err, test.code)
		}
		if got := strings.Join(trailer.Get("www-authenticate"), ""); (test.code != codes.OK) != strings.HasPrefix(got, `Bearer realm="sysml-grpc"`) {
			t.Errorf("%s token: www-authenticate trailer = %q", test.name, got)
		}
	}
	reflector := grpc_reflection_v1.NewServerReflectionClient(dialConn(t, addr))
	stream, err := reflector.ServerReflectionInfo(callContext(t))
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
	})
	if _, err := stream.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Errorf("the reflection stream without a token: %v, want Unauthenticated", err)
	}
	authed := metadata.AppendToOutgoingContext(callContext(t), "authorization", "Bearer "+testToken)
	if _, err := client.Evaluate(authed, &pb.EvaluateRequest{ModelHash: "no-such-model", Expression: "1"}); status.Code(err) != codes.NotFound {
		t.Errorf("an unknown model over grpc-go: %v, want NotFound", err)
	}
	time.Sleep(100 * time.Millisecond)
	logs := s.logs.String()
	wantLog(t, logs, "level=WARN", "call refused", "peer=127.0.0.1:", "method=/sysml.SysMLService/GetServerInfo", "protocol=grpc", "duration_ms=")
	wantLog(t, logs, "level=INFO", `msg=call`, "code=ok", "authenticated=true", "protocol=grpc")
	wantLog(t, logs, "level=INFO", `msg=call`, "method=/sysml.SysMLService/Evaluate", "code=not_found", "authenticated=true", "protocol=grpc")
	if strings.Contains(logs, testToken) {
		t.Errorf("the log carries the token:\n%s", logs)
	}
}

// TestGRPCRobustnessAuthStartup: the default bind is loopback, an address
// other machines reach needs a token or the explicit opt-out, and the flags
// that contradict each other or stdio are refused.
func TestGRPCRobustnessAuthStartup(t *testing.T) {
	t.Run("default binds loopback", func(t *testing.T) {
		s := startService(t, "-port", "0", "-health-port", "0")
		if addr := s.address(30 * time.Second); !strings.HasPrefix(addr, "127.0.0.1:") {
			t.Errorf("listening on %s, want loopback", addr)
		}
		if logs := s.logs.String(); !strings.Contains(logs, "addr=127.0.0.1:") {
			t.Errorf("the listening line does not name loopback:\n%s", logs)
		}
	})
	t.Run("health binds the same host", func(t *testing.T) {
		s := startService(t, "-port", "0", "-health-port", freePort(t))
		s.address(30 * time.Second)
		if logs := s.logs.String(); !strings.Contains(logs, "Health check server listening\" addr=localhost:") {
			t.Errorf("the health listener did not bind loopback:\n%s", logs)
		}
	})
	t.Run("non-loopback without a token is refused", func(t *testing.T) {
		s := startService(t, "-host", "0.0.0.0", "-port", "0", "-health-port", "0")
		if s.waitStatus(30*time.Second) != 2 {
			t.Errorf("exit status != 2:\n%s", s.logs.String())
		}
		for _, want := range []string{"other machines reach", "-token-file, -token or OPENSYSML_GRPC_TOKEN", "-allow-unauthenticated"} {
			if !strings.Contains(s.logs.String(), want) {
				t.Errorf("the refusal does not say %q:\n%s", want, s.logs.String())
			}
		}
	})
	t.Run("the opt-out serves it open", func(t *testing.T) {
		s := startService(t, "-host", "0.0.0.0", "-port", "0", "-health-port", "0", "-allow-unauthenticated")
		client := dial(t, s.address(30*time.Second))
		if _, err := client.GetServerInfo(callContext(t), &pb.ServerInfoRequest{}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(s.logs.String(), "no authentication") {
			t.Errorf("no warning about serving open:\n%s", s.logs.String())
		}
	})
	t.Run("the environment variable is a token", func(t *testing.T) {
		t.Setenv(tokenEnv, testToken)
		s := startService(t, "-host", "0.0.0.0", "-port", "0", "-health-port", "0", "-allow-insecure-token-transport")
		client := dial(t, s.address(30*time.Second))
		if _, err := client.GetServerInfo(callContext(t), &pb.ServerInfoRequest{}); status.Code(err) != codes.Unauthenticated {
			t.Errorf("a call without the token: %v, want Unauthenticated", err)
		}
		if !strings.Contains(s.logs.String(), "auth=\"bearer token\"") {
			t.Errorf("the listening line does not say a token guards it:\n%s", s.logs.String())
		}
	})
	t.Run("a token in clear on an address other machines reach is refused", func(t *testing.T) {
		s := startService(t, "-host", "0.0.0.0", "-port", "0", "-health-port", "0", "-token", testToken)
		if s.waitStatus(30*time.Second) != 2 {
			t.Errorf("exit status != 2:\n%s", s.logs.String())
		}
		for _, want := range []string{"in clear", "-tls-cert and -tls-key", "-allow-insecure-token-transport"} {
			if !strings.Contains(s.logs.String(), want) {
				t.Errorf("the refusal does not say %q:\n%s", want, s.logs.String())
			}
		}
	})
	t.Run("a token file is read", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(file, []byte(testToken+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := startService(t, "-host", "0.0.0.0", "-port", "0", "-health-port", "0", "-token-file", file, "-allow-insecure-token-transport")
		ctx := metadata.AppendToOutgoingContext(callContext(t), "authorization", "Bearer "+testToken)
		if _, err := dial(t, s.address(30*time.Second)).GetServerInfo(ctx, &pb.ServerInfoRequest{}); err != nil {
			t.Errorf("the file's token was refused: %v", err)
		}
		if !strings.Contains(s.logs.String(), "travels in clear") {
			t.Errorf("no warning that the token crosses in clear:\n%s", s.logs.String())
		}
	})
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"a token and the opt-out contradict", []string{"-token", testToken, "-allow-unauthenticated"}, "contradict each other"},
		{"both token flags", []string{"-token", testToken, "-token-file", "x"}, "give one"},
		{"a malformed token", []string{"-token", "two words"}, "RFC 6750"},
		{"stdio refuses -host", []string{"-transport", "stdio", "-host", "0.0.0.0"}, "-host is not for -transport stdio"},
		{"stdio refuses -token", []string{"-transport", "stdio", "-token", testToken}, "-token is not for -transport stdio"},
		{"stdio refuses the opt-out", []string{"-transport", "stdio", "-allow-unauthenticated"}, "-allow-unauthenticated is not for -transport stdio"},
		{"the clear-transport opt-out needs a token", []string{"-allow-insecure-token-transport"}, "there is no token"},
		{"stdio refuses the clear-transport opt-out", []string{"-transport", "stdio", "-allow-insecure-token-transport"}, "-allow-insecure-token-transport is not for -transport stdio"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := startService(t, test.args...)
			if s.waitStatus(30*time.Second) != 2 {
				t.Errorf("exit status != 2:\n%s", s.logs.String())
			}
			if !strings.Contains(s.logs.String(), test.want) {
				t.Errorf("the refusal does not say %q:\n%s", test.want, s.logs.String())
			}
		})
	}
}

// TestGRPCRobustnessAuthNoTokenOnLoopback: loopback with no token is served
// open, as before, and the log still names the peer.
func TestGRPCRobustnessAuthNoTokenOnLoopback(t *testing.T) {
	url, logs := guardedServerWithout(t)
	client := protoconnect.NewSysMLServiceClient(http.DefaultClient, url)
	if _, err := client.GetServerInfo(callContext(t), connect.NewRequest(&pb.ServerInfoRequest{})); err != nil {
		t.Fatal(err)
	}
	wantLog(t, logs.String(), "level=INFO", "peer=127.0.0.1:", "code=ok", "authenticated=false")
}

func guardedServerWithout(t *testing.T) (string, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })
	return connectServer(t), &logs
}
