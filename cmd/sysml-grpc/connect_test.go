// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const connectTestModel = "package T { part def P { attribute a = 7; } }"

// connectServer starts the Connect handler over cleartext HTTP, which is how the
// prototype serves a deployment that offers no TLS.
func connectServer(t *testing.T) string {
	t.Helper()

	svc, err := sysmlgrpc.NewService(4, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	srv := httptest.NewServer(h2c.NewHandler(connectHandler(svc, "test", nil), &http2.Server{}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// h2cClient dials cleartext HTTP/2, which the gRPC and full Connect protocols
// need and which Go's default transport will not do.
func h2cClient() *http.Client {
	return &http.Client{Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}}
}

// TestConnectServesEveryProtocol establishes the claim the evaluation rests on:
// one handler answers the Connect protocol, gRPC and gRPC-Web.
func TestConnectServesEveryProtocol(t *testing.T) {
	url := connectServer(t)

	tests := []struct {
		name    string
		client  *http.Client
		options []connect.ClientOption
	}{
		{"connect protocol, protobuf body", http.DefaultClient, nil},
		{"connect protocol, JSON body", http.DefaultClient,
			[]connect.ClientOption{connect.WithProtoJSON()}},
		{"grpc", h2cClient(), []connect.ClientOption{connect.WithGRPC()}},
		{"grpc-web", http.DefaultClient, []connect.ClientOption{connect.WithGRPCWeb()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := protoconnect.NewSysMLServiceClient(test.client, url, test.options...)

			parsed, err := client.ParseFile(context.Background(), connect.NewRequest(
				&pb.ParseFileRequest{
					Source: &pb.ParseFileRequest_Content{Content: connectTestModel},
				}))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}

			evaluated, err := client.Evaluate(context.Background(), connect.NewRequest(
				&pb.EvaluateRequest{
					Expression: "1 + 2 * 3",
					ModelHash:  parsed.Msg.ModelHash,
				}))
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got := evaluated.Msg.GetResult().GetIntValue(); got != 7 {
				t.Errorf("Evaluate = %d, want 7", got)
			}
		})
	}
}

// TestConnectAnswersAPlainPOST is the curl case: a client with no generated
// code, no gRPC library and no HTTP/2 posts JSON and reads JSON.
func TestConnectAnswersAPlainPOST(t *testing.T) {
	url := connectServer(t)

	post := func(procedure, body string) map[string]any {
		t.Helper()

		res, err := http.Post(url+"/sysml.SysMLService/"+procedure,
			"application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", procedure, err)
		}
		defer res.Body.Close()

		if res.ProtoMajor != 1 {
			t.Errorf("%s answered over HTTP/%d, want HTTP/1.1", procedure, res.ProtoMajor)
		}
		var answer map[string]any
		if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
			t.Fatalf("decode %s answer: %v", procedure, err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: HTTP %d: %v", procedure, res.StatusCode, answer)
		}
		return answer
	}

	parsed := post("ParseFile", `{"content":"`+connectTestModel+`"}`)
	hash, ok := parsed["modelHash"].(string)
	if !ok || hash == "" {
		t.Fatalf("ParseFile reported no modelHash: %v", parsed)
	}

	evaluated := post("Evaluate", `{"expression":"1 + 2 * 3","modelHash":"`+hash+`"}`)
	// A JSON body spells an int64 as a string, which a client reading the
	// answer by hand has to know.
	result, _ := evaluated["result"].(map[string]any)
	if got := result["intValue"]; got != "7" {
		t.Errorf("Evaluate = %v, want \"7\"", got)
	}
}

// TestConnectServesGRPCClientsUnchanged checks the generated grpc-go stub — the
// same code path the Python client and grpcurl use — against the Connect
// handler, since migration risk turns on that working with no client change.
func TestConnectServesGRPCClientsUnchanged(t *testing.T) {
	url := strings.TrimPrefix(connectServer(t), "http://")

	conn, err := grpc.Dial(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := pb.NewSysMLServiceClient(conn)

	parsed, err := client.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: connectTestModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	instantiated, err := client.Instantiate(context.Background(), &pb.InstantiateRequest{
		ModelHash: parsed.ModelHash,
		SymbolId:  "T::P",
	})
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if got := instantiated.GetInstance().GetTypeSymbolId(); got != "T::P" {
		t.Errorf("Instantiate type = %q, want T::P", got)
	}
}

// TestConnectReportsStatusCodes checks a Connect client reads the same codes a
// gRPC client reads, since the clients share their error handling.
func TestConnectReportsStatusCodes(t *testing.T) {
	client := protoconnect.NewSysMLServiceClient(http.DefaultClient, connectServer(t))

	_, err := client.Evaluate(context.Background(), connect.NewRequest(&pb.EvaluateRequest{
		Expression: "1 + 1",
		ModelHash:  "missing",
	}))
	if err == nil {
		t.Fatal("evaluating against an unknown model succeeded")
	}
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Errorf("code = %s, want %s", got, connect.CodeNotFound)
	}
}

func TestConnectRejectsGRPCWebText(t *testing.T) {
	url := connectServer(t)
	request, err := http.NewRequest(http.MethodPost, url+"/sysml.SysMLService/GetServerInfo", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/grpc-web-text")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", response.StatusCode)
	}
}

func TestServeConnectTLS(t *testing.T) {
	svc, err := sysmlgrpc.NewService(4, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)
	certFile, keyFile := writeTestCertificate(t)
	certificate, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certificate) {
		t.Fatal("append test certificate")
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	served := make(chan error, 1)
	go func() {
		served <- serveConnect(ctx, lis, svc, "test", nil, certFile, keyFile)
	}()

	address := lis.Addr().String()
	httpClient := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "127.0.0.1", RootCAs: roots},
	}}
	var response *http.Response
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		request, requestErr := http.NewRequest(http.MethodGet, "https://"+address+"/health", nil)
		if requestErr == nil {
			response, err = httpClient.Do(request)
			if err == nil {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || response == nil {
		t.Fatalf("TLS health request: %v", err)
	}
	response.Body.Close()
	if response.ProtoMajor != 1 {
		t.Errorf("TLS JSON client used HTTP/%d, want HTTP/1.1", response.ProtoMajor)
	}

	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: "127.0.0.1",
		RootCAs:    roots,
	})))
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := pb.NewSysMLServiceClient(conn)
	if _, err := client.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: connectTestModel},
	}); err != nil {
		t.Fatalf("TLS ParseFile: %v", err)
	}
	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serveConnect: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveConnect did not stop")
	}
}

func writeTestCertificate(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	dir := t.TempDir()
	certFile := dir + "/cert.pem"
	keyFile := dir + "/key.pem"
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}
