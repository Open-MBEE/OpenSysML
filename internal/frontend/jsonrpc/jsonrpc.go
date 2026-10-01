// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package jsonrpc speaks the JSON half of stdiorpc's protocol over a pipe:
// Content-Length-delimited frames carrying JSON-RPC 2.0 bodies, with the
// canonical gRPC status codes numbering a refused call. It is shared by the
// WebAssembly commands, whose frames are read without net/textproto so the
// networking packages stay out of a small binary.
package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Content types a frame may carry, as stdiorpc reads them. Only JSON is served;
// a protobuf frame is refused rather than answered in kind.
const (
	ContentTypeJSON     = "application/json"
	ContentTypeProtobuf = "application/proto"
)

// jsonrpcVersion is the only version this server answers.
const jsonrpcVersion = "2.0"

// maxBodyBytes bounds one frame, so a client that miscounts a header cannot
// make the server allocate without limit.
const maxBodyBytes = 128 << 20

// The canonical gRPC status codes an answer is reported under, numbered as
// every transport of the service numbers them.
const (
	CodeCanceled           = 1
	CodeUnknown            = 2
	CodeInvalidArgument    = 3
	CodeDeadlineExceeded   = 4
	CodeNotFound           = 5
	CodeResourceExhausted  = 8
	CodeFailedPrecondition = 9
	CodeUnimplemented      = 12
	CodeInternal           = 13
)

// Error is a refused call: the canonical status code and its message.
type Error struct {
	Code    uint32
	Message string
}

// Error returns the status message.
func (e *Error) Error() string { return e.Message }

// Errorf is a refused call with a formatted message.
func Errorf(code uint32, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Decode reads the request body as protojson does over the lowerCamel field
// names: unknown fields are ignored, absent params are the empty request.
func Decode(params []byte, into any) *Error {
	if len(params) == 0 {
		return nil
	}
	if err := json.Unmarshal(params, into); err != nil {
		return Errorf(CodeInvalidArgument, "%s", err.Error())
	}
	return nil
}

// ErrorParts reads the canonical status code and message a failed call is
// answered with; the codes number the same as gRPC's.
func ErrorParts(err error) (uint32, string) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, e.Message
	}
	return CodeUnknown, err.Error()
}

// Envelope is the JSON-RPC answer the JS host surface returns for one call:
// the result body, or the error's code and message, under a null id.
func Envelope(body []byte, err error) string {
	if err != nil {
		code, message := ErrorParts(err)
		quoted, jerr := json.Marshal(message)
		if jerr != nil {
			quoted = []byte(`"error"`)
		}
		return fmt.Sprintf(`{"jsonrpc":"2.0","id":null,"error":{"code":%d,"message":%s}}`,
			code, quoted)
	}
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":null,"result":%s}`, body)
}

// request is one call: the method names a served method, and params carries
// its request message.
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
	body        []byte
	contentType string
}

// Handler answers one call's params body, as an engine does.
type Handler func(ctx context.Context, method string, params []byte) ([]byte, error)

// Serve reads Content-Length-delimited frames from r and writes answers to w
// until r reaches its end, speaking the JSON half of stdiorpc's protocol: the
// same framing, envelope and error codes, with a protobuf frame refused since
// this product carries no protobuf. Calls are handled sequentially and answered
// in order.
func Serve(ctx context.Context, r io.Reader, w io.Writer, call Handler) error {
	out := bufio.NewWriter(w)
	reader := bufio.NewReader(r)

	for {
		f, err := readFrame(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		handle(ctx, f, out, call)
	}
}

// handle answers one frame; a frame that is not a call at all is still
// answered, under a null id, so a client sees the failure.
func handle(ctx context.Context, f frame, out *bufio.Writer, call Handler) {
	if f.contentType == ContentTypeProtobuf {
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			Error:   &responseError{Code: CodeInvalidArgument, Message: "only application/json bodies are served"},
		})
		return
	}

	var req request
	if err := json.Unmarshal(f.body, &req); err != nil {
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			Error:   &responseError{Code: CodeInvalidArgument, Message: err.Error()},
		})
		return
	}
	if req.JSONRPC != jsonrpcVersion {
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &responseError{
				Code:    CodeInvalidArgument,
				Message: fmt.Sprintf("jsonrpc %q is not %q", req.JSONRPC, jsonrpcVersion),
			},
		})
		return
	}

	body, err := call(ctx, req.Method, req.Params)
	if err != nil {
		code, message := ErrorParts(err)
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error:   &responseError{Code: code, Message: message},
		})
		return
	}
	writeJSON(out, response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: body})
}

// writeJSON frames one JSON-RPC answer.
func writeJSON(out *bufio.Writer, res response) {
	body, err := json.Marshal(res)
	if err != nil {
		body = []byte(`{"jsonrpc":"2.0","error":{"code":13,"message":"response could not be encoded"}}`)
	}
	fmt.Fprintf(out, "Content-Length: %d\r\nContent-Type: %s\r\n\r\n", len(body), ContentTypeJSON)
	_, _ = out.Write(body)
	_ = out.Flush()
}

// readFrame reads one Content-Length-delimited frame, defaulting to a JSON body
// when the frame names no content type.
func readFrame(r *bufio.Reader) (frame, error) {
	header, err := readMIMEHeader(r)
	if err != nil {
		return frame{}, err
	}

	written := headerGet(header, "Content-Length")
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
	if written := headerGet(header, "Content-Type"); written != "" {
		media, _, _ := strings.Cut(written, ";")
		contentType = strings.TrimSpace(media)
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, err
	}
	return frame{body: body, contentType: contentType}, nil
}
