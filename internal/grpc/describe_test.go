package grpc

import (
	"fmt"
	"strconv"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// describeValue reports the set oneof arm of a pb.Value and its payload.
func describeValue(v *pb.Value) (string, interface{}) {
	switch k := v.Kind.(type) {
	case *pb.Value_IntValue:
		return "int_value", k.IntValue
	case *pb.Value_RealValue:
		return "real_value", k.RealValue
	case *pb.Value_BoolValue:
		return "bool_value", k.BoolValue
	case *pb.Value_StringValue:
		return "string_value", k.StringValue
	case *pb.Value_InstanceId:
		return "instance_id", k.InstanceId
	case *pb.Value_Sequence:
		return "sequence", k.Sequence
	case *pb.Value_Quantity:
		return "quantity", describeQuantity(k.Quantity)
	case *pb.Value_Null:
		return "null", nil
	case *pb.Value_Unset:
		return "unset", nil
	default:
		return "no arm", nil
	}
}

// describeQuantity renders a quantity as "<magnitude> [<unit as written>] =
// <reduction>", the form the wire tests pin.
func describeQuantity(q *pb.Quantity) string {
	if q == nil {
		return ""
	}
	magnitude := "unset"
	switch m := q.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		magnitude = strconv.FormatInt(m.IntMagnitude, 10)
	case *pb.Quantity_RealMagnitude:
		magnitude = strconv.FormatFloat(m.RealMagnitude, 'g', -1, 64)
	}
	return fmt.Sprintf("%s [%s] = %s", magnitude, q.GetUnit(), describeUnitTerm(q.GetUnitTerm()))
}
