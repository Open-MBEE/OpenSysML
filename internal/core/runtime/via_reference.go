package runtime

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// viaSender re-roots a send whose via path starts at a feature the behavior binds to an
// object, such as a reference parameter: the message leaves that object's port.
func (ec *EvalContext) viaSender(send lower.Send, self *Instance) (lower.Send, *Instance, error) {
	if !send.IsVia || !send.TargetPath {
		return send, self, nil
	}
	holder, port, err := ec.viaHolder(send.Target, self)
	if err != nil {
		return send, self, err
	}
	if port == send.Target {
		return send, self, nil
	}
	send.Target, send.TargetPath = port, false
	return send, holder, nil
}

// viaHolder returns the object whose port a via path names and the port's name: self and
// the path as written, unless the root is a bound object rather than a feature of self.
func (ec *EvalContext) viaHolder(path string, self *Instance) (*Instance, string, error) {
	segments := strings.Split(path, ".")
	root := segments[0]
	if len(segments) < 2 || root == thisName {
		return self, path, nil
	}
	if self != nil {
		if _, held := self.FeatureValues[root]; held {
			return self, path, nil
		}
	}
	value, bound := ec.Lookup(root)
	if !bound {
		return self, path, nil
	}
	held := heldElements(value)
	if len(held) != 1 || held[0].Kind != ValInstance {
		return self, path, &SendTargetValueError{Target: path, Name: root, Value: FormatValue(value)}
	}
	holder, ok := ec.ctx.instances[held[0].Instance]
	if !ok {
		return self, path, &SendTargetValueError{Target: path, Name: root, Value: FormatValue(value)}
	}
	for _, segment := range segments[1 : len(segments)-1] {
		next, ok, err := ec.ctx.fvObject(holder, segment)
		if err != nil {
			return self, path, err
		}
		if !ok {
			return self, path, &UnknownSendPortError{Port: path}
		}
		holder = next
	}
	return holder, segments[len(segments)-1], nil
}
