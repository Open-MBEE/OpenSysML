package opensysml

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestCallerCoversEveryRPC ties the transport interface to the proto: every
// RPC of the service has one caller method answering its response type, so an
// RPC added to the proto fails here until the client can make it.
func TestCallerCoversEveryRPC(t *testing.T) {
	answered := map[protoreflect.FullName][]string{}
	callerType := reflect.TypeOf((*caller)(nil)).Elem()
	messageType := reflect.TypeOf((*proto.Message)(nil)).Elem()
	for i := 0; i < callerType.NumMethod(); i++ {
		method := callerType.Method(i)
		if method.Type.NumOut() == 0 || !method.Type.Out(0).Implements(messageType) {
			continue
		}
		response := reflect.Zero(method.Type.Out(0)).Interface().(proto.Message)
		name := response.ProtoReflect().Descriptor().FullName()
		answered[name] = append(answered[name], method.Name)
	}

	services := pb.File_sysml_proto.Services()
	rpcs := 0
	for i := 0; i < services.Len(); i++ {
		methods := services.Get(i).Methods()
		for j := 0; j < methods.Len(); j++ {
			rpc := methods.Get(j)
			rpcs++
			output := rpc.Output().FullName()
			switch got := answered[output]; len(got) {
			case 1:
			case 0:
				t.Errorf("no caller method answers %s (%s)", rpc.Name(), output)
			default:
				t.Errorf("%s (%s) is answered by more than one caller method: %v", rpc.Name(), output, got)
			}
			delete(answered, output)
		}
	}
	if rpcs == 0 {
		t.Fatal("the proto declares no RPC")
	}
	for output, methods := range answered {
		t.Errorf("caller methods %v answer %s, which no RPC returns", methods, output)
	}
}
