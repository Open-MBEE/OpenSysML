package runtime

import "testing"

// pendingAsk is a call under way with an output written, an inout carried in and
// an object among its values, as a capture must take it.
func pendingAsk(object *Instance) *pendingCall {
	return &pendingCall{
		id:      7,
		outputs: map[string]Value{"result": constInt(1), "who": objectValue(object)},
		returns: map[string]bool{"result": true, "who": true},
		inouts:  map[string]Value{"x": constInt(2)},
	}
}

// checkPendingAsk fails unless call is pendingAsk's call with its object at who.
func checkPendingAsk(t *testing.T, what string, call *pendingCall, who int64) {
	t.Helper()
	if call == nil {
		t.Fatalf("%s: no call pending, want the one captured", what)
	}
	if call.id != 7 || call.taken {
		t.Errorf("%s: call id %d taken %v, want 7 not taken", what, call.id, call.taken)
	}
	if got := call.outputs["result"]; got.Const.Int != 1 || len(call.outputs) != 2 {
		t.Errorf("%s: outputs %v, want result = 1 and who", what, call.outputs)
	}
	if got, _ := call.outputs["who"].Object(); got != who {
		t.Errorf("%s: who = #%d, want #%d", what, got, who)
	}
	if !call.returns["result"] || !call.returns["who"] || len(call.returns) != 2 {
		t.Errorf("%s: returns %v, want result and who", what, call.returns)
	}
	if got := call.inouts["x"]; got.Const.Int != 2 || len(call.inouts) != 1 {
		t.Errorf("%s: inouts %v, want x = 2", what, call.inouts)
	}
}

// A state executor's capture takes the call it is answering by value: what the
// executor writes to the call after the capture is undone by the restore, and a
// restored call does not share its maps with the capture.
func TestStateCaptureTakesThePendingCall(t *testing.T) {
	exec := callOwnerMachine(t)
	exec.pendingCall = pendingAsk(exec.self)
	capture := exec.capture()

	exec.pendingCall.outputs["result"] = constInt(9)
	exec.pendingCall.inouts["x"] = constInt(3)
	exec.pendingCall.taken = true
	exec.pendingCall = nil
	capture.restore()
	checkPendingAsk(t, "after the restore", exec.pendingCall, exec.self.ID)

	exec.pendingCall.outputs["result"] = constInt(5)
	exec.pendingCall.returns["extra"] = true
	capture.restore()
	checkPendingAsk(t, "after the second restore", exec.pendingCall, exec.self.ID)

	exec.pendingCall = nil
	if capture = exec.capture(); capture.pendingCall != nil {
		t.Errorf("a capture with no call pending holds %v", capture.pendingCall)
	}
}

// A held image carries the call a machine is answering: the copy's machine holds
// the same call with its values as the destination's own, apart from the source's.
func TestHeldImageCarriesThePendingCall(t *testing.T) {
	_, src, bulb := lampBulb(t)
	machine := lampMachine(t, bulb)
	machine.pendingCall = pendingAsk(bulb)

	dst := imageInto(t, src, bulb)
	copied, _ := dst.Instance(bulb.ID)
	imaged := lampMachine(t, copied)
	checkPendingAsk(t, "the copy", imaged.pendingCall, copied.ID)

	imaged.pendingCall.outputs["result"] = constInt(9)
	imaged.pendingCall.taken = true
	checkPendingAsk(t, "the source after the copy wrote", machine.pendingCall, bulb.ID)

	machine.pendingCall = nil
	plain := imageInto(t, src, bulb)
	plainCopy, _ := plain.Instance(bulb.ID)
	if call := lampMachine(t, plainCopy).pendingCall; call != nil {
		t.Errorf("a copy of a machine answering no call holds %v", call)
	}
}
