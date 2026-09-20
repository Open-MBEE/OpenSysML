package runtime

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// viaSender re-roots a send whose via path starts at a feature the behavior binds to an
// object, such as a reference parameter: the message leaves that object's port.
func (ec *EvalContext) viaSender(send lower.Send, self *Instance) (lower.Send, *Instance, error) {
	if ec == nil || !send.IsVia || !send.TargetPath {
		return send, self, nil
	}
	holder, port, err := ec.viaHolder(send.Target, send.ViaSelf, self)
	if err != nil {
		return send, self, err
	}
	if holder == self && port == send.Target {
		return send, self, nil
	}
	send.Target, send.TargetPath = port, false
	if fv, ok := holder.FeatureValues[port]; ok && fv.Feature != nil && fv.Feature.Symbol != nil {
		send.TargetSym = fv.Feature.Symbol
	}
	return send, holder, nil
}

// viaHolder returns the object whose port a via path names and the port's name: self and
// the path as written, unless the behavior binds the root, which shadows a same-named
// feature of self as it does in any expression. A path written from `this` is self's.
func (ec *EvalContext) viaHolder(path string, viaSelf bool, self *Instance) (*Instance, string, error) {
	segments := strings.Split(path, ".")
	root := segments[0]
	if viaSelf {
		return self, path, nil
	}
	held, bound, err := ec.boundHolders(root, path)
	if err != nil || !bound {
		return self, path, err
	}
	if len(held) != 1 {
		value, _ := ec.Lookup(root)
		return self, path, &SendTargetValueError{Target: path, Name: root, Value: FormatValue(value)}
	}
	holder := held[0]
	if len(segments) == 1 {
		owner, port, ok := portOwner(holder)
		if !ok {
			return self, path, &ViaNotPortError{Via: path, Value: FormatValue(Value{Kind: ValInstance, Instance: holder.ID})}
		}
		return owner, port, nil
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

// portOwner answers the object a port object belongs to and the port's name in it,
// false for an object that is no port.
func portOwner(inst *Instance) (*Instance, string, bool) {
	owner, feature := inst.Owner()
	if owner == nil {
		return nil, "", false
	}
	fv, ok := owner.FeatureValues[feature]
	if !ok || !isPortFeature(fv.Feature) {
		return nil, "", false
	}
	return owner, feature, true
}

// boundEndDeliveries resolves a connection end whose root the behavior binds, as a via
// path is: to the port of each object the binding holds. false where it binds none.
func (ec *EvalContext) boundEndDeliveries(end string) ([]ownerDelivery, bool, error) {
	segments := strings.Split(end, ".")
	if ec == nil || segments[0] == thisName {
		return nil, false, nil
	}
	held, bound, err := ec.boundHolders(segments[0], end)
	if err != nil || !bound {
		return nil, bound, err
	}
	if len(segments) == 1 {
		var out []ownerDelivery
		for _, inst := range held {
			owner, port, ok := portOwner(inst)
			if !ok {
				return nil, true, &ViaNotPortError{Via: end, Value: FormatValue(Value{Kind: ValInstance, Instance: inst.ID})}
			}
			out = append(out, ownerDelivery{object: owner.ID, port: port})
		}
		return out, true, nil
	}
	addrs, err := ec.ctx.addressesFrom(held, segments[1:])
	if err != nil {
		return nil, true, err
	}
	var out []ownerDelivery
	for _, addr := range addrs {
		if addr.Delivery == DeliverPort && addr.Object != 0 {
			out = append(out, ownerDelivery{object: addr.Object, port: addr.Port})
		}
	}
	return out, true, nil
}

// boundHolders returns the objects a binding of ec holds and whether ec binds the name;
// a binding holding anything but objects is that error, naming the path it leads.
func (ec *EvalContext) boundHolders(name, path string) ([]*Instance, bool, error) {
	value, bound := ec.Lookup(name)
	if !bound {
		return nil, false, nil
	}
	var out []*Instance
	for _, held := range heldElements(value) {
		inst, ok := ec.ctx.instances[held.Instance]
		if held.Kind != ValInstance || !ok {
			return nil, true, &SendTargetValueError{Target: path, Name: name, Value: FormatValue(value)}
		}
		out = append(out, inst)
	}
	return out, true, nil
}
