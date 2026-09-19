// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package stdiorpc serves SysMLService over a pair of pipes rather than a
// socket. The header framing is the one cmd/sysml-lsp already speaks
// (Content-Length, then the body) and carries two body encodings: a JSON-RPC
// 2.0 envelope, and a raw protobuf body whose method and id travel in headers.
// It is an evaluation prototype; see
// docs/internals/design/transport-evaluation.md.
package stdiorpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Content types a frame may carry. JSON wraps the message in a JSON-RPC
// envelope; protobuf carries the same wire bytes gRPC carries as the whole
// body, so the framing is not measured as if it were the encoding.
const (
	ContentTypeJSON     = "application/json"
	ContentTypeProtobuf = "application/proto"
)

// Headers a protobuf frame uses in place of the JSON envelope's fields, and the
// header a failed protobuf call is answered under.
const (
	HeaderMethod  = "Sysml-Method"
	HeaderID      = "Sysml-Id"
	HeaderCode    = "Sysml-Status-Code"
	HeaderMessage = "Sysml-Status-Message"
)

// jsonrpcVersion is the only version this server answers.
const jsonrpcVersion = "2.0"

// maxBodyBytes bounds one frame, so a client that miscounts a header cannot
// make the server allocate without limit.
const maxBodyBytes = 128 << 20

// request is one call: the method names an RPC of SysMLService, and params
// carries its request message.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// response is one answer, carrying either a result or an error, never both.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

// responseError reports a failed call. Code is the canonical status code the
// service returned, so a client reads the same code it reads over gRPC.
type responseError struct {
	Code    uint32 `json:"code"`
	Message string `json:"message"`
}

// frame is one message read off the pipe: its headers and its body.
type frame struct {
	header      textproto.MIMEHeader
	body        []byte
	contentType string
}

// handler runs one RPC of SysMLService: it decodes the request with dec and
// answers with the response message.
type handler func(ctx context.Context, dec func(any) error) (any, error)

// Server answers calls read from one reader on one writer.
type Server struct {
	methods map[string]handler
	impl    pb.SysMLServiceServer

	// writing serializes frames, since answers are written by the goroutine
	// that handled the call and several may finish at once.
	writing sync.Mutex
	out     *bufio.Writer
}

// NewServer builds a server dispatching to impl. The method table is read from
// the generated service descriptor, so every RPC of SysMLService is served and
// none can be forgotten when one is added.
func NewServer(impl pb.SysMLServiceServer) *Server {
	methods := make(map[string]handler, len(pb.SysMLService_ServiceDesc.Methods))
	for _, m := range pb.SysMLService_ServiceDesc.Methods {
		call := m.Handler
		methods[m.MethodName] = func(ctx context.Context, dec func(any) error) (any, error) {
			return call(impl, ctx, dec, nil)
		}
	}
	return &Server{methods: methods, impl: impl}
}

// Serve reads frames from r and writes answers to w until r reaches its end.
// Calls are handled concurrently and answered as they finish, so a slow call
// does not hold up the ones behind it; a client correlates answers by id.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	s.out = bufio.NewWriter(w)
	reader := bufio.NewReader(r)

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		f, err := readFrame(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if f.contentType == ContentTypeProtobuf {
				s.handleProtobuf(ctx, f)
				return
			}
			s.handleJSON(ctx, f)
		}()
	}
}

// handleJSON answers one JSON-RPC frame. A frame that is not a call at all is
// still answered, under a null id, so a client sees the parse failure.
func (s *Server) handleJSON(ctx context.Context, f frame) {
	var req request
	if err := json.Unmarshal(f.body, &req); err != nil {
		s.writeJSON(response{
			JSONRPC: jsonrpcVersion,
			Error:   &responseError{Code: uint32(connect.CodeInvalidArgument), Message: err.Error()},
		})
		return
	}
	if req.JSONRPC != jsonrpcVersion {
		s.writeJSON(response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &responseError{
				Code:    uint32(connect.CodeInvalidArgument),
				Message: fmt.Sprintf("jsonrpc %q is not %q", req.JSONRPC, jsonrpcVersion),
			},
		})
		return
	}

	res, err := s.call(ctx, req.Method, func(msg proto.Message) error {
		if len(req.Params) == 0 {
			return nil
		}
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(req.Params, msg); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil
	})
	if err != nil {
		code, message := errorParts(err)
		s.writeJSON(response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error:   &responseError{Code: code, Message: message},
		})
		return
	}

	body, err := protojson.Marshal(res)
	if err != nil {
		s.writeJSON(response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error:   &responseError{Code: uint32(connect.CodeInternal), Message: err.Error()},
		})
		return
	}
	s.writeJSON(response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: body})
}

// handleProtobuf answers one protobuf frame, whose body is the request message
// itself and whose method and id are headers.
func (s *Server) handleProtobuf(ctx context.Context, f frame) {
	id := f.header.Get(HeaderID)
	res, err := s.call(ctx, f.header.Get(HeaderMethod), func(msg proto.Message) error {
		if err := proto.Unmarshal(f.body, msg); err != nil {
			return connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil
	})
	if err != nil {
		code, message := errorParts(err)
		s.write(nil, ContentTypeProtobuf, textproto.MIMEHeader{
			HeaderID:      {id},
			HeaderCode:    {strconv.Itoa(int(code))},
			HeaderMessage: {message},
		})
		return
	}

	body, err := proto.Marshal(res)
	if err != nil {
		s.write(nil, ContentTypeProtobuf, textproto.MIMEHeader{
			HeaderID:      {id},
			HeaderCode:    {strconv.Itoa(int(connect.CodeInternal))},
			HeaderMessage: {err.Error()},
		})
		return
	}
	s.write(body, ContentTypeProtobuf, textproto.MIMEHeader{
		HeaderID:   {id},
		HeaderCode: {"0"},
	})
}

// call runs the named RPC, decoding its request with decode, and returns the
// response message. The generated unary handler is what runs, so the prototype
// shares the service's behavior with the gRPC path rather than restating it.
func (s *Server) call(
	ctx context.Context,
	name string,
	decode func(proto.Message) error,
) (proto.Message, error) {
	method, ok := s.methods[name]
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented,
			fmt.Errorf("SysMLService has no method %q", name))
	}

	res, err := method(ctx, func(into any) error {
		msg, ok := into.(proto.Message)
		if !ok {
			return connect.NewError(connect.CodeInternal,
				fmt.Errorf("request of %s is not a protobuf message", name))
		}
		return decode(msg)
	})
	if err != nil {
		return nil, err
	}
	msg, ok := res.(proto.Message)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("response of %s is not a protobuf message", name))
	}
	return msg, nil
}

// errorParts reads the canonical status code and message a failed call is
// answered with; the codes number the same as gRPC's.
func errorParts(err error) (uint32, string) {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return uint32(ce.Code()), ce.Message()
	}
	return uint32(connect.CodeUnknown), err.Error()
}

// writeJSON frames one JSON-RPC answer.
func (s *Server) writeJSON(res response) {
	body, err := json.Marshal(res)
	if err != nil {
		body = []byte(`{"jsonrpc":"2.0","error":{"code":13,"message":"response could not be encoded"}}`)
	}
	s.write(body, ContentTypeJSON, nil)
}

// write frames one answer. Failing to write is the end of the session, so it is
// not reported to a client that can no longer read it.
func (s *Server) write(body []byte, contentType string, header textproto.MIMEHeader) {
	s.writing.Lock()
	defer s.writing.Unlock()

	fmt.Fprintf(s.out, "Content-Length: %d\r\nContent-Type: %s\r\n", len(body), contentType)
	for name, values := range header {
		for _, value := range values {
			fmt.Fprintf(s.out, "%s: %s\r\n", name, value)
		}
	}
	fmt.Fprint(s.out, "\r\n")
	_, _ = s.out.Write(body)
	_ = s.out.Flush()
}

// readFrame reads one Content-Length-delimited frame, defaulting to a JSON body
// when the frame names no content type.
func readFrame(r *bufio.Reader) (frame, error) {
	header, err := textproto.NewReader(r).ReadMIMEHeader()
	if err != nil {
		return frame{}, err
	}

	written := header.Get("Content-Length")
	if written == "" {
		return frame{}, errors.New("frame has no Content-Length")
	}
	length, err := strconv.Atoi(written)
	if err != nil || length < 0 {
		return frame{}, fmt.Errorf("frame has an unreadable Content-Length %q", written)
	}
	if length > maxBodyBytes {
		return frame{}, fmt.Errorf("frame of %d bytes exceeds the %d-byte limit", length, maxBodyBytes)
	}

	// A charset parameter is the LSP spelling; only the media type selects the
	// encoding here.
	contentType := ContentTypeJSON
	if written := header.Get("Content-Type"); written != "" {
		media, _, _ := strings.Cut(written, ";")
		contentType = strings.TrimSpace(media)
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, err
	}
	return frame{header: header, body: body, contentType: contentType}, nil
}
