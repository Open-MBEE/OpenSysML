package opensysml

import (
	"fmt"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

// The conversions from the wire types to the public ones. Every conversion
// copies: nothing a Client returns aliases engine state or a protobuf message,
// so the caller owns what it gets, in process or not.

func symbolFromProto(sym *pb.SymbolInfo) *Symbol {
	if sym == nil {
		return nil
	}
	out := &Symbol{
		ID:                        sym.Id,
		Name:                      sym.Name,
		Kind:                      sym.Kind,
		WithheldLibraryAttributes: int(sym.WithheldLibraryAttributes),
	}
	if len(sym.Metadata) > 0 {
		out.Metadata = make(map[string]string, len(sym.Metadata))
		for key, value := range sym.Metadata {
			out.Metadata[key] = value
		}
	}
	if len(sym.ChildIds) > 0 {
		out.ChildIDs = append([]string(nil), sym.ChildIds...)
	}
	for _, attr := range sym.Attributes {
		out.Attributes = append(out.Attributes, Attribute{
			Name:  attr.Name,
			Type:  attr.Type,
			Value: valueFromProto(attr.Value),
			Unit:  attr.Unit,
		})
	}
	if sym.TypeInfo != nil {
		out.Type = &TypeInfo{
			Declared:        sym.TypeInfo.Declared,
			ResolvedID:      sym.TypeInfo.ResolvedId,
			ResolvedKind:    sym.TypeInfo.ResolvedKind,
			Primitive:       sym.TypeInfo.Primitive,
			PrimitiveSource: sym.TypeInfo.PrimitiveSource,
			Quantity:        sym.TypeInfo.Quantity,
			Unit:            sym.TypeInfo.Unit,
		}
	}
	if sym.Multiplicity != nil {
		out.Multiplicity = &Multiplicity{Lower: sym.Multiplicity.Lower, Upper: sym.Multiplicity.Upper}
	}
	for _, spec := range sym.Specializations {
		out.Specializations = append(out.Specializations, Specialization{
			Kind:       spec.Kind,
			Declared:   spec.Declared,
			TargetID:   spec.TargetId,
			TargetKind: spec.TargetKind,
		})
	}
	return out
}

func diagnosticsFromProto(diags []*pb.Diagnostic) []Diagnostic {
	if len(diags) == 0 {
		return nil
	}
	out := make([]Diagnostic, 0, len(diags))
	for _, diag := range diags {
		converted := Diagnostic{Severity: diag.Severity, Message: diag.Message, Code: diag.Code}
		if diag.Span != nil {
			converted.Span = &Span{
				File:      diag.Span.File,
				StartLine: int(diag.Span.StartLine),
				StartCol:  int(diag.Span.StartCol),
				EndLine:   int(diag.Span.EndLine),
				EndCol:    int(diag.Span.EndCol),
			}
		}
		out = append(out, converted)
	}
	return out
}

func valueFromProto(value *pb.Value) Value {
	if value == nil {
		return nil
	}
	switch kind := value.Kind.(type) {
	case *pb.Value_IntValue:
		return Int(kind.IntValue)
	case *pb.Value_RealValue:
		return Real(kind.RealValue)
	case *pb.Value_Complex:
		return Complex(sysmlgrpc.ProtoToComplex(kind.Complex))
	case *pb.Value_BoolValue:
		return Bool(kind.BoolValue)
	case *pb.Value_StringValue:
		return String(kind.StringValue)
	case *pb.Value_InstanceId:
		return InstanceID(kind.InstanceId)
	case *pb.Value_Sequence:
		elements := make(Sequence, 0, len(kind.Sequence.GetElements()))
		for _, element := range kind.Sequence.GetElements() {
			elements = append(elements, valueFromProto(element))
		}
		return elements
	case *pb.Value_Null:
		return Null(kind.Null)
	case *pb.Value_Quantity:
		quantity, ok := quantityFromProto(kind.Quantity)
		if !ok {
			return Null("unsupported: quantity without a magnitude")
		}
		return quantity
	case *pb.Value_EnumLiteral:
		return EnumLiteral{
			LiteralID:     kind.EnumLiteral.GetLiteralId(),
			EnumerationID: kind.EnumLiteral.GetEnumerationId(),
			Name:          kind.EnumLiteral.GetName(),
		}
	case *pb.Value_Unset:
		return Unset{}
	case *pb.Value_Array:
		if err := sysmlgrpc.CheckArrayShape(kind.Array.GetDimensions(), len(kind.Array.GetElements())); err != nil {
			return Null("unsupported: " + err.Error())
		}
		out := Array{
			Dimensions: append([]int64(nil), kind.Array.GetDimensions()...),
			Elements:   make([]Value, 0, len(kind.Array.GetElements())),
		}
		for _, element := range kind.Array.GetElements() {
			out.Elements = append(out.Elements, valueFromProto(element))
		}
		return out
	case *pb.Value_Vector:
		out := make(Vector, 0, len(kind.Vector.GetComponents()))
		for _, component := range kind.Vector.GetComponents() {
			number, ok := valueFromProto(component).(Number)
			if !ok {
				return Null("unsupported: vector with a non-numeric component")
			}
			out = append(out, number)
		}
		return out
	case *pb.Value_VectorQuantity:
		if len(kind.VectorQuantity.GetComponents()) == 0 {
			return Null("unsupported: vector quantity without components")
		}
		out := make(VectorQuantity, 0, len(kind.VectorQuantity.GetComponents()))
		for _, component := range kind.VectorQuantity.GetComponents() {
			quantity, ok := quantityFromProto(component)
			if !ok {
				return Null("unsupported: vector quantity with a component without a magnitude")
			}
			out = append(out, quantity)
		}
		return out
	case *pb.Value_MeasurementRef:
		ref := kind.MeasurementRef
		if ref.GetUnit() == "" && ref.GetUnitTerm() == nil && ref.GetUnitId() == "" {
			return Null("unsupported: measurement reference naming no unit")
		}
		if ref.GetUnitTerm() == nil {
			return Null("unsupported: measurement reference without its reduction")
		}
		return MeasurementRef{Unit: ref.GetUnit(), Term: unitTermFromProto(ref.GetUnitTerm()), UnitID: ref.GetUnitId()}
	case *pb.Value_Function:
		if kind.Function.GetCalcId() == "" {
			return Null("unsupported: function naming no calc")
		}
		return Function{CalcID: kind.Function.GetCalcId(), Self: InstanceID(kind.Function.GetSelfId())}
	case *pb.Value_Set:
		out := make(Set, 0, len(kind.Set.GetElements()))
		for _, element := range kind.Set.GetElements() {
			member := valueFromProto(element)
			if out.Contains(member) {
				return Null(fmt.Sprintf("unsupported: set lists a member twice: %v", member))
			}
			out = append(out, member)
		}
		return out
	case *pb.Value_TensorQuantity:
		if err := sysmlgrpc.CheckTensorShape(kind.TensorQuantity.GetDimensions(), len(kind.TensorQuantity.GetComponents())); err != nil {
			return Null("unsupported: " + err.Error())
		}
		out := TensorQuantity{
			Dimensions: append([]int64(nil), kind.TensorQuantity.GetDimensions()...),
			Components: make([]Quantity, 0, len(kind.TensorQuantity.GetComponents())),
		}
		for _, component := range kind.TensorQuantity.GetComponents() {
			quantity, ok := quantityFromProto(component)
			if !ok {
				return Null("unsupported: tensor quantity with a component without a magnitude")
			}
			out.Components = append(out.Components, quantity)
		}
		return out
	default:
		// A newer service's arm parses as an unknown field: no kind at all.
		return Null("unsupported: a value arm this client does not know")
	}
}

// valueToProto marshals a value a caller supplies. Unset is refused here, the
// way the service refuses it: it reports that a feature holds no value, which
// is something to read rather than to send.
func valueToProto(value Value) (*pb.Value, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case Int:
		return &pb.Value{Kind: &pb.Value_IntValue{IntValue: int64(v)}}, nil
	case Real:
		return &pb.Value{Kind: &pb.Value_RealValue{RealValue: float64(v)}}, nil
	case Complex:
		return &pb.Value{Kind: &pb.Value_Complex{Complex: sysmlgrpc.ComplexToProto(complex128(v))}}, nil
	case Bool:
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: bool(v)}}, nil
	case String:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: string(v)}}, nil
	case InstanceID:
		return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: int64(v)}}, nil
	case Null:
		return &pb.Value{Kind: &pb.Value_Null{Null: string(v)}}, nil
	case Sequence:
		sequence := &pb.ValueSequence{Elements: make([]*pb.Value, 0, len(v))}
		for _, element := range v {
			sent, err := valueToProto(element)
			if err != nil {
				return nil, err
			}
			sequence.Elements = append(sequence.Elements, sent)
		}
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: sequence}}, nil
	case Quantity:
		return &pb.Value{Kind: &pb.Value_Quantity{Quantity: quantityToProto(v)}}, nil
	case EnumLiteral:
		return &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: &pb.EnumLiteral{
			LiteralId:     v.LiteralID,
			EnumerationId: v.EnumerationID,
			Name:          v.Name,
		}}}, nil
	case Array:
		array := &pb.Array{
			Dimensions: append([]int64(nil), v.Dimensions...),
			Elements:   make([]*pb.Value, 0, len(v.Elements)),
		}
		for _, element := range v.Elements {
			sent, err := valueToProto(element)
			if err != nil {
				return nil, err
			}
			array.Elements = append(array.Elements, sent)
		}
		return &pb.Value{Kind: &pb.Value_Array{Array: array}}, nil
	case Vector:
		vector := &pb.Vector{Components: make([]*pb.Value, 0, len(v))}
		for _, component := range v {
			sent, err := valueToProto(component)
			if err != nil {
				return nil, err
			}
			vector.Components = append(vector.Components, sent)
		}
		return &pb.Value{Kind: &pb.Value_Vector{Vector: vector}}, nil
	case VectorQuantity:
		vq := &pb.VectorQuantity{Components: make([]*pb.Quantity, 0, len(v))}
		for _, component := range v {
			vq.Components = append(vq.Components, quantityToProto(component))
		}
		return &pb.Value{Kind: &pb.Value_VectorQuantity{VectorQuantity: vq}}, nil
	case MeasurementRef:
		return &pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: &pb.MeasurementRef{
			Unit:     v.Unit,
			UnitTerm: unitTermToProto(v.Term),
			UnitId:   v.UnitID,
		}}}, nil
	case Function:
		if v.CalcID == "" {
			return nil, &StatusError{Code: CodeInvalidArgument, Message: "a function names no calc"}
		}
		return &pb.Value{Kind: &pb.Value_Function{Function: &pb.Function{CalcId: v.CalcID, SelfId: int64(v.Self)}}}, nil
	case Set:
		if twice, ok := v.repeated(); ok {
			return nil, &StatusError{
				Code:    CodeInvalidArgument,
				Message: fmt.Sprintf("set lists a member twice: %v", twice),
			}
		}
		set := &pb.ValueSet{Elements: make([]*pb.Value, 0, len(v))}
		for _, element := range v {
			sent, err := valueToProto(element)
			if err != nil {
				return nil, err
			}
			set.Elements = append(set.Elements, sent)
		}
		return &pb.Value{Kind: &pb.Value_Set{Set: set}}, nil
	case TensorQuantity:
		tq := &pb.TensorQuantity{
			Dimensions: append([]int64(nil), v.Dimensions...),
			Components: make([]*pb.Quantity, 0, len(v.Components)),
		}
		for _, component := range v.Components {
			tq.Components = append(tq.Components, quantityToProto(component))
		}
		return &pb.Value{Kind: &pb.Value_TensorQuantity{TensorQuantity: tq}}, nil
	case Unset:
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "unset is not a value a caller can supply",
		}
	default:
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "unknown value kind"}
	}
}

func quantityToProto(quantity Quantity) *pb.Quantity {
	out := &pb.Quantity{Unit: quantity.Unit}
	switch magnitude := quantity.Magnitude.(type) {
	case Int:
		out.Magnitude = &pb.Quantity_IntMagnitude{IntMagnitude: int64(magnitude)}
	case Real:
		out.Magnitude = &pb.Quantity_RealMagnitude{RealMagnitude: float64(magnitude)}
	}
	out.UnitTerm = unitTermToProto(quantity.Term)
	return out
}

func unitTermToProto(term *UnitTerm) *pb.UnitTerm {
	if term == nil {
		return nil
	}
	out := &pb.UnitTerm{ScaleNum: term.ScaleNum, ScaleDen: term.ScaleDen}
	for _, factor := range term.Factors {
		out.Factors = append(out.Factors, &pb.UnitFactor{UnitId: factor.UnitID, Exponent: factor.Exponent})
	}
	return out
}

func unitTermFromProto(term *pb.UnitTerm) *UnitTerm {
	if term == nil {
		return nil
	}
	out := &UnitTerm{ScaleNum: term.ScaleNum, ScaleDen: term.ScaleDen}
	for _, factor := range term.Factors {
		out.Factors = append(out.Factors, UnitFactor{UnitID: factor.UnitId, Exponent: factor.Exponent})
	}
	return out
}

// valuesFromProto converts a map of values, keeping the keys the service used.
func valuesFromProto(values map[string]*pb.Value) map[string]Value {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]Value, len(values))
	for name, value := range values {
		out[name] = valueFromProto(value)
	}
	return out
}

// instancesFromProto converts an instance graph.
func instancesFromProto(instances []*pb.Instance) []*Instance {
	if len(instances) == 0 {
		return nil
	}
	out := make([]*Instance, 0, len(instances))
	for _, instance := range instances {
		out = append(out, instanceFromProto(instance))
	}
	return out
}

// quantityFromProto is false for a quantity carrying no magnitude, which no
// number stands in for.
func quantityFromProto(quantity *pb.Quantity) (Quantity, bool) {
	out := Quantity{Unit: quantity.GetUnit()}
	switch magnitude := quantity.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		out.Magnitude = Int(magnitude.IntMagnitude)
	case *pb.Quantity_RealMagnitude:
		out.Magnitude = Real(magnitude.RealMagnitude)
	default:
		return Quantity{}, false
	}
	out.Term = unitTermFromProto(quantity.GetUnitTerm())
	return out, true
}

func instanceFromProto(inst *pb.Instance) *Instance {
	if inst == nil {
		return nil
	}
	out := &Instance{ID: inst.Id, TypeSymbolID: inst.TypeSymbolId}
	if len(inst.FeatureValues) > 0 {
		out.FeatureValues = make(map[string]FeatureValue, len(inst.FeatureValues))
		for name, fv := range inst.FeatureValues {
			converted := FeatureValue{
				FeatureName:  fv.FeatureName,
				Value:        valueFromProto(fv.Value),
				Materialized: fv.Materialized,
				Error:        fv.Error,
			}
			for _, value := range fv.Values {
				converted.Values = append(converted.Values, valueFromProto(value))
			}
			out.FeatureValues[name] = converted
		}
	}
	return out
}
