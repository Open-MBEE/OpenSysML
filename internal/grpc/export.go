package grpc

import (
	"context"
	"errors"
	"os"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// Convert writes a model in another representation, so a client can save a model
// it read rather than only inspect it. Argument faults fail the call; a model
// the converter refuses is reported in the response's error and diagnostics.
func (s *Service) Convert(ctx context.Context, req *pb.ConvertRequest) (*pb.ConvertResponse, error) {
	if err := s.requireCapability(CapabilityConvert); err != nil {
		return nil, err
	}
	name, data, err := s.convertSource(req)
	if err != nil {
		return nil, err
	}
	from, err := convertFrom(req, name)
	if err != nil {
		return nil, err
	}
	if req.ToFormat == "" {
		return nil, statusError(connect.CodeInvalidArgument, "to_format is required: expected sysml, kerml, ttl, turtle or rdf")
	}
	to, err := export.ParseFormat(req.ToFormat)
	if err != nil {
		return nil, statusError(connect.CodeInvalidArgument, err.Error())
	}
	if !to.Writable() {
		return nil, statusError(connect.CodeInvalidArgument, (&export.NotWritableError{Format: to}).Error())
	}

	resp := &pb.ConvertResponse{FromFormat: from.String(), ToFormat: to.String()}
	// Marked on the response rather than left to the client to infer, so a caller
	// that let a format be inferred learns the mapping it got is experimental.
	if export.IsExperimental(from, to) {
		resp.Experimental = true
		resp.ExperimentalNotice = export.Notice(from, to)
	}
	out, syntax, err := convertModel(name, data, from, to, req.TolerateSyntaxErrors)
	if err != nil {
		resp.Error = err.Error()
		var broken *export.SyntaxError
		if errors.As(err, &broken) {
			resp.Diagnostics = s.filterDiagnosticCapabilities(syntaxDiagnostics(broken))
		}
		return resp, nil
	}
	resp.Content = string(out)
	resp.Diagnostics = s.filterDiagnosticCapabilities(syntaxDiagnostics(syntax))
	return resp, nil
}

// convertModel runs the conversion, tolerating unreadable notation only when the
// request asked for it.
func convertModel(name string, data []byte, from, to export.Format, tolerant bool) ([]byte, *export.SyntaxError, error) {
	if tolerant {
		return export.ConvertTolerant(name, data, from, to)
	}
	out, err := export.Convert(name, data, from, to)
	return out, nil, err
}

// convertSource reads the model the request names, and the name to report it by.
// A model_hash converts the source that parse read rather than the file as it
// stands now, so a model is written back out as the client inspected it.
func (s *Service) convertSource(req *pb.ConvertRequest) (string, []byte, error) {
	switch src := req.Source.(type) {
	case *pb.ConvertRequest_Content:
		return "<content>", []byte(src.Content), nil
	case *pb.ConvertRequest_ModelHash:
		cached, ok := s.cache.Get(src.ModelHash)
		if !ok {
			return "", nil, statusErrorf(connect.CodeNotFound,
				"model %s is no longer cached: parse it again, or convert its file_path or content",
				src.ModelHash)
		}
		// Conversion writes one document out, so a model of several is refused
		// rather than converted from one of them.
		doc, err := cached.SoleDocument()
		if err != nil {
			return "", nil, err
		}
		return doc.Source.Name(), doc.Source.Bytes(), nil
	case *pb.ConvertRequest_FilePath:
		// #nosec G304 -- reading the model file the client names is the point,
		// and the service runs with the caller's own privileges.
		data, err := os.ReadFile(src.FilePath)
		if err != nil {
			return "", nil, statusErrorf(connect.CodeNotFound, "file not found: %v", err)
		}
		return src.FilePath, data, nil
	default:
		return "", nil, statusError(connect.CodeInvalidArgument, "source must be model_hash, file_path or content")
	}
}

// convertFrom resolves the source format, inferring it from the file name when
// the request left it unset.
func convertFrom(req *pb.ConvertRequest, name string) (export.Format, error) {
	if req.FromFormat != "" {
		from, err := export.ParseFormat(req.FromFormat)
		if err != nil {
			return 0, statusError(connect.CodeInvalidArgument, err.Error())
		}
		return from, nil
	}
	if req.GetModelHash() != "" {
		// Parse reads notation, so a cached model is notation whatever it was
		// named — including one parsed from inline content, which has no name.
		return export.FormatSysML, nil
	}
	if req.GetFilePath() == "" {
		return 0, statusError(connect.CodeInvalidArgument, "from_format is required for inline content: expected "+export.FormatList)
	}
	from, err := export.FormatOfPath(name)
	if err != nil {
		return 0, statusError(connect.CodeInvalidArgument, export.Advise(err, "pass from_format, or "+export.ExtensionAdvice).Error())
	}
	return from, nil
}

// syntaxDiagnostics reports a SyntaxError as diagnostics, with spans when the
// input was notation and a bare message when it was not.
func syntaxDiagnostics(syntax *export.SyntaxError) []*pb.Diagnostic {
	if syntax == nil {
		return nil
	}
	if len(syntax.Diags) > 0 && syntax.File != nil {
		diags := make([]*pb.Diagnostic, 0, len(syntax.Diags))
		for _, diag := range syntax.Diags {
			diags = append(diags, ParserDiagnosticToProto(diag, syntax.File))
		}
		return diags
	}
	diags := make([]*pb.Diagnostic, 0, len(syntax.Messages))
	for _, message := range syntax.Messages {
		diags = append(diags, &pb.Diagnostic{
			Severity: "error",
			Message:  message,
			Code:     SyntaxDiagnosticCode,
			Span:     &pb.Span{File: syntax.Name},
		})
	}
	return diags
}
