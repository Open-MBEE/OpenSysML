// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package engine

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

// Serve reads Content-Length-delimited frames from r and writes answers to w
// until r reaches its end, speaking the JSON half of stdiorpc's protocol: the
// same framing, envelope and error codes, with a protobuf frame refused since
// this product carries no protobuf. Calls are handled sequentially and answered
// in order.
func (e *Engine) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
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
		e.handle(ctx, f, out)
	}
}

// handle answers one frame; a frame that is not a call at all is still
// answered, under a null id, so a client sees the failure.
func (e *Engine) handle(ctx context.Context, f frame, out *bufio.Writer) {
	if f.contentType == ContentTypeProtobuf {
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			Error:   &responseError{Code: codeInvalidArgument, Message: "only application/json bodies are served"},
		})
		return
	}

	var req request
	if err := json.Unmarshal(f.body, &req); err != nil {
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			Error:   &responseError{Code: codeInvalidArgument, Message: err.Error()},
		})
		return
	}
	if req.JSONRPC != jsonrpcVersion {
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error: &responseError{
				Code:    codeInvalidArgument,
				Message: fmt.Sprintf("jsonrpc %q is not %q", req.JSONRPC, jsonrpcVersion),
			},
		})
		return
	}

	body, err := e.Call(ctx, req.Method, req.Params)
	if err != nil {
		code, message := errorParts(err)
		writeJSON(out, response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error:   &responseError{Code: code, Message: message},
		})
		return
	}
	writeJSON(out, response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: body})
}

// errorParts reads the canonical status code and message a failed call is
// answered with; the codes number the same as gRPC's.
func errorParts(err error) (uint32, string) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, e.Message
	}
	return codeUnknown, err.Error()
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
	return frame{body: body, contentType: contentType}, nil
}
