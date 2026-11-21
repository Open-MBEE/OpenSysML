package opensysml

import (
	"context"
	"errors"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// knownValues refuses a response holding a value of a kind this client does not
// know, which would otherwise read as a null the service never sent.
type knownValues struct{}

func (knownValues) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if err != nil {
			return resp, err
		}
		if message, ok := resp.Any().(proto.Message); ok && holdsUnknownValue(message.ProtoReflect()) {
			return nil, connect.NewError(connect.CodeUnimplemented, errUnknownValueArm)
		}
		return resp, nil
	}
}

func (knownValues) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (knownValues) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

var errUnknownValueArm = errors.New("the service answered with a value of a kind this client does not know; a newer client reads it")

var valueName = (&pb.Value{}).ProtoReflect().Descriptor().FullName()

// reachesValue memoizes, by message name, whether a message of that type can hold a Value.
var reachesValue sync.Map

// holdsUnknownValue reports whether message holds a Value with no kind set: a newer
// service's arm, which parses as an unknown field or, from JSON, as nothing.
func holdsUnknownValue(message protoreflect.Message) bool {
	descriptor := message.Descriptor()
	if !canHoldValue(descriptor, map[protoreflect.FullName]bool{}) {
		return false
	}
	if descriptor.FullName() == valueName && message.WhichOneof(descriptor.Oneofs().ByName("kind")) == nil {
		return true
	}
	found := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case field.Message() == nil || field.IsMap() && field.MapValue().Message() == nil:
		case field.IsList():
			list := value.List()
			for i := 0; i < list.Len() && !found; i++ {
				found = holdsUnknownValue(list.Get(i).Message())
			}
		case field.IsMap():
			value.Map().Range(func(_ protoreflect.MapKey, entry protoreflect.Value) bool {
				found = holdsUnknownValue(entry.Message())
				return !found
			})
		default:
			found = holdsUnknownValue(value.Message())
		}
		return !found
	})
	return found
}

// canHoldValue reports whether a message of this type is, or can contain, a Value.
// visiting holds the types on the path, which a recursive type would otherwise revisit.
func canHoldValue(descriptor protoreflect.MessageDescriptor, visiting map[protoreflect.FullName]bool) bool {
	name := descriptor.FullName()
	if name == valueName {
		return true
	}
	if known, ok := reachesValue.Load(name); ok {
		return known.(bool)
	}
	if visiting[name] {
		return false
	}
	visiting[name] = true
	reaches := false
	fields := descriptor.Fields()
	for i := 0; i < fields.Len() && !reaches; i++ {
		field := fields.Get(i)
		if field.IsMap() {
			field = field.MapValue()
		}
		if field.Message() != nil {
			reaches = canHoldValue(field.Message(), visiting)
		}
	}
	delete(visiting, name)
	// A false met inside a cycle may be an artefact of the cut, so only the root's answer is kept.
	if reaches || len(visiting) == 0 {
		reachesValue.Store(name, reaches)
	}
	return reaches
}
