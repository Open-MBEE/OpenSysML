package runtime

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Message is a signal instance in flight.
//
// SignalType names the message's type: the type the send statement named, or
// the scalar type of the value it evaluated to. An accept whose parameter is
// typed consumes only messages of that type, so two sends of different types
// reach different accepts regardless of the order they were posted in.
//
// Target names the receiving node of the sending behavior a `send m to r`
// addressed; a consumer accepts a message addressed to itself or to no one.
//
// Port names the port the message reached — the peer end a `via p` send routed
// to, or the port an addressed send resolved to — and only an accept on that
// port consumes it, keeping port-routed and addressed traffic separate.
//
// Object identifies the object the message reached, 0 for none, and Delivery
// what of that destination a consumer must satisfy to take the message.
//
// PortID identifies the port object the message reached, 0 where the run holds
// none. A binding connector makes a boundary port and an inner port one object,
// so an accept on either port takes a message that reached the other.
type Message struct {
	SignalType string
	// Signal is the definition SignalType resolved to when the send was built,
	// nil where the message's type is known only as a name. An accept matches it
	// by conformance, so a subtype message satisfies a supertype accept and
	// same-named definitions of different packages stay apart.
	Signal *symbols.Symbol
	// Event is the feature the message was sent from (`send shutDown to x`), nil
	// where none or unresolved; EventName is its written name, the fallback then.
	// EventObject is the occurrence that feature held when sent, or the object
	// performing a behavioral feature (`send alert()`), 0 for none: two parts of
	// one type hold their own occurrences of the one declared event.
	Event       *symbols.Symbol
	EventName   string
	EventObject int64
	Target      string
	Port        string
	Object      int64
	PortID      int64
	Delivery    DeliveryKind
	// Payload binds features of the message's type by name, as the send's arguments did.
	Payload map[string]Value
	// Value is the one value a send of an expression carries (`send 7`, `send d`),
	// the occurrence `send new T(…)` constructed, or the one an accept built from
	// Payload; nil until any.
	Value *Value
}

// DeliveryKind is what a message's destination resolved to, and so what a
// consumer must match: an unaddressed message resolved nothing and any consumer
// may take it, while every addressed or routed one names a destination in full.
type DeliveryKind uint8

const (
	// DeliverAnyone is a message no send addressed, such as one injected from
	// outside the model: it has no destination to hold a consumer to.
	DeliverAnyone DeliveryKind = iota
	// DeliverPort is the port of an object, reached by a connection or addressed.
	DeliverPort
	// DeliverPortReceiver is a receiver of an object reached through a port.
	DeliverPortReceiver
	// DeliverReceiver is the receiving node named within an object.
	DeliverReceiver
	// DeliverObject is an object itself, whichever of its consumers accepts.
	DeliverObject
)

// Call is the payload of an EventCall: the operation invoked and its arguments.
type Call struct {
	Operation string
	Args      map[string]Value
}

// PostMessage puts a message on the context-wide bus, where every executor
// sharing this context can see it. Actions and state machines communicate
// through this bus rather than through per-executor queues, so a message a
// state machine's entry action sends can be accepted by one of its transitions.
//
// A message posted with a destination but no Delivery — one injected from
// outside the model — is held to the destination it names.
func (ctx *Context) PostMessage(msg Message) {
	if msg.Delivery == DeliverAnyone {
		msg.Delivery = deliveryOf(msg)
	}
	ctx.messages = append(ctx.messages, msg)
}

// deliveryOf is what the fields of a message name as its destination, most
// specific first: only a message naming nothing is open to any consumer.
func deliveryOf(msg Message) DeliveryKind {
	switch {
	case msg.Port != "" && msg.Target != "":
		return DeliverPortReceiver
	case msg.Port != "":
		return DeliverPort
	case msg.Target != "":
		return DeliverReceiver
	case msg.Object != 0:
		return DeliverObject
	}
	return DeliverAnyone
}

// TakeMessage removes and returns the oldest message satisfying match. Messages
// that do not match keep their place in the queue, so a consumer looking for
// one type does not consume or reorder another's. A match probing the machines
// may post and drop messages behind the one it examines; those are visited too.
func (ctx *Context) TakeMessage(match func(Message) bool) (Message, bool) {
	for i := 0; i < len(ctx.messages); i++ {
		if msg := ctx.messages[i]; match(msg) {
			ctx.messages = append(ctx.messages[:i], ctx.messages[i+1:]...)
			return msg, true
		}
	}
	return Message{}, false
}

// PendingMessages returns the messages still in flight, oldest first.
func (ctx *Context) PendingMessages() []Message {
	out := make([]Message, len(ctx.messages))
	copy(out, ctx.messages)
	return out
}

// SignalDefinitionKinds are the definitions a signal is declared as: what an
// accept may be typed by and a send may carry. A behavior definition is neither.
var SignalDefinitionKinds = []symbols.SymbolKind{
	symbols.SymbolAttributeDef,
	symbols.SymbolItemDef,
	symbols.SymbolOccurrenceDef,
	symbols.SymbolPartDef,
	symbols.SymbolIndividualDef,
	symbols.SymbolEnumerationDef,
	symbols.SymbolMetadataDef,
}

// IsSignalDefinition reports whether a symbol declares a definition a signal may
// be typed by, one of SignalDefinitionKinds.
func IsSignalDefinition(sym *symbols.Symbol) bool {
	return isDefinitionSymbol(sym) && slices.Contains(SignalDefinitionKinds, sym.Kind)
}

// SignalMessage builds the message `send Signal(args) to <object>` posts, for PostMessage;
// a nil object is one anyone may take, an argument no feature of the signal admits is refused.
// An element that is no signal definition (see SignalDefinitionKinds) is refused with
// ErrNotASignal.
func (ctx *Context) SignalMessage(signal *symbols.Symbol, args map[string]Value, to *Instance) (Message, error) {
	if !IsSignalDefinition(signal) {
		return Message{}, fmt.Errorf("%w: %s", ErrNotASignal, symbolText(signal))
	}
	features := ctx.FeaturesOf(signal)
	payload := make(map[string]Value, len(args))
	for _, name := range sortedArgNames(args) {
		feat := carriedFeature(features, name)
		if feat == nil {
			return Message{}, fmt.Errorf("%w: %s carries no feature %q%s",
				ErrSignalArgument, symbolText(signal), name, carriedFeaturesNote(features))
		}
		arg := args[name]
		if err := ctx.checkAdmits(feat, symbolText(signal)+"."+name, &arg, admitWritten); err != nil {
			return Message{}, fmt.Errorf("%w: %w", ErrSignalArgument, err)
		}
		payload[name] = arg
	}
	msg := NamedSignalMessage(signal.Name, to)
	msg.Signal, msg.Payload = signal, payload
	return msg, nil
}

// NamedSignalMessage builds the message a send of a signal no declaration types
// posts: matched by name alone, addressed to the object when there is one.
func NamedSignalMessage(name string, to *Instance) Message {
	msg := Message{SignalType: name}
	if addr, ok := objectAddress(objectID(to)); ok {
		msg.Object, msg.Delivery = addr.Object, addr.Delivery
	}
	return msg
}

// sortedArgNames lists the argument names in a stable order.
func sortedArgNames(args map[string]Value) []string {
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// carriedFeature is the feature of the name, nil when none has it.
func carriedFeature(features []EffectiveFeature, name string) *EffectiveFeature {
	for i := range features {
		if features[i].Name == name {
			return &features[i]
		}
	}
	return nil
}

// carriedFeaturesNote names the features a signal carries, for an argument error.
func carriedFeaturesNote(features []EffectiveFeature) string {
	if len(features) == 0 {
		return " (it carries none)"
	}
	names := make([]string, 0, len(features))
	for _, feat := range features {
		names = append(names, feat.Name)
	}
	return " (it carries " + strings.Join(names, ", ") + ")"
}

// reaches reports whether a consumer named name, accepting on port and performed
// by object, may take this message: every part of the destination must hold, the
// ports always included, so port-routed and addressed traffic stay apart. A
// behavior no object performs has no identity to compare, so a message naming
// the receiver it is neither excludes it nor is excluded by it.
func (m Message) reaches(name, port string, object int64) bool {
	if m.Port != port {
		return false
	}
	switch m.Delivery {
	case DeliverPort, DeliverObject:
		return m.Object == object
	case DeliverPortReceiver:
		return m.Target == name && m.Object == object
	case DeliverReceiver:
		return m.Target == name && (m.Object == object || m.Object == 0 || object == 0)
	}
	return true
}

// objectID is the identity of an object, 0 for none.
func objectID(inst *Instance) int64 {
	if inst == nil {
		return 0
	}
	return inst.ID
}

// messageReaches reports whether a consumer named name, accepting on port and
// performed by self, may take message m: by the destination as written, or by
// the identity of the port object it reached — a port a binding connector
// joins to another is that other port, whichever object's feature names it.
// Resolving the consumer's port may materialize it, which can fail.
func (ctx *Context) messageReaches(m Message, name, port string, self *Instance) (bool, error) {
	if m.reaches(name, port, objectID(self)) {
		return true, nil
	}
	if m.PortID == 0 || port == "" {
		return false, nil
	}
	portID, err := ctx.portInstanceID(self, port)
	if err != nil {
		return false, err
	}
	if m.PortID != portID {
		return false, nil
	}
	switch m.Delivery {
	case DeliverPort:
		return true, nil
	case DeliverPortReceiver:
		return m.Target == name, nil
	}
	return false, nil
}

// portInstanceID is the identity of the port object a dotted port path of
// holder names, materializing it; 0 where the path names no port object.
func (ctx *Context) portInstanceID(holder *Instance, port string) (int64, error) {
	if holder == nil || port == "" {
		return 0, nil
	}
	current := holder
	segments := strings.Split(port, ".")
	for i, segment := range segments {
		fv, held := current.FeatureValues[segment]
		if !held || (i == len(segments)-1 && !isPortFeature(fv.Feature)) {
			return 0, nil
		}
		next, ok, err := ctx.fvObject(current, segment)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, nil
		}
		current = next
	}
	return current.ID, nil
}

// postVia routes a message out of a sending port: every port joined to it that
// can receive the message gets a copy, which is the ends whose flow features
// carry inward after conjugation. A send that reaches none of them is delivered
// nowhere, which is a typed error rather than a message quietly dropped — the
// model asked for a delivery the connections it declares cannot make.
func (ctx *Context) postVia(conns []lower.Connection, msg Message, send lower.Send, self *Instance) error {
	if send.Receiver != "" && send.Scope != nil {
		sym, ok := ctx.portSymbol(send.Scope, send.Target)
		if !ok || sym == nil || sym.Kind != symbols.SymbolPortUsage ||
			!ctx.ownPortPath(send.Scope, strings.Split(send.Target, ".")) {
			return &UnknownSendPortError{Port: send.Target, Receiver: send.Receiver}
		}
	}
	receiver := send.Receiver
	if receiver != "" {
		receiverSend := send
		receiverSend.Target = send.Receiver
		receiverSend.TargetPath = send.ReceiverPath
		receiverSend.IsVia = false
		addr, err := ctx.resolveRoutedReceiver(receiverSend, self)
		if err != nil || (addr.Object != 0 && addr.Object != objectID(self)) {
			return &UnreachableSendReceiverError{Port: send.Target, Receiver: send.Receiver}
		}
		receiver = addr.Name
	}
	routable := ctx.realizedConnections(ctx.routableConnections(conns, self, send.Scope), self)
	receiving, outbound, typeMismatch, err := ctx.connectedDeliveries(
		routable, self, send, msg, receiver != "",
	)
	if err != nil {
		return err
	}
	crossing, crossMismatch, err := ctx.ownerDeliveries(self, send, msg, receiver != "")
	if err != nil {
		return err
	}
	typeMismatch = typeMismatch || crossMismatch
	if len(receiving) == 0 && len(crossing) == 0 {
		if typeMismatch && receiver != "" {
			return &SendPortTypeMismatchError{
				Port: send.Target, Receiver: receiver, SignalType: msg.SignalType,
			}
		}
		return &UnroutableSendError{Port: send.Target, Outbound: outbound}
	}
	// A connection joins two objects, so each copy is held to the identity of the
	// object whose port the end resolved to rather than to the sender's. Two
	// destinations naming one port object (through a binding) get one copy.
	// Every destination is resolved before any copy is queued, so a failure
	// leaves nothing behind.
	posted := map[ownerDelivery]bool{}
	postedPorts := map[int64]bool{}
	var routed []Message
	for _, delivery := range append(receiving, crossing...) {
		if posted[delivery] {
			continue
		}
		posted[delivery] = true
		portID, err := ctx.portInstanceID(ctx.instances[delivery.object], delivery.port)
		if err != nil {
			return err
		}
		if portID != 0 {
			if postedPorts[portID] {
				continue
			}
			postedPorts[portID] = true
		}
		copied := msg
		copied.Target = receiver
		copied.Port = delivery.port
		copied.Object = delivery.object
		copied.PortID = portID
		copied.Delivery = DeliverPort
		if receiver != "" {
			copied.Delivery = DeliverPortReceiver
		}
		routed = append(routed, copied)
	}
	for _, m := range routed {
		ctx.PostMessage(m)
	}
	return nil
}

// resolveRoutedReceiver requires the named receiver to be an action or state
// reachable on the sending object before it can accept the routed message.
func (ctx *Context) resolveRoutedReceiver(send lower.Send, self *Instance) (messageAddress, error) {
	separator := "::"
	if send.TargetPath {
		separator = "."
	}
	segments := strings.Split(send.Target, separator)
	if !ctx.routedReceiverExists(send.Scope, segments, len(segments) > 1, self) {
		return messageAddress{}, fmt.Errorf("receiver %q is unresolved", send.Target)
	}
	addrs, err := ctx.resolveAddresses(send, self)
	if err != nil {
		return messageAddress{}, err
	}
	// A routed receiver is a node of the sending object, so of the addresses
	// resolved the one held to that object — or to none — is the receiver's.
	for _, addr := range addrs {
		if addr.Delivery == DeliverReceiver && (addr.Object == 0 || addr.Object == objectID(self)) {
			return addr, nil
		}
	}
	for _, addr := range addrs {
		if addr.Delivery == DeliverReceiver {
			return addr, nil
		}
	}
	return messageAddress{}, fmt.Errorf("receiver %q is not a receiving node", send.Target)
}

// routedReceiverExists prefers a directly declared receiving node over inherited
// feature names, then checks behavior features of the object being addressed.
func (ctx *Context) routedReceiverExists(scope *symbols.Scope, segments []string, path bool, self *Instance) bool {
	if len(segments) == 0 || segments[0] == "" {
		return false
	}
	if path {
		sym, ok := ctx.pathSymbol(scope, segments)
		return ok && isRoutedReceiverSymbol(sym)
	}
	name := segments[0]
	for current := scope; current != nil; {
		for _, sym := range symbols.PreferDeclared(current.LookupLocalAll(name)) {
			if isRoutedReceiverSymbol(sym) {
				return true
			}
		}
		parent := current.Parent()
		if parent == nil || !isRoutedReceiverSymbol(parent.Owner()) {
			break
		}
		current = parent
	}
	if self == nil {
		return false
	}
	for _, of := range ctx.FeaturesOfObject(self) {
		if of.Name == name && isRoutedReceiverSymbol(of.Feature.Symbol) {
			return true
		}
	}
	return false
}

// isRoutedReceiverSymbol accepts only actions and states as routed receivers,
// leaving other named members out of receiver address resolution.
func isRoutedReceiverSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolActionDef, symbols.SymbolActionUsage,
		symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return true
	default:
		return false
	}
}

// messageAddress is where an addressed send delivers: what the address resolved
// to, and the object, port or receiving node naming it. Only the constructors
// below build one, so no address can name a destination in part.
type messageAddress struct {
	Delivery DeliveryKind
	Name     string
	Port     string
	Object   int64
}

// portAddress is a port of an object, refused where no port was resolved.
func portAddress(port string, object int64) (messageAddress, bool) {
	if port == "" {
		return messageAddress{}, false
	}
	return messageAddress{Delivery: DeliverPort, Port: port, Object: object}, true
}

// receiverAddress is a receiving node of an object, refused where it is unnamed.
func receiverAddress(name string, object int64) (messageAddress, bool) {
	if name == "" {
		return messageAddress{}, false
	}
	return messageAddress{Delivery: DeliverReceiver, Name: name, Object: object}, true
}

// objectAddress is an object itself, refused where no object was reached: a
// destination confined to object 0 would confine the message to nothing.
func objectAddress(object int64) (messageAddress, bool) {
	if object == 0 {
		return messageAddress{}, false
	}
	return messageAddress{Delivery: DeliverObject, Object: object}, true
}

// postTo delivers an addressed send to every object its target resolves to,
// one copy per address, each held to that object's own identity.
func (ctx *Context) postTo(msg Message, send lower.Send, self *Instance) error {
	addrs, err := ctx.resolveAddresses(send, self)
	if err != nil {
		return err
	}
	copies := make([]Message, 0, len(addrs))
	for _, addr := range addrs {
		copied := msg
		copied.Target, copied.Port, copied.Object = addr.Name, addr.Port, addr.Object
		copied.Delivery = addr.Delivery
		if addr.Delivery == DeliverPort {
			copied.PortID, err = ctx.portInstanceID(ctx.instances[addr.Object], addr.Port)
			if err != nil {
				return err
			}
		}
		copies = append(copies, copied)
	}
	for _, copied := range copies {
		ctx.PostMessage(copied)
	}
	return nil
}

// resolveAddresses answers what a `send m to t` addressed: the objects t
// belongs to and the port path within them, resolved through the instance
// graph — several where the target reaches through a multi-valued feature. A
// chain the graph does not reach is a port of the sender itself, and
// unroutable if it is neither; a name reaching neither is the receiving node
// of that name.
func (ctx *Context) resolveAddresses(send lower.Send, self *Instance) ([]messageAddress, error) {
	if send.Target == "" {
		// A send addressing no one is for the sending object, or for whoever accepts
		// it where no object sent it.
		if addr, ok := objectAddress(objectID(self)); ok {
			return []messageAddress{addr}, nil
		}
		return []messageAddress{{Delivery: DeliverAnyone}}, nil
	}
	if !send.TargetPath {
		return ctx.namedAddresses(send, self)
	}
	segments := strings.Split(send.Target, ".")
	addrs, err := ctx.featureAddresses(send.Scope, self, segments)
	if err != nil {
		return nil, err
	}
	if len(addrs) > 0 {
		return addrs, nil
	}
	if sym, ok := ctx.portSymbol(send.Scope, send.Target); ok &&
		sym.Kind == symbols.SymbolPortUsage && ctx.ownPortPath(send.Scope, segments) {
		if addr, ok := portAddress(send.Target, objectID(self)); ok {
			return []messageAddress{addr}, nil
		}
	}
	return nil, &UnroutableSendError{Port: send.Target, Address: true}
}

// namedAddresses resolves a target named rather than chained (`R`, `P::R`): an
// unqualified name is a feature, port or receiving node of the sending object,
// and a qualified one is the element its path names, never a same-named element
// of the sender.
func (ctx *Context) namedAddresses(send lower.Send, self *Instance) ([]messageAddress, error) {
	segments := strings.Split(send.Target, "::")
	name := segments[len(segments)-1]
	if len(segments) > 1 {
		return ctx.qualifiedAddresses(send, self, segments)
	}
	addrs, err := ctx.featureAddresses(send.Scope, self, segments)
	if err != nil {
		return nil, err
	}
	if len(addrs) > 0 {
		return addrs, nil
	}
	if sym, resolved := ctx.pathSymbol(send.Scope, segments); resolved &&
		sym.Kind == symbols.SymbolPortUsage {
		if addr, built := portAddress(name, objectID(self)); built {
			return []messageAddress{addr}, nil
		}
		return nil, &UnroutableSendError{Port: send.Target, Address: true}
	}
	if addr, built := receiverAddress(name, objectID(self)); built {
		return []messageAddress{addr}, nil
	}
	return nil, &UnroutableSendError{Port: send.Target, Address: true}
}

// qualifiedAddresses resolves a target naming a namespace path (`alpha::reader`,
// `P::Driver`) to the element that path names, never to a same-named feature of
// the sender: the qualifier chooses the object, so the address is the occurrence
// the path leads through, or unroutable where this run reaches none.
func (ctx *Context) qualifiedAddresses(send lower.Send, self *Instance, segments []string) ([]messageAddress, error) {
	addrs, err := ctx.featureAddresses(send.Scope, nil, segments)
	if err != nil {
		return nil, err
	}
	if len(addrs) > 0 {
		return addrs, nil
	}
	target, resolved := ctx.pathSymbol(send.Scope, segments)
	if !resolved {
		return nil, &UnroutableSendError{Port: send.Target, Address: true}
	}
	name := segments[len(segments)-1]
	// A path leading through no occurrence names an element of the sending
	// behavior's own namespace, where its bare name is that same element:
	// `P::Node::Machine` is the sender's machine, `P::alpha::inPort` is alpha's.
	if local, ok := ctx.pathSymbol(send.Scope, []string{name}); ok && local == target {
		addrs, err := ctx.featureAddresses(send.Scope, self, []string{name})
		if err != nil {
			return nil, err
		}
		if len(addrs) > 0 {
			return addrs, nil
		}
		if target.Kind == symbols.SymbolPortUsage {
			if addr, built := portAddress(name, objectID(self)); built {
				return []messageAddress{addr}, nil
			}
		} else if addr, built := receiverAddress(name, objectID(self)); built {
			return []messageAddress{addr}, nil
		}
	}
	// A receiver no object owns has no identity of its own; a sender that has one
	// cannot address it by name without reaching its own same-named element.
	if self == nil && target.Kind != symbols.SymbolPortUsage {
		if addr, built := receiverAddress(name, 0); built {
			return []messageAddress{addr}, nil
		}
	}
	return nil, &UnroutableSendError{Port: send.Target, Address: true}
}

// featureAddresses walks a target through the instance graph from the object
// its first segment belongs to. A segment held as a collection denotes every
// element it holds (KerML §7.3.4.6), so the walk carries a set of objects and
// the target resolves to one address per object reached, without duplicates.
// No address is reported where a segment names no feature, or one that is
// neither a port nor an occurrence to descend into. A failure to read an
// object of the graph is that failure, not a bad address.
func (ctx *Context) featureAddresses(scope *symbols.Scope, self *Instance, segments []string) ([]messageAddress, error) {
	owner, rest, ok, err := ctx.addressOwner(scope, self, segments)
	if err != nil || !ok {
		return nil, err
	}
	owners := []*Instance{owner}
	var out []messageAddress
	seen := map[messageAddress]bool{}
	add := func(addr messageAddress, built bool) {
		if built && !seen[addr] {
			seen[addr] = true
			out = append(out, addr)
		}
	}
	for i, segment := range rest {
		var next []*Instance
		for _, owner := range owners {
			fv, held := owner.FeatureValues[segment]
			if !held {
				continue
			}
			if isPortFeature(fv.Feature) {
				add(portAddress(strings.Join(rest[i:], "."), owner.ID))
				continue
			}
			// A behavior of an object is a receiving node of it, addressed by name.
			if i == len(rest)-1 && isBehaviorFeature(fv.Feature) {
				add(receiverAddress(segment, owner.ID))
				continue
			}
			held2, err := ctx.fvObjects(owner, segment)
			if err != nil {
				return nil, err
			}
			next = append(next, held2...)
		}
		owners = next
		if len(owners) == 0 {
			return out, nil
		}
	}
	for _, owner := range owners {
		add(objectAddress(owner.ID))
	}
	return out, nil
}

// addressOwner answers which object a target's leading segments belong to: the
// sending object, or one holding it, where the first names a feature of it, else the occurrence
// the shortest prefix names in the send's scope — a prefix rather than one name,
// since a namespace qualifies the occurrence in `P::alpha.inPort`.
func (ctx *Context) addressOwner(scope *symbols.Scope, self *Instance, segments []string) (*Instance, []string, bool, error) {
	// A name is a feature of the sending object, or of an object holding it: a
	// nested object addresses a sibling through the object they belong to.
	for up := self; up != nil; up = up.owner {
		if fv, held := up.FeatureValues[segments[0]]; held && ctx.namesFeature(scope, up, fv, segments[0]) {
			return up, segments, true, nil
		}
	}
	if scope == nil || ctx.resolver == nil {
		return nil, nil, false, nil
	}
	for n := 1; n <= len(segments); n++ {
		sym, ok := ctx.pathSymbol(scope, segments[:n])
		if !ok || !isOccurrenceUsage(sym) || !ctx.occursOnce(sym) {
			continue
		}
		if self != nil && self.Type == sym {
			return self, segments[n:], true, nil
		}
		// A target that names an object this run cannot build fails as that, rather
		// than being reported as an address naming nothing.
		inst, err := ctx.occurrenceOf(sym)
		if err != nil {
			return nil, nil, false, err
		}
		return inst, segments[n:], true, nil
	}
	return nil, nil, false, nil
}

// namesFeature reports whether a feature value of the sending object is what a name in
// the send's scope denotes: a nearer declaration, such as a node of the sending
// behavior, shadows the object's feature as name resolution has it.
func (ctx *Context) namesFeature(scope *symbols.Scope, self *Instance, fv *FeatureValue, name string) bool {
	sym, ok := ctx.pathSymbol(scope, []string{name})
	if !ok || (fv.Feature != nil && fv.Feature.Symbol == sym) {
		return true
	}
	for _, of := range ctx.FeaturesOfObject(self) {
		if of.Feature.Symbol == sym {
			return true
		}
	}
	return false
}

// ownPortPath reports whether a port path names a port of the sending behavior
// itself. A path led by a namespace or by an occurrence names a port of another
// object, which is unroutable where the instance graph did not reach it.
func (ctx *Context) ownPortPath(scope *symbols.Scope, segments []string) bool {
	sym, ok := ctx.pathSymbol(scope, segments[:1])
	if !ok || isOccurrenceUsage(sym) {
		return false
	}
	return sym.Kind != symbols.SymbolPackage && sym.Kind != symbols.SymbolNamespace
}

// fvObject reads the object a feature of inst holds as its scalar value,
// materializing it, and reports whether the feature holds one at all. A feature
// value that cannot be read is that failure rather than a feature holding no object.
func (ctx *Context) fvObject(inst *Instance, name string) (*Instance, bool, error) {
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		return nil, false, err
	}
	if fv == nil || fv.Value.Kind != ValInstance {
		return nil, false, nil
	}
	held, ok := ctx.instances[fv.Value.Instance]
	return held, ok, nil
}

// fvObjects reads every object a feature of inst holds, materializing it: the
// one its scalar value names, or each element of its collection in order.
func (ctx *Context) fvObjects(inst *Instance, name string) ([]*Instance, error) {
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil || fv == nil {
		return nil, err
	}
	var out []*Instance
	for _, v := range heldElements(fv.HeldValue()) {
		if v.Kind != ValInstance {
			continue
		}
		if held, ok := ctx.instances[v.Instance]; ok {
			out = append(out, held)
		}
	}
	return out, nil
}

// heldElements flattens a held value to the values it holds: the elements of a
// collection, or the value itself.
func heldElements(v Value) []Value {
	switch v.Kind {
	case ValSequence:
		if v.Sequence() == nil {
			return nil
		}
		return v.Sequence().Elements()
	case ValSet:
		if v.Set() == nil {
			return nil
		}
		return v.Set().Elements()
	}
	return []Value{v}
}

// isPortFeature reports whether a feature is a port, where an address stops: the
// rest of its path is the port path within the object.
func isPortFeature(feature *EffectiveFeature) bool {
	return feature != nil && feature.Symbol != nil && feature.Symbol.Kind == symbols.SymbolPortUsage
}

// isBehaviorFeature reports whether a feature is a behavior an object performs,
// which receives by name rather than being an object of its own.
func isBehaviorFeature(feature *EffectiveFeature) bool {
	return feature != nil && isBehaviorSymbol(feature.Symbol)
}

// isBehaviorSymbol reports whether sym declares a behavior an object performs.
func isBehaviorSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolActionUsage, symbols.SymbolStateUsage:
		return true
	}
	return false
}

// send builds and posts the message a send statement describes; a message the
// send cannot build or deliver leaves nothing building it created behind.
func (ctx *Context) send(ec *EvalContext, scope *symbols.Scope, conns []lower.Connection, s lower.Send, self *Instance) error {
	mark, attached := len(ctx.created), len(ctx.objectBehaviors)
	msg, err := ec.buildMessage(scope, s)
	if err != nil {
		ctx.abandonCreationSince(mark, attached)
		return err
	}
	built, started := len(ctx.created), len(ctx.objectBehaviors)
	if err := ctx.post(conns, msg, s, self); err != nil {
		ctx.abandonCreationBetween(mark, built, attached, started)
		return err
	}
	return nil
}

// post delivers a built message the way the send addressed it: routed through
// the connections of the sending port, or straight onto the bus. self is the
// object performing the behavior that sent it, nil for a behavior no object
// performs.
func (ctx *Context) post(conns []lower.Connection, msg Message, send lower.Send, self *Instance) error {
	if send.IsVia {
		return ctx.postVia(conns, msg, send, self)
	}
	return ctx.postTo(msg, send, self)
}

// carriesEvent reports whether m was sent from the event feature an accept
// subsets (`accept :> left.alert`), as evaluated in ec: the same declaration
// once it resolves, so a typed message of a same-named type never satisfies it,
// and the same occurrence where the message holds one, so a sibling part's event
// or an accept path evaluating to none never does; by name where it does not resolve.
func (ec *EvalContext) carriesEvent(m Message, subsets ast.Node) bool {
	path := lower.FeaturePath(subsets)
	if path == "" {
		return true
	}
	want, ok := ec.ctx.featureSymbol(ec.scope, path)
	if !ok {
		name := lastSegment(path)
		return name == m.EventName || name == m.SignalType
	}
	if m.Event == nil || want != m.Event {
		return false
	}
	if m.EventObject == 0 {
		return true
	}
	occurrence, ok := ec.eventOccurrence(subsets, want)
	return ok && occurrence == m.EventObject
}

// eventOccurrence is the occurrence an accept's event path names in ec: what the
// feature holds, or the object performing a behavioral feature (`left.alert`, `alert`).
func (ec *EvalContext) eventOccurrence(subsets ast.Node, event *symbols.Symbol) (int64, bool) {
	if isBehaviorSymbol(event) {
		chain, ok := subsets.(*ast.FeatureChainExpr)
		if !ok {
			return objectID(ec.self), ec.self != nil
		}
		subsets = chain.Operand
	}
	value, err := ec.eval(subsets)
	if err != nil || value.Kind != ValInstance {
		return 0, false
	}
	return value.Instance, true
}

// lastSegment is the feature a dotted path ends at.
func lastSegment(path string) string {
	return path[strings.LastIndex(path, ".")+1:]
}

// featureSymbol resolves a dotted feature path written in scope to the feature
// it names, through an alias.
func (ctx *Context) featureSymbol(scope *symbols.Scope, path string) (*symbols.Symbol, bool) {
	segments := strings.Split(path, ".")
	if ctx.resolver == nil || (ctx.model == nil && len(segments) > 1) {
		return nil, false
	}
	sym, ok := ctx.pathSymbol(scope, segments)
	if !ok {
		return nil, false
	}
	return ctx.resolver.AliasedElement(sym), true
}

// messageMatches reports whether a message satisfies an accept whose parameter
// is typed as want, written in scope: the message's type must conform to the
// definition want resolves to, so a subtype message satisfies a supertype
// accept and same-named definitions of different packages stay apart. Where
// either side resolves to no symbol, the written names are compared instead.
func (ctx *Context) messageMatches(m Message, want *ast.QualifiedName, scope *symbols.Scope) bool {
	if want == nil || len(want.Parts) == 0 {
		return true
	}
	if m.Signal != nil && ctx.model != nil {
		if wantSym := ctx.resolveTypeRef(scope, want); wantSym != nil {
			return ctx.conforms(m.Signal, wantSym)
		}
	}
	return m.SignalType == want.Parts[len(want.Parts)-1].Text
}

// buildMessage evaluates a send statement into a message.
//
// `send Ping to m` (a type) sends it with no payload; `send new Ping(3) to m`
// sends it carrying the arguments; any other expression's value is sent as `value`.
//
// A via send keeps a receiver target when one was stated; postVia fills in the
// reached port and final delivery kind.
//
// Names resolve in the send's declaring scope, which sees what a nested block
// imports; scope is the fallback where the lowered send records none.
func (e *EvalContext) buildMessage(scope *symbols.Scope, send lower.Send) (Message, error) {
	if send.Scope != nil {
		scope = send.Scope
	}
	target := send.Target
	if send.IsVia {
		target = send.Receiver
	}
	if sym, ok := e.namedType(scope, send.Message); ok {
		return Message{SignalType: sym.Name, Signal: sym, Target: target, Payload: map[string]Value{}}, nil
	}
	if constructor, ok := send.Message.(*ast.ConstructorExpr); ok {
		return e.buildConstructedMessage(scope, constructor, target)
	}

	// `send shutDown() to self` sends the invoked behavioral feature carrying its
	// arguments, never calling it; a calculation is called and its value sent.
	if invocation, ok := send.Message.(*ast.InvocationExpr); ok && !e.invokesCalc(scope, invocation) {
		msg, err := e.buildInvokedMessage(scope, invocation, target)
		if err != nil {
			return Message{}, err
		}
		msg.Event, msg.EventName = e.sentFeature(scope, invocation.Type)
		if isBehaviorSymbol(msg.Event) {
			msg.EventObject = objectID(e.self)
		}
		return msg, nil
	}

	value, err := e.Eval(send.Message)
	if err != nil {
		return Message{}, fmt.Errorf("eval send message: %w", err)
	}
	signalType := valueTypeName(value)
	var signal *symbols.Symbol
	if signalType == "" && value.Kind == ValInstance {
		signal = e.ctx.objectSignalSymbol(value.Instance)
		if signal != nil {
			signalType = signal.Name
		}
	}
	if signalType == "" {
		return Message{}, fmt.Errorf("send: message of kind %v has no signal type", value.Kind)
	}
	event, eventName := e.sentFeature(scope, send.Message)
	msg := Message{
		SignalType: signalType,
		Signal:     signal,
		Event:      event,
		EventName:  eventName,
		Target:     target,
		Value:      &value,
	}
	if event != nil && value.Kind == ValInstance {
		msg.EventObject = value.Instance
	}
	return msg, nil
}

// sentFeature is the feature a send reads its message from (`send a.b via p`)
// and its written name; nil where unresolved, both empty for other messages.
func (e *EvalContext) sentFeature(scope *symbols.Scope, message ast.Node) (*symbols.Symbol, string) {
	path := lower.FeaturePath(message)
	if path == "" {
		return nil, ""
	}
	name := lastSegment(path)
	if scope == nil || e.ctx == nil {
		return nil, name
	}
	sym, ok := e.ctx.featureSymbol(scope, path)
	if !ok {
		return nil, name
	}
	return sym, name
}

// acceptedValue is the value an accept binds its payload name to: the single
// value the message carries, or an occurrence of its signal built from the
// arguments the send named, so `accept p : Ping` sees a Ping object either way.
// The occurrence is kept on the message: a guard evaluated during transition
// selection and the firing that follows read the same object.
func (ctx *Context) acceptedValue(msg *Message) (Value, error) {
	if msg.Value != nil {
		return *msg.Value, nil
	}
	if msg.Signal == nil {
		return Value{}, fmt.Errorf("%w: %s carries no single value to bind",
			ErrNoValue, orAnonymousSignal(msg.SignalType))
	}
	value, err := ctx.materializeAccepted(*msg)
	if err != nil {
		return Value{}, err
	}
	msg.Value = &value
	return value, nil
}

// materializeAccepted builds the occurrence an accept binds a typed message as.
func (ctx *Context) materializeAccepted(msg Message) (Value, error) {
	value, err := ctx.materializeMessage(msg)
	if err != nil {
		return Value{}, fmt.Errorf("accepted %s: %w", msg.SignalType, err)
	}
	return value, nil
}

// materializeMessage builds the occurrence a typed message carries, leaving no
// instance behind when a payload entry names no feature of it, does not fit
// one, or two entries name one feature.
func (ctx *Context) materializeMessage(msg Message) (Value, error) {
	mark := len(ctx.created)
	inst, err := ctx.materialize(msg.Signal, 0, nil, "")
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return Value{}, fmt.Errorf("materialize: %w", err)
	}
	names := make([]string, 0, len(msg.Payload))
	for name := range msg.Payload {
		names = append(names, name)
	}
	sort.Strings(names)
	written := make(map[*FeatureValue]string, len(names))
	for _, name := range names {
		fv, held := inst.FeatureValues[name]
		if !held {
			ctx.abandonInstancesSince(mark)
			return Value{}, fmt.Errorf("%q names no feature it carries", name)
		}
		if fv != nil {
			if earlier, twice := written[fv]; twice {
				ctx.abandonInstancesSince(mark)
				return Value{}, fmt.Errorf("%s and %s are one feature, bound twice", earlier, name)
			}
			written[fv] = name
		}
		if err := inst.SetFeatureValue(ctx, name, msg.Payload[name]); err != nil {
			ctx.abandonInstancesSince(mark)
			return Value{}, err
		}
	}
	return Value{Kind: ValInstance, Instance: inst.ID}, nil
}

// invokesCalc reports whether an invocation calls a calculation — the declaration
// evaluating it would select — rather than naming a signal to send.
func (e *EvalContext) invokesCalc(scope *symbols.Scope, invocation *ast.InvocationExpr) bool {
	if invocation.Type == nil {
		return false
	}
	if e.ctx == nil || e.ctx.resolver == nil || e.ctx.model == nil || scope == nil {
		return false
	}
	sel := passes.SelectInvocation(e.ctx.resolver, e.ctx.model, scope, invocation, semantics.PerformsBehavior)
	if sel.Ambiguous {
		return true
	}
	return e.ctx.model.Evaluates(sel.Called())
}

// buildInvokedMessage builds the message of `send shutDown(7) to self`: the
// invoked behavioral feature types it and the arguments are its payload, a lone
// positional one also as `value`, which an accept binds its parameter to.
func (e *EvalContext) buildInvokedMessage(scope *symbols.Scope, invocation *ast.InvocationExpr, target string) (Message, error) {
	if invocation.Operand != nil {
		return Message{}, fmt.Errorf("send %s: a message is not sent through a receiver", ast.SimpleName(invocation.Type))
	}
	signalType, signal, err := e.messageType(scope, invocation.Type)
	if err != nil {
		return Message{}, err
	}
	return e.buildTypedMessage(scope, signalType, signal, target,
		messageArgs{typeRef: invocation.Type, args: invocation.Args, named: invocation.NamedArgs, loneValue: true})
}

// buildConstructedMessage builds the message of `send new Telemetry(3) via
// antenna`: the constructed definition types it and the arguments bind its
// features, so an accept binds a Telemetry whose first feature is 3, never the 3.
// The occurrence is constructed at the send, so an argument no feature admits, or
// one beyond the features, is rejected there whether or not an accept consumes it.
func (e *EvalContext) buildConstructedMessage(scope *symbols.Scope, constructor *ast.ConstructorExpr, target string) (Message, error) {
	signal, err := e.constructedType(scope, constructor.Type, "send")
	if err != nil {
		return Message{}, err
	}
	if err := e.checkConstructorArity(signal, constructor, "send "+signal.Name); err != nil {
		return Message{}, err
	}
	msg, err := e.buildTypedMessage(scope, signal.Name, signal, target,
		messageArgs{typeRef: constructor.Type, args: constructor.Args, named: constructor.NamedArgs})
	if err != nil {
		return Message{}, err
	}
	value, err := e.ctx.materializeMessage(msg)
	if err != nil {
		return Message{}, fmt.Errorf("send new %s: %w", signal.Name, err)
	}
	msg.Value = &value
	return msg, nil
}

// evalConstructor evaluates `new T(…)` as a value: the object of T whose
// constructible features the arguments bind, read as what it denotes.
func (e *EvalContext) evalConstructor(constructor *ast.ConstructorExpr) (Value, error) {
	typ, err := e.constructedType(e.scope, constructor.Type, "")
	if err != nil {
		return Value{}, err
	}
	what := "new " + typ.Name
	if err := e.checkConstructorArity(typ, constructor, what); err != nil {
		return Value{}, err
	}
	msg, err := e.buildTypedMessage(e.scope, typ.Name, typ, "",
		messageArgs{typeRef: constructor.Type, args: constructor.Args, named: constructor.NamedArgs, written: what})
	if err != nil {
		return Value{}, err
	}
	value, err := e.ctx.materializeMessage(msg)
	if err != nil {
		return Value{}, fmt.Errorf("%s: %w", what, err)
	}
	return e.ctx.objectValue(e.ctx.instances[value.Instance])
}

// checkConstructorArity rejects positional arguments beyond the constructed
// type's constructible features.
func (e *EvalContext) checkConstructorArity(typ *symbols.Symbol, constructor *ast.ConstructorExpr, what string) error {
	if e.ctx.model == nil {
		return nil
	}
	if n := len(e.ctx.model.ConstructibleFeatures(typ)); len(constructor.Args) > n {
		return fmt.Errorf("%s: new %s takes %d argument(s), found %d", what, typ.Name, n, len(constructor.Args))
	}
	return nil
}

// constructedType resolves the type `new T(…)` instantiates: a definition, or a
// usage, which is a type too. A name that resolves to nothing or to no type is
// an error, reported under the statement (`send`) the constructor is written in.
func (e *EvalContext) constructedType(scope *symbols.Scope, typeRef *ast.QualifiedName, statement string) (*symbols.Symbol, error) {
	name := ast.QualifiedText(typeRef)
	prefix := "new"
	if statement != "" {
		prefix = statement + " new"
	}
	if name == "" {
		return nil, fmt.Errorf("%s: the constructor names no type", prefix)
	}
	if scope == nil || e.ctx == nil || e.ctx.resolver == nil {
		return nil, fmt.Errorf("%s %s: no scope resolves the type", prefix, name)
	}
	sym, ok := e.ctx.resolver.ResolveQualified(scope, typeRef)
	if !ok || sym == nil {
		return nil, fmt.Errorf("%s %s: unresolved reference: %s", prefix, name, name)
	}
	switch sym.Decl.(type) {
	case *ast.Definition, *ast.Usage:
		return sym, nil
	}
	return nil, fmt.Errorf("%s %s: %s is a %s, not a type", prefix, name, name, sym.Kind)
}

// messageType names the type of an invoked message and resolves it to the
// definition it reaches, nil when it reaches none.
func (e *EvalContext) messageType(scope *symbols.Scope, typeRef *ast.QualifiedName) (string, *symbols.Symbol, error) {
	signalType := ast.SimpleName(typeRef)
	if signalType == "" {
		return "", nil, fmt.Errorf("send: the message names no signal")
	}
	signal, ok := e.definitionNamed(scope, typeRef)
	if !ok {
		return signalType, nil, nil
	}
	return signal.Name, signal, nil
}

// buildTypedMessage builds a message typed by signal, named typeRef, carrying each
// argument under the feature it binds: a positional one the feature at its position
// (`argN` where the type has none), a label the feature it names. Binding one feature
// twice, by position and label or by two labels, is an error rather than the last
// value. With loneValue a lone positional argument is also the message's Value.
// Errors name the construct as written, `send T` unless stated otherwise.
type messageArgs struct {
	typeRef   *ast.QualifiedName
	args      []ast.Node
	named     []ast.NamedArg
	loneValue bool
	written   string
}

func (e *EvalContext) buildTypedMessage(scope *symbols.Scope, signalType string, signal *symbols.Symbol, target string, in messageArgs) (Message, error) {
	typeRef, args, named, loneValue := in.typeRef, in.args, in.named, in.loneValue
	written := in.written
	if written == "" {
		written = "send " + signalType
	}
	msg := Message{SignalType: signalType, Signal: signal, Target: target,
		Payload: make(map[string]Value, len(args)+len(named))}
	var slots []*symbols.Symbol
	if signal != nil && e.ctx != nil && e.ctx.model != nil {
		slots = e.ctx.model.ConstructibleFeatures(signal)
	}
	bound := make(map[*symbols.Symbol]string, len(args)+len(named))
	for i, arg := range args {
		value, err := e.Eval(arg)
		if err != nil {
			return Message{}, fmt.Errorf("eval argument %d of %s: %w", i+1, written, err)
		}
		name := fmt.Sprintf("arg%d", i+1)
		if i < len(slots) {
			name = slots[i].Name
			bound[slots[i]] = name
		}
		msg.Payload[name] = value
		if loneValue && len(args) == 1 && len(named) == 0 {
			msg.Value = &value
		}
	}
	for _, arg := range named {
		label := ast.QualifiedText(arg.Name)
		if label == "" {
			return Message{}, fmt.Errorf("%s: an argument is named by nothing", written)
		}
		name, err := e.constructorLabel(scope, signal, typeRef, arg.Name, bound)
		if err != nil {
			return Message{}, fmt.Errorf("%s: %w", written, err)
		}
		if _, twice := msg.Payload[name]; twice {
			return Message{}, fmt.Errorf("%s: %s is bound twice", written, label)
		}
		value, err := e.Eval(arg.Value)
		if err != nil {
			return Message{}, fmt.Errorf("eval argument %s of %s: %w", label, written, err)
		}
		msg.Payload[name] = value
	}
	return msg, nil
}

// constructorLabel returns the shape name of the member of signal a constructor
// label names (resolved as the checker does); a foreign, masked or rebound feature is an error.
func (e *EvalContext) constructorLabel(scope *symbols.Scope, signal *symbols.Symbol, typeRef, qn *ast.QualifiedName, bound map[*symbols.Symbol]string) (string, error) {
	label := ast.QualifiedText(qn)
	if signal == nil || e.ctx == nil || e.ctx.resolver == nil || e.ctx.model == nil {
		if len(qn.Parts) != 1 {
			return "", fmt.Errorf("%s is not a feature of %s", label, ast.SimpleName(typeRef))
		}
		return qn.Parts[0].Text, nil
	}
	feature, ok := e.ctx.resolver.ResolveReference(resolve.Reference{Scope: scope, QN: qn, Constructed: typeRef})
	if !ok || feature == nil || !slices.Contains(e.ctx.model.MembersOf(signal), feature) {
		return "", fmt.Errorf("%s is not a feature of %s", label, signal.Name)
	}
	shape := e.ctx.model.ShapeFeatures(signal)
	i := slices.IndexFunc(shape, func(f semantics.ShapeFeature) bool {
		return f.Declared == feature || f.Symbol == feature
	})
	if i < 0 {
		return "", fmt.Errorf("%s is not a feature of %s", label, signal.Name)
	}
	slot := e.ctx.model.ConstructibleFeatureFor(signal, shape[i].Declared)
	if slot == nil {
		return "", fmt.Errorf("%s is not a feature a constructor of %s binds", label, signal.Name)
	}
	name := shape[i].Name
	if earlier, twice := bound[slot]; twice {
		if earlier == name {
			return "", fmt.Errorf("%s is bound twice", name)
		}
		return "", fmt.Errorf("%s and %s are one feature, bound twice", earlier, name)
	}
	bound[slot] = name
	return name, nil
}

// triggerName describes a transition's trigger for traces. Traces are compared
// against goldens, so the text has to be stable: printing the trigger node
// itself emits a pointer address.
func triggerName(trigger ast.Node) string {
	switch t := trigger.(type) {
	case nil:
		return ""
	case *ast.AcceptEvent:
		return "accept " + orAny(ast.SimpleName(t.SignalType))
	case *ast.CallEvent:
		return "call " + orAny(ast.SimpleName(t.Operation))
	case *ast.TimeEvent:
		return "time"
	case *ast.ChangeEvent:
		return "change"
	default:
		return fmt.Sprintf("%T", trigger)
	}
}

// eventName names a dispatched occurrence as triggerName names the triggers it
// matches, from the occurrence itself so the name is the same whichever took it.
// A message sent from an event feature is that feature's occurrence, named as
// the accept subsetting it is written, so same-typed events stay apart.
func eventName(event *Event) string {
	switch payload := event.Payload.(type) {
	case Message:
		if payload.EventName != "" {
			return "accept :> " + payload.EventName
		}
		return "accept " + orAny(payload.SignalType)
	case Call:
		return "call " + orAny(payload.Operation)
	}
	switch event.Type {
	case EventTime:
		return "time"
	case EventChange:
		return "change"
	default:
		return event.Type.String()
	}
}

// triggerDescription describes the event an accept waits for in the notation it
// was written in, which is what an error about it, or a view of a suspended run,
// has to name. The expression a trigger waits on is named when it is a name;
// there is no printer for an arbitrary one, so the keyword alone stands for it.
func triggerDescription(trigger ast.Node) string {
	switch t := trigger.(type) {
	case *ast.TimeEvent:
		keyword := "after"
		if t.Absolute {
			keyword = "at"
		}
		return joinWords("accept", keyword, ast.SimpleName(t.Duration))
	case *ast.ChangeEvent:
		return joinWords("accept", "when", ast.SimpleName(t.Condition))
	default:
		return triggerName(trigger)
	}
}

// joinWords joins the words of a description, dropping the ones that are empty.
func joinWords(words ...string) string {
	kept := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

// viaSuffix describes the port an accept waits on, for an error message, or
// nothing when it waits on none.
func viaSuffix(port string) string {
	if port == "" {
		return ""
	}
	return " via " + port
}

// orAny names a type or operation, or reports that any is accepted when the
// model named none.
func orAny(name string) string {
	if name == "" {
		return "any"
	}
	return name
}

// namedType reports the definition expr names when it names a type definition
// rather than denoting a value.
func (e *EvalContext) namedType(scope *symbols.Scope, expr ast.Node) (*symbols.Symbol, bool) {
	return e.definitionNamed(scope, ast.AsQualifiedName(expr))
}

// definitionNamed resolves a name to the definition it reaches (resolution
// already follows an alias to its element) or reports that it names none.
func (e *EvalContext) definitionNamed(scope *symbols.Scope, qname *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qname == nil || scope == nil || e.ctx == nil || e.ctx.resolver == nil {
		return nil, false
	}
	sym, ok := e.ctx.resolver.ResolveQualified(scope, qname)
	if !ok || sym == nil {
		return nil, false
	}
	if !isDefinitionSymbol(sym) {
		return nil, false
	}
	return sym, true
}

// objectSignalSymbol is the definition an object sent as a message
// materializes, which is the type an accept of it matches by conformance.
func (ctx *Context) objectSignalSymbol(id int64) *symbols.Symbol {
	inst, ok := ctx.instances[id]
	if !ok || inst == nil || ctx.model == nil {
		return nil
	}
	if isDefinitionSymbol(inst.Type) {
		return inst.Type
	}
	for _, sup := range ctx.model.AllSupertypes(inst.Type) {
		if isDefinitionSymbol(sup) {
			return sup
		}
	}
	return nil
}

// isDefinitionSymbol reports whether a symbol declares a definition, not a usage.
func isDefinitionSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	_, isDef := sym.Decl.(*ast.Definition)
	return isDef
}

// valueTypeName names the type of a value, as a send statement's signal type.
func valueTypeName(v Value) string {
	switch v.Kind {
	case ValString:
		return "String"
	case ValConst:
		switch v.Const.Kind {
		case semantics.ValInt:
			return "Integer"
		case semantics.ValReal:
			return "Real"
		case semantics.ValBool:
			return "Boolean"
		}
	}
	return ""
}
