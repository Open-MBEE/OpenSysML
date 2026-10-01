// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package syntax serves the purely syntactic RPCs a WebAssembly build can
// answer without a symbol table or a standard library: parsing content for its
// syntax diagnostics, reformatting it, and classifying its lexical tokens. It
// is stateless — a call carries everything it needs — and its diagnostic shape
// is the protojson Diagnostic the gRPC service answers with.
package syntax

import (
	"context"
	"encoding/json"
	"io"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/format"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/semtok"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Diagnostic is the Diagnostic message: severity, message, code and the span it
// was located at, under the protojson field names.
type Diagnostic struct {
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`
	Span     *Span  `json:"span,omitempty"`
	Code     string `json:"code,omitempty"`
}

// Span is the Span message: the file and its start and end line/column.
type Span struct {
	File      string `json:"file,omitempty"`
	StartLine int32  `json:"startLine,omitempty"`
	StartCol  int32  `json:"startCol,omitempty"`
	EndLine   int32  `json:"endLine,omitempty"`
	EndCol    int32  `json:"endCol,omitempty"`
}

// int32Clamp narrows a line or column number to the proto's int32, saturating
// rather than wrapping: a position past 2^31 would otherwise be reported as a
// negative one.
func int32Clamp(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
}

// FromDiag converts a diag.Diagnostic to the Diagnostic message.
func FromDiag(d diag.Diagnostic, sf *source.SourceFile) *Diagnostic {
	li := sf.Lines()
	start := li.PosAt(d.Span.Offset)
	end := li.PosAt(d.Span.End())

	return &Diagnostic{
		Severity: d.Severity.String(),
		Message:  d.Message,
		Code:     d.Code,
		Span: &Span{
			File:      sf.Name(),
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
		},
	}
}

// ParserDiagnostic converts a parser error to the Diagnostic message.
func ParserDiagnostic(d parser.Diagnostic, sf *source.SourceFile) *Diagnostic {
	li := sf.Lines()
	start := li.PosAt(d.Span.Offset)
	end := li.PosAt(d.Span.End())

	return &Diagnostic{
		Severity: "error", // Parser diagnostics are always errors
		Message:  d.Message,
		Code:     d.ErrorCode(),
		Span: &Span{
			File:      sf.Name(),
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
		},
	}
}

// The request and response messages of the methods sysml-syntax serves, shaped
// as the protojson of the gRPC calls they mirror: Parse is ParseFile on inline
// content, Format is a sysml→sysml Convert, and Tokens is the LSP
// SemanticTokens answer with its legend.

type parseRequest struct {
	Content  string `json:"content,omitempty"`
	Language string `json:"language,omitempty"`
}

type parseResponse struct {
	Diagnostics []*Diagnostic `json:"diagnostics,omitempty"`
}

type formatRequest struct {
	Content              string `json:"content,omitempty"`
	Language             string `json:"language,omitempty"`
	TolerateSyntaxErrors bool   `json:"tolerateSyntaxErrors,omitempty"`
}

type formatResponse struct {
	Content     string        `json:"content,omitempty"`
	Diagnostics []*Diagnostic `json:"diagnostics,omitempty"`
	Error       string        `json:"error,omitempty"`
}

type tokensRequest struct {
	Content  string `json:"content,omitempty"`
	Language string `json:"language,omitempty"`
}

type tokensResponse struct {
	Legend legend   `json:"legend"`
	Data   []uint32 `json:"data"`
}

// legend is the SemanticTokensLegend: the token types and modifiers in the
// order an encoded token indexes them by.
type legend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

// contentName names a document a request sent inline, as the gRPC calls name
// one: its diagnostics are reported against it.
const contentName = "<content>"

// Call runs the method named on params, the protojson request body, and returns
// the response body. A name the package does not serve is refused
// Unimplemented, and a request that does not decode InvalidArgument.
func Call(ctx context.Context, method string, params []byte) (result []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = jsonrpc.Errorf(jsonrpc.CodeInternal, "internal error in %s: %v", method, r)
		}
	}()
	switch method {
	case "Parse":
		var req parseRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		return parse(&req)
	case "Format":
		var req formatRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		return formatDocument(&req)
	case "Tokens":
		var req tokensRequest
		if err := jsonrpc.Decode(params, &req); err != nil {
			return nil, err
		}
		return tokens(&req)
	}
	return nil, jsonrpc.Errorf(jsonrpc.CodeUnimplemented, "%s is not served by sysml-syntax", method)
}

// Serve answers Content-Length-delimited JSON-RPC frames over r and w until r
// ends, the same transport the engine serves.
func Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	return jsonrpc.Serve(ctx, r, w, Call)
}

// kindOf reads the request's language: SysML when none is named, as inline
// content is read everywhere.
func kindOf(language string) (source.Kind, error) {
	switch language {
	case "", "sysml":
		return source.KindSysML, nil
	case "kerml":
		return source.KindKerML, nil
	default:
		return 0, jsonrpc.Errorf(jsonrpc.CodeInvalidArgument,
			"language must be sysml or kerml, got %q", language)
	}
}

// parseContent parses the request's content as a document of its language.
func parseContent(language, content string) (*source.SourceFile, *parser.Parser, error) {
	kind, err := kindOf(language)
	if err != nil {
		return nil, nil, err
	}
	file := source.NewWithKind(contentName, []byte(content), kind)
	p := parser.New(file)
	p.ParseFile()
	return file, p, nil
}

// parseDiagnostics is a parsed document's diagnostics: its syntax errors, then
// its warnings as warnings rather than as errors.
func parseDiagnostics(file *source.SourceFile, p *parser.Parser) []*Diagnostic {
	var diags []*Diagnostic
	for _, d := range p.Diagnostics {
		diags = append(diags, ParserDiagnostic(d, file))
	}
	for _, d := range parser.AsDiagnostics(nil, p.Warnings) {
		diags = append(diags, FromDiag(d, file))
	}
	return diags
}

// syntaxDiagnostics is the parsed document's syntax errors alone.
func syntaxDiagnostics(file *source.SourceFile, p *parser.Parser) []*Diagnostic {
	var diags []*Diagnostic
	for _, d := range p.Diagnostics {
		diags = append(diags, ParserDiagnostic(d, file))
	}
	return diags
}

// marshal writes the response body; a refusal to encode it is internal.
func marshal(resp any) ([]byte, error) {
	body, err := json.Marshal(resp)
	if err != nil {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInternal, "%s", err.Error())
	}
	return body, nil
}

// parse answers the syntax diagnostics of the request's content.
func parse(req *parseRequest) ([]byte, error) {
	file, p, err := parseContent(req.Language, req.Content)
	if err != nil {
		return nil, err
	}
	return marshal(&parseResponse{Diagnostics: parseDiagnostics(file, p)})
}

// formatDocument answers the request's content re-indented, as a sysml→sysml
// Convert does: a document with syntax errors is refused in the response's
// error unless the request tolerated them, and a formatter failure reports
// itself there too — neither is an RPC error.
func formatDocument(req *formatRequest) ([]byte, error) {
	file, p, err := parseContent(req.Language, req.Content)
	if err != nil {
		return nil, err
	}
	syntax := parser.SyntaxErrorOf(contentName, file, p.Diagnostics)
	if syntax != nil && !req.TolerateSyntaxErrors {
		return marshal(&formatResponse{
			Error:       syntax.Error(),
			Diagnostics: syntaxDiagnostics(file, p),
		})
	}
	out, ferr := format.Source(contentName, []byte(req.Content), format.DefaultOptions)
	if ferr != nil {
		return marshal(&formatResponse{Error: ferr.Error()})
	}
	return marshal(&formatResponse{
		Content:     string(out),
		Diagnostics: syntaxDiagnostics(file, p),
	})
}

// tokens answers the lexical semantic tokens of the request's content: the
// full legend and the LSP delta-encoded data over the lexer's keywords,
// comments and literals.
func tokens(req *tokensRequest) ([]byte, error) {
	if _, err := kindOf(req.Language); err != nil {
		return nil, err
	}
	content := []byte(req.Content)
	classes := semtok.Classes()
	types := make([]string, len(classes))
	for i, c := range classes {
		types[i] = c.String()
	}
	mods := semtok.Modifiers()
	modifiers := make([]string, len(mods))
	for i, m := range mods {
		modifiers[i] = m.String()
	}
	return marshal(&tokensResponse{
		Legend: legend{TokenTypes: types, TokenModifiers: modifiers},
		Data:   semtok.Encode(content, semtok.Lexical(content)),
	})
}
