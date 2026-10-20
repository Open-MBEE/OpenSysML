package runtime

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const (
	transferFQN                    = "Transfers::Transfer"
	occurrenceIncomingTransfersFQN = occurrenceFQN + "::incomingTransfers"
	occurrenceOutgoingTransfersFQN = occurrenceFQN + "::outgoingTransfers"
)

// transferRecord is one Transfer performance the run completed: the object of
// Transfers::Transfer standing for it, the occurrences it left (the sending
// performance, object and port) and those it reached (the accepting
// performance, object and port), as Occurrences.kerml `outgoingTransfers` and
// `incomingTransfers` list them.
type transferRecord struct {
	id      int64
	source  int64 // the end feature `source`: the sending performance, else the sending object
	target  int64 // the end feature `target`: the accepting performance, else its object
	payload Value // what the transfer carried, none where the message carried nothing
	sources []int64
	targets []int64
}

// recordTransfer records a message an accept took as a Transfer from what sent
// it to the performance that accepted it: the transfer began when the message
// was sent and ended now, its source is the sending performance (else the
// sending object), its target the accepting performance, and its payload what
// the message carried. A message from outside the run has no source and
// records no transfer.
func (ctx *Context) recordTransfer(msg Message, receiver, receiverObject *Instance) error {
	if msg.Sender == 0 && msg.SenderPerformance == 0 {
		return nil
	}
	sym := ctx.librarySymbol(transferFQN)
	if sym == nil {
		return nil
	}
	transfer, err := ctx.materialize(sym, 0, nil, "")
	if err != nil {
		return fmt.Errorf("transfer of %s: %w", acceptedEventName(msg), err)
	}
	source := msg.SenderPerformance
	if source == 0 {
		source = msg.Sender
	}
	target := objectID(receiver)
	if target == 0 {
		target = objectID(receiverObject)
	}
	prior := ctx.lives[transfer.ID]
	ended := ctx.newActivation()
	ctx.lives[transfer.ID] = life{reached: prior.reached, began: prior.began, ended: ended, beganAt: msg.SentAt, endedAt: ctx.clock.Now()}
	ctx.livesChanged()
	record := transferRecord{
		id:      transfer.ID,
		source:  source,
		target:  target,
		payload: acceptedPayload(msg),
		sources: nonzeroIDs(msg.SenderPerformance, msg.Sender, msg.SenderPort),
		targets: nonzeroIDs(objectID(receiver), objectID(receiverObject), msg.PortID),
	}
	before := len(ctx.transfers)
	ctx.transfers = append(ctx.transfers, record)
	ctx.noteProbeUndo(func() { ctx.transfers = ctx.transfers[:before] })
	return nil
}

// acceptedPayload is what a message carries as the transfer's payload: its
// value, else the arguments it was sent with; none where it carries nothing.
func acceptedPayload(msg Message) Value {
	if msg.Value != nil {
		return *msg.Value
	}
	if len(msg.Payload) == 0 {
		return Value{}
	}
	names := make([]string, 0, len(msg.Payload))
	for name := range msg.Payload {
		names = append(names, name)
	}
	slices.Sort(names)
	values := make([]Value, 0, len(names))
	for _, name := range names {
		values = append(values, msg.Payload[name])
	}
	if len(values) == 1 {
		return values[0]
	}
	return sequenceOf(values)
}

// nonzeroIDs keeps the object ids that name an object, each once, in order.
func nonzeroIDs(ids ...int64) []int64 {
	var out []int64
	for _, id := range ids {
		if id != 0 && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// transferValues answers a collection member of inst over the transfers the
// run completed: those that reached it (`incomingTransfers`) or left it
// (`outgoingTransfers`); false for a member that is neither.
func (ctx *Context) transferValues(inst *Instance, sym *symbols.Symbol) ([]Value, bool, error) {
	var ends func(transferRecord) []int64
	switch {
	case ctx.isLibraryMember(sym, occurrenceIncomingTransfersFQN):
		ends = func(r transferRecord) []int64 { return r.targets }
	case ctx.isLibraryMember(sym, occurrenceOutgoingTransfersFQN):
		ends = func(r transferRecord) []int64 { return r.sources }
	case ctx.isLibraryMember(sym, transferFQN+"::payload"):
		// ends stays nil: the payload is the transfer object's own.
	default:
		return nil, false, nil
	}
	var values []Value
	if ends == nil {
		for _, record := range ctx.transfers {
			if record.id == inst.ID && record.payload.Kind != ValInvalid {
				return elementsOf(record.payload), true, nil
			}
		}
		return nil, true, nil
	}
	for _, record := range ctx.transfers {
		if !slices.Contains(ends(record), inst.ID) {
			continue
		}
		val, err := ctx.objectValue(ctx.instances[record.id])
		if err != nil {
			return nil, true, err
		}
		values = append(values, val)
	}
	return values, true, nil
}

// recordedValues answers a collection member of inst from what the run recorded:
// the successions that fired or the transfers that completed; false for any other.
func (ctx *Context) recordedValues(inst *Instance, sym *symbols.Symbol) ([]Value, bool, error) {
	if values, ok, err := ctx.successionValues(inst, sym); ok {
		return values, ok, err
	}
	return ctx.transferValues(inst, sym)
}

// transferEndValue answers the end feature `source` or `target` of a Transfer
// object the run recorded (Transfers.kerml: the occurrences the transfer is
// from and to); false for any other read.
func (ctx *Context) transferEndValue(inst *Instance, sym *symbols.Symbol) (Value, bool, error) {
	var end func(transferRecord) int64
	switch {
	case ctx.isLibraryMember(sym, transferFQN+"::source"):
		end = func(r transferRecord) int64 { return r.source }
	case ctx.isLibraryMember(sym, transferFQN+"::target"):
		end = func(r transferRecord) int64 { return r.target }
	default:
		return Value{}, false, nil
	}
	for _, record := range ctx.transfers {
		if record.id == inst.ID && end(record) != 0 {
			val, err := ctx.objectValue(ctx.instances[end(record)])
			return val, true, err
		}
	}
	return Value{}, false, nil
}
