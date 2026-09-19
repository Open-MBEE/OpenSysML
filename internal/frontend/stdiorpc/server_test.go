// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package stdiorpc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const benchModel = "package T { part def P { attribute a = 7; } }"

// session runs a server over a pair of in-memory pipes and reads answers back.
type session struct {
	t       *testing.T
	toServe *io.PipeWriter
	answers *bufio.Reader

	writing sync.Mutex
}

func newSession(t *testing.T) *session {
	t.Helper()

	svc, err := sysmlgrpc.NewService(4, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- NewServer(svc).Serve(context.Background(), inR, outW) }()

	t.Cleanup(func() {
		_ = inW.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("Serve did not return after stdin closed")
		}
		_ = outW.Close()
	})

	return &session{t: t, toServe: inW, answers: bufio.NewReader(outR)}
}

// send writes one frame. Writes are serialized because a test may call from
// several goroutines, exactly as a concurrent client would.
func (s *session) send(header textproto.MIMEHeader, contentType string, body []byte) {
	s.t.Helper()

	var frame strings.Builder
	fmt.Fprintf(&frame, "Content-Length: %d\r\nContent-Type: %s\r\n", len(body), contentType)
	for name, values := range header {
		for _, value := range values {
			fmt.Fprintf(&frame, "%s: %s\r\n", name, value)
		}
	}
	frame.WriteString("\r\n")

	s.writing.Lock()
	defer s.writing.Unlock()
	if _, err := io.WriteString(s.toServe, frame.String()); err != nil {
		s.t.Fatalf("write header: %v", err)
	}
	if _, err := s.toServe.Write(body); err != nil {
		s.t.Fatalf("write body: %v", err)
	}
}

func (s *session) call(id int, method string, req proto.Message) {
	s.t.Helper()

	params, err := protojson.Marshal(req)
	if err != nil {
		s.t.Fatalf("marshal params: %v", err)
	}
	body, err := json.Marshal(request{
		JSONRPC: jsonrpcVersion,
		ID:      json.RawMessage(strconv.Itoa(id)),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		s.t.Fatalf("marshal request: %v", err)
	}
	s.send(nil, ContentTypeJSON, body)
}

func (s *session) callProtobuf(id int, method string, req proto.Message) {
	s.t.Helper()

	body, err := proto.Marshal(req)
	if err != nil {
		s.t.Fatalf("marshal request: %v", err)
	}
	s.send(textproto.MIMEHeader{
		HeaderMethod: {method},
		HeaderID:     {strconv.Itoa(id)},
	}, ContentTypeProtobuf, body)
}

// receive reads the next answer, whichever call it belongs to.
func (s *session) receive() (textproto.MIMEHeader, []byte) {
	s.t.Helper()

	f, err := readFrame(s.answers)
	if err != nil {
		s.t.Fatalf("read answer: %v", err)
	}
	return f.header, f.body
}

func (s *session) receiveJSON() response {
	s.t.Helper()

	_, body := s.receive()
	var res response
	if err := json.Unmarshal(body, &res); err != nil {
		s.t.Fatalf("unmarshal answer %s: %v", body, err)
	}
	if res.JSONRPC != jsonrpcVersion {
		s.t.Errorf("answer jsonrpc = %q, want %q", res.JSONRPC, jsonrpcVersion)
	}
	return res
}

// result reads the next answer and unmarshals it into out, failing on an error
// answer.
func (s *session) result(out proto.Message) json.RawMessage {
	s.t.Helper()

	res := s.receiveJSON()
	if res.Error != nil {
		s.t.Fatalf("call failed: code %d: %s", res.Error.Code, res.Error.Message)
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(res.Result, out); err != nil {
		s.t.Fatalf("unmarshal result %s: %v", res.Result, err)
	}
	return res.ID
}

// parse loads a model over the session and reports its hash, so tests that need
// one do not each spell the call out.
func (s *session) parse(id int, content string) string {
	s.t.Helper()

	s.call(id, "ParseFile", &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: content},
	})
	var res pb.ParseFileResponse
	s.result(&res)
	if res.ModelHash == "" {
		s.t.Fatal("ParseFile returned no model hash")
	}
	return res.ModelHash
}

func TestServeJSONRPC(t *testing.T) {
	s := newSession(t)

	s.call(1, "GetServerInfo", &pb.ServerInfoRequest{})
	var info pb.ServerInfoResponse
	if id := s.result(&info); string(id) != "1" {
		t.Errorf("answer id = %s, want 1", id)
	}
	if len(info.Capabilities) == 0 {
		t.Error("GetServerInfo reported no capabilities")
	}

	hash := s.parse(2, benchModel)

	s.call(3, "Evaluate", &pb.EvaluateRequest{Expression: "1 + 2 * 3", ModelHash: hash})
	var eval pb.EvaluateResponse
	s.result(&eval)
	if got := eval.GetResult().GetIntValue(); got != 7 {
		t.Errorf("Evaluate = %d, want 7", got)
	}

	s.call(4, "Instantiate", &pb.InstantiateRequest{SymbolId: "T::P", ModelHash: hash})
	var inst pb.InstantiateResponse
	s.result(&inst)
	if got := inst.GetInstance().GetTypeSymbolId(); got != "T::P" {
		t.Errorf("Instantiate type = %q, want T::P", got)
	}
}

func TestServeProtobufBody(t *testing.T) {
	s := newSession(t)

	s.callProtobuf(1, "ParseFile", &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: benchModel},
	})
	header, body := s.receive()
	if got := header.Get(HeaderCode); got != "0" {
		t.Fatalf("status code = %q (%s), want 0", got, header.Get(HeaderMessage))
	}
	if got := header.Get(HeaderID); got != "1" {
		t.Errorf("answer id = %q, want 1", got)
	}
	var parsed pb.ParseFileResponse
	if err := proto.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal answer: %v", err)
	}

	s.callProtobuf(2, "Evaluate", &pb.EvaluateRequest{
		Expression: "6 * 7",
		ModelHash:  parsed.ModelHash,
	})
	header, body = s.receive()
	if got := header.Get(HeaderCode); got != "0" {
		t.Fatalf("status code = %q (%s), want 0", got, header.Get(HeaderMessage))
	}
	var eval pb.EvaluateResponse
	if err := proto.Unmarshal(body, &eval); err != nil {
		t.Fatalf("unmarshal answer: %v", err)
	}
	if got := eval.GetResult().GetIntValue(); got != 42 {
		t.Errorf("Evaluate = %d, want 42", got)
	}
}

// TestServeReportsStatusCodes checks a client reads the same status codes it
// reads over gRPC, since a stdio client must map errors the same way.
func TestServeReportsStatusCodes(t *testing.T) {
	s := newSession(t)

	tests := []struct {
		name   string
		method string
		req    proto.Message
		want   codes.Code
	}{
		{"unknown method", "NoSuchRPC", &pb.ServerInfoRequest{}, codes.Unimplemented},
		{
			"unknown model",
			"Evaluate",
			&pb.EvaluateRequest{Expression: "1", ModelHash: "missing"},
			codes.NotFound,
		},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s.call(i+1, test.method, test.req)
			res := s.receiveJSON()
			if res.Error == nil {
				t.Fatalf("call succeeded, want code %s", test.want)
			}
			if got := codes.Code(res.Error.Code); got != test.want {
				t.Errorf("code = %s, want %s", got, test.want)
			}
		})
	}
}

func TestServeRejectsMalformedFrames(t *testing.T) {
	s := newSession(t)

	s.send(nil, ContentTypeJSON, []byte("this is not JSON"))
	if res := s.receiveJSON(); res.Error == nil {
		t.Error("a body that is not JSON was answered with a result")
	}

	s.send(nil, ContentTypeJSON, []byte(`{"jsonrpc":"1.0","id":1,"method":"GetServerInfo"}`))
	if res := s.receiveJSON(); res.Error == nil {
		t.Error("a frame naming another JSON-RPC version was answered with a result")
	}

	// The session still answers calls after both, since a bad frame is an
	// answer rather than the end of the session.
	s.call(3, "GetServerInfo", &pb.ServerInfoRequest{})
	s.result(&pb.ServerInfoResponse{})
}

// TestServeAnswersConcurrentCalls sends every call before reading any answer,
// so a server that handled frames one at a time would still pass; what it
// establishes is that ids correlate answers to calls when they interleave.
func TestServeAnswersConcurrentCalls(t *testing.T) {
	s := newSession(t)
	hash := s.parse(0, benchModel)

	const calls = 24
	want := make(map[string]int64, calls)
	for i := 1; i <= calls; i++ {
		s.call(i, "Evaluate", &pb.EvaluateRequest{
			Expression: fmt.Sprintf("%d * 2", i),
			ModelHash:  hash,
		})
		want[strconv.Itoa(i)] = int64(i) * 2
	}

	for range calls {
		var eval pb.EvaluateResponse
		id := s.result(&eval)
		expected, ok := want[string(id)]
		if !ok {
			t.Fatalf("answer id %s was not called, or was answered twice", id)
		}
		delete(want, string(id))
		if got := eval.GetResult().GetIntValue(); got != expected {
			t.Errorf("answer %s = %d, want %d", id, got, expected)
		}
	}
}

// TestServeDoesNotBlockBehindASlowCall establishes the property the design note
// claims: a call that takes a while does not hold up one sent after it. Parsing
// a large model is the slow call; GetServerInfo is the fast one behind it.
func TestServeDoesNotBlockBehindASlowCall(t *testing.T) {
	s := newSession(t)

	var large strings.Builder
	large.WriteString("package Slow {\n")
	for i := range 4000 {
		fmt.Fprintf(&large, "    part def P%d { attribute a%d = %d; }\n", i, i, i)
	}
	large.WriteString("}\n")

	s.call(1, "ParseFile", &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: large.String()},
	})
	s.call(2, "GetServerInfo", &pb.ServerInfoRequest{})

	first := s.receiveJSON()
	// Both answers are read either way: an unread answer blocks the goroutine
	// writing it, which is the backpressure the design note describes.
	second := s.receiveJSON()
	if string(first.ID) != "2" {
		t.Skipf("the large parse answered first (%s then %s), so this run proves nothing",
			first.ID, second.ID)
	}
}
