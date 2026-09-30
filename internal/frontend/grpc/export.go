package grpc

import (
	"context"
	"errors"
	"os"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// Convert writes a model in another representation, so a client can save a model
// it read rather than only inspect it. Argument faults fail the call; a model
// the converter refuses is reported in the response's error and diagnostics.
func (s *Service) Convert(ctx context.Context, req *pb.ConvertRequest) (*pb.ConvertResponse, error) {
	if err := s.requireCapability(CapabilityConvert); err != nil {
		return nil, err
	}
	if out, done, err := s.convertModelOfDocuments(req); done {
		return out, err
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
		return nil, statusError(connect.CodeInvalidArgument, "to_format is required: expected "+convert.FormatList)
	}
	to, err := convert.ParseFormat(req.ToFormat)
	if err != nil {
		return nil, statusError(connect.CodeInvalidArgument, err.Error())
	}
	if !to.Writable() {
		return nil, statusError(connect.CodeInvalidArgument, (&convert.NotWritableError{Format: to}).Error())
	}
	opts, err := convertOptions(req.IdForm, from, to)
	if err != nil {
		return nil, err
	}

	resp := &pb.ConvertResponse{FromFormat: from.String(), ToFormat: to.String()}
	// Marked on the response rather than left to the client to infer, so a caller
	// that let a format be inferred learns the mapping it got is experimental.
	if convert.IsExperimental(from, to) {
		resp.Experimental = true
		resp.ExperimentalNotice = convert.Notice(from, to)
	}
	out, syntax, err := convertModel(name, data, from, to, req.TolerateSyntaxErrors, opts)
	if err != nil {
		resp.Error = err.Error()
		var broken *convert.SyntaxError
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
func convertModel(name string, data []byte, from, to convert.Format, tolerant bool, opts convert.Options) ([]byte, *convert.SyntaxError, error) {
	if tolerant {
		return convert.ConvertTolerantWith(name, data, from, to, opts)
	}
	out, err := convert.ConvertWith(name, data, from, to, opts)
	return out, nil, err
}

// convertOptions reads id_form as `sysml -id` reads its argument: how derived
// element ids are spelled when notation is written as a graph. It is refused
// for any other direction, and for a value that names no id form.
func convertOptions(idForm string, from, to convert.Format) (convert.Options, error) {
	opts := convert.Options{}
	if idForm == "" {
		return opts, nil
	}
	if from != convert.FormatSysML || (to != convert.FormatTurtle && to != convert.FormatAPIJSON) {
		return opts, statusError(connect.CodeInvalidArgument, "id_form applies to notation converted to ttl or api-json")
	}
	form, ok := export.ParseIDForm(idForm)
	if !ok {
		return opts, statusErrorf(connect.CodeInvalidArgument, "id_form wants qualified or uuid, not %q", idForm)
	}
	opts.ID = form
	return opts, nil
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
		// A model of several documents is converted whole (convertModelOfDocuments).
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
func convertFrom(req *pb.ConvertRequest, name string) (convert.Format, error) {
	if req.FromFormat != "" {
		from, err := convert.ParseFormat(req.FromFormat)
		if err != nil {
			return 0, statusError(connect.CodeInvalidArgument, err.Error())
		}
		return from, nil
	}
	if req.GetModelHash() != "" {
		// Parse reads notation, so a cached model is notation whatever it was
		// named — including one parsed from inline content, which has no name.
		return convert.FormatSysML, nil
	}
	if req.GetFilePath() == "" {
		return 0, statusError(connect.CodeInvalidArgument, "from_format is required for inline content: expected "+convert.FormatList)
	}
	from, err := convert.FormatOfPath(name)
	if err != nil {
		return 0, statusError(connect.CodeInvalidArgument, convert.Advise(err, "pass from_format, or "+convert.ExtensionAdvice).Error())
	}
	return from, nil
}

// syntaxDiagnostics reports a SyntaxError as diagnostics, with spans when the
// input was notation and a bare message when it was not.
func syntaxDiagnostics(syntax *convert.SyntaxError) []*pb.Diagnostic {
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

// convertModelOfDocuments converts a cached model of several documents as one
// graph, each reference from one document to an element another declares
// linked to it, to Turtle or the API's JSON element form. done is false for any
// other request, which converts one document as before.
func (s *Service) convertModelOfDocuments(req *pb.ConvertRequest) (*pb.ConvertResponse, bool, error) {
	hash, ok := req.Source.(*pb.ConvertRequest_ModelHash)
	if !ok {
		return nil, false, nil
	}
	cached, found := s.cache.Get(hash.ModelHash)
	if !found || len(cached.Documents) < 2 {
		return nil, false, nil
	}
	to, err := convert.ParseFormat(req.ToFormat)
	if err != nil {
		return nil, true, statusError(connect.CodeInvalidArgument, err.Error())
	}
	if req.FromFormat != "" {
		if from, err := convert.ParseFormat(req.FromFormat); err != nil || from != convert.FormatSysML {
			return nil, true, statusError(connect.CodeInvalidArgument, "a model parsed from several documents is notation; from_format must be empty or sysml")
		}
	}
	if to != convert.FormatTurtle && to != convert.FormatAPIJSON {
		return nil, true, statusErrorf(connect.CodeFailedPrecondition,
			"notation is written for one document, and this model has %d; convert it to %s or %s, or convert each document",
			len(cached.Documents), convert.FormatTurtle, convert.FormatAPIJSON)
	}
	// A model parsed from notation is converted to a graph, so id_form applies.
	opts, err := convertOptions(req.IdForm, convert.FormatSysML, to)
	if err != nil {
		return nil, true, err
	}
	resp := &pb.ConvertResponse{FromFormat: convert.FormatSysML.String(), ToFormat: to.String()}
	if convert.IsExperimental(convert.FormatSysML, to) {
		resp.Experimental = true
		resp.ExperimentalNotice = convert.Notice(convert.FormatSysML, to)
	}
	// A document the parser could not read whole is refused, as one converted
	// alone is: the graph would silently miss what the parser skipped.
	var refused []string
	documents := make([]export.ModelDocument, 0, len(cached.Documents))
	for _, doc := range cached.Documents {
		if syntax := convert.SyntaxErrorOf(doc.Source.Name(), doc.Source, doc.ParseDiags); syntax != nil {
			refused = append(refused, syntax.Error())
			resp.Diagnostics = append(resp.Diagnostics, syntaxDiagnostics(syntax)...)
			continue
		}
		documents = append(documents, export.ModelDocument{File: doc.Source, Root: doc.Root})
	}
	if len(refused) > 0 {
		resp.Diagnostics = s.filterDiagnosticCapabilities(resp.Diagnostics)
		resp.Error = strings.Join(refused, "\n")
		return resp, true, nil
	}
	graph, err := export.ModelToRDFWith(documents, opts.ID)
	if err != nil {
		resp.Error = err.Error()
		return resp, true, nil
	}
	out, err := convert.FromGraph(graph, to)
	if err != nil {
		resp.Error = err.Error()
		return resp, true, nil
	}
	resp.Content = string(out)
	return resp, true, nil
}
