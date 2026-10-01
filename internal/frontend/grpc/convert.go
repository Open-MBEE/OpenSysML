package grpc

import (
	"math"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// SymbolToProtoIn converts a Symbol to protobuf SymbolInfo in an existing
// conversion context.
func SymbolToProtoIn(sym *symbols.Symbol, sc *symbolfacts.Context) *pb.SymbolInfo {
	return infoToProto(symbolfacts.Of(sym, sc), sc.Index)
}

func infoToProto(info *symbolfacts.Info, idx *symbols.Index) *pb.SymbolInfo {
	if info == nil {
		return nil
	}
	out := &pb.SymbolInfo{
		Id:                        info.Id,
		Name:                      info.Name,
		Kind:                      info.Kind,
		Metadata:                  info.Metadata,
		ChildIds:                  info.ChildIds,
		WithheldLibraryAttributes: info.WithheldLibraryAttributes,
	}
	if info.Attributes != nil {
		out.Attributes = make([]*pb.AttributeInfo, 0, len(info.Attributes))
	}
	if t := info.TypeInfo; t != nil {
		out.TypeInfo = &pb.TypeInfo{
			Declared:        t.Declared,
			ResolvedId:      t.ResolvedId,
			ResolvedKind:    t.ResolvedKind,
			Primitive:       t.Primitive,
			PrimitiveSource: t.PrimitiveSource,
			Quantity:        t.Quantity,
			Unit:            t.Unit,
		}
	}
	if m := info.Multiplicity; m != nil {
		out.Multiplicity = &pb.MultiplicityInfo{Lower: m.Lower, Upper: m.Upper}
	}
	for _, spec := range info.Specializations {
		out.Specializations = append(out.Specializations, &pb.Specialization{
			Kind: spec.Kind, Declared: spec.Declared, TargetId: spec.TargetId, TargetKind: spec.TargetKind,
		})
	}
	for _, attribute := range info.Attributes {
		a := &pb.AttributeInfo{Name: attribute.Name, Type: attribute.Type, Unit: attribute.Unit}
		if attribute.Value != nil {
			switch {
			case attribute.Value.String != nil:
				a.Value = &pb.Value{Kind: &pb.Value_StringValue{StringValue: *attribute.Value.String}}
			case attribute.Value.Const != nil:
				a.Value = protoconv.ValueToProto(runtime.Value{
					Kind: runtime.ValConst, Const: *attribute.Value.Const,
				}, idx)
			}
		}
		out.Attributes = append(out.Attributes, a)
	}
	return out
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

// DiagnosticToProto converts a diag.Diagnostic to protobuf.
func DiagnosticToProto(diag diag.Diagnostic, sf *source.SourceFile) *pb.Diagnostic {
	li := sf.Lines()
	start := li.PosAt(diag.Span.Offset)
	end := li.PosAt(diag.Span.End())

	return &pb.Diagnostic{
		Severity: diag.Severity.String(),
		Message:  diag.Message,
		Code:     diag.Code,
		Span: &pb.Span{
			File:      sf.Name(),
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
		},
	}
}

// RunNoteDiagnosticsToProto converts what a run noted about itself — its choice
// points and the guards it could not evaluate — to informational diagnostics,
// located in their model document; one outside the model carries no span.
func RunNoteDiagnosticsToProto(notes []runtime.RunNote, model *CachedModel) []*pb.Diagnostic {
	if len(notes) == 0 {
		return nil
	}
	pbDiags := make([]*pb.Diagnostic, 0, len(notes))
	for _, n := range notes {
		diag := n.Diagnostic()
		file, _ := n.Location()
		if sf := model.document(file); sf != nil {
			pbDiags = append(pbDiags, DiagnosticToProto(diag, sf))
			continue
		}
		pbDiags = append(pbDiags, &pb.Diagnostic{Severity: diag.Severity.String(), Message: diag.Message, Code: diag.Code})
	}
	return pbDiags
}

// SyntaxDiagnosticCode is the code a parser error is reported under when the
// parser gave it none of its own.
const SyntaxDiagnosticCode = parser.CodeSyntax

// ParserDiagnosticToProto converts a parser error to protobuf.
func ParserDiagnosticToProto(diag parser.Diagnostic, sf *source.SourceFile) *pb.Diagnostic {
	li := sf.Lines()
	start := li.PosAt(diag.Span.Offset)
	end := li.PosAt(diag.Span.End())

	return &pb.Diagnostic{
		Severity: "error", // Parser diagnostics are always errors
		Message:  diag.Message,
		Code:     diag.ErrorCode(),
		Span: &pb.Span{
			File:      sf.Name(),
			StartLine: int32Clamp(start.Line),
			StartCol:  int32Clamp(start.Col),
			EndLine:   int32Clamp(end.Line),
			EndCol:    int32Clamp(end.Col),
		},
	}
}
