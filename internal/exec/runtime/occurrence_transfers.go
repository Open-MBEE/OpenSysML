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

// transferRecord is one Transfer performance of the run: its object, the
// occurrences it left (`outgoingTransfers`) and those it reached (`incomingTransfers`).
type transferRecord struct {
	id      int64
	source  int64 // the end feature `source`: the sending performance, else the sending object
	target  int64 // the end feature `target`: the accepting performance, else the receiving object
	payload Value // what the transfer carries, none where the message carries nothing
	sources []int64
	targets []int64
}

// beginTransfer records a posted message as a Transfer begun now, from the sending
// performance (else object) to the object it is bound for; one from outside records none.
func (ctx *Context) beginTransfer(msg *Message) error {
	if msg.Sender == 0 && msg.SenderPerformance == 0 {
		return nil
	}
	sym := ctx.librarySymbol(transferFQN)
	if sym == nil {
		return nil
	}
	transfer, err := ctx.materialize(sym, 0, nil, "")
	if err != nil {
		return fmt.Errorf("transfer of %s: %w", acceptedEventName(*msg), err)
	}
	source := msg.SenderPerformance
	if source == 0 {
		source = msg.Sender
	}
	prior := ctx.lives[transfer.ID]
	ctx.lives[transfer.ID] = life{reached: prior.reached, began: ctx.newActivation(), beganAt: msg.SentAt}
	record := transferRecord{
		id:      transfer.ID,
		source:  source,
		target:  msg.Object,
		payload: acceptedPayload(*msg),
		sources: nonzeroIDs(msg.SenderPerformance, msg.Sender, msg.SenderPort),
		targets: nonzeroIDs(msg.Object, msg.PortID),
	}
	before := len(ctx.transfers)
	ctx.transfers = append(ctx.transfers, record)
	ctx.noteProbeUndo(func() { ctx.transfers = ctx.transfers[:before] })
	ctx.livesChanged()
	msg.Transfer = transfer.ID
	return nil
}

// recordTransfer completes the Transfer of a message an accept took: it ends now
// and its target is the accepting performance (else its object).
func (ctx *Context) recordTransfer(msg Message, receiver, receiverObject *Instance) error {
	at := slices.IndexFunc(ctx.transfers, func(r transferRecord) bool { return r.id == msg.Transfer && msg.Transfer != 0 })
	if at < 0 {
		if err := ctx.beginTransfer(&msg); err != nil || msg.Transfer == 0 {
			return err
		}
		at = len(ctx.transfers) - 1
	}
	previous := ctx.transfers[at]
	record := previous
	record.sources, record.targets = slices.Clone(previous.sources), slices.Clone(previous.targets)
	if target := objectID(receiver); target != 0 {
		record.target = target
	} else if target := objectID(receiverObject); target != 0 {
		record.target = target
	}
	for _, id := range nonzeroIDs(objectID(receiver), objectID(receiverObject), msg.PortID) {
		if !slices.Contains(record.targets, id) {
			record.targets = append(record.targets, id)
		}
	}
	if record.payload.Kind == ValInvalid {
		record.payload = acceptedPayload(msg)
	}
	l := ctx.lives[record.id]
	l.ended, l.endedAt = ctx.newActivation(), ctx.clock.Now()
	ctx.lives[record.id] = l
	ctx.transfers[at] = record
	ctx.noteProbeUndo(func() { ctx.transfers[at] = previous })
	ctx.livesChanged()
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

// transferValues answers inst's `incomingTransfers` (the transfers that reached
// it) or `outgoingTransfers` (those that left it); false for a member that is neither.
func (ctx *Context) transferValues(inst *Instance, sym *symbols.Symbol) ([]Value, bool, error) {
	ctx.readsLives()
	var ends func(transferRecord) []int64
	switch {
	case ctx.isLibraryMember(sym, inst.Type, occurrenceIncomingTransfersFQN):
		ends = func(r transferRecord) []int64 { return r.targets }
	case ctx.isLibraryMember(sym, inst.Type, occurrenceOutgoingTransfersFQN):
		ends = func(r transferRecord) []int64 { return r.sources }
	case ctx.isLibraryMember(sym, inst.Type, transferFQN+"::payload"):
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

// recordedValues answers a collection member of inst from the time order, the
// successions that fired or the transfers; false for any other.
func (ctx *Context) recordedValues(inst *Instance, sym *symbols.Symbol) ([]Value, bool, error) {
	if values, ok, err := ctx.successionValues(inst, sym); ok {
		return values, ok, err
	}
	return ctx.transferValues(inst, sym)
}

// transferEndValue answers the end feature `source` or `target` of a recorded
// Transfer object (Transfers.kerml); false for any other read.
func (ctx *Context) transferEndValue(inst *Instance, sym *symbols.Symbol) (Value, bool, error) {
	ctx.readsLives()
	var end func(transferRecord) int64
	switch {
	case ctx.isLibraryMember(sym, inst.Type, transferFQN+"::source"):
		end = func(r transferRecord) int64 { return r.source }
	case ctx.isLibraryMember(sym, inst.Type, transferFQN+"::target"):
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
