package runtime

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func intConst(i int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}
}

// Streaming writes staged ahead of a target's performance are kept per source: a
// source's next write replaces its own earlier value however many other sources wrote
// in between, and taking a delivery shifts what every source left waiting.
func TestStagedStreamsAreKeptPerSource(t *testing.T) {
	target := &ast.Usage{NodeBase: ast.NodeBase{NodeSpan: source.Span{Offset: 10, Len: 1}}, Kind: ast.UsageAction}
	frame, a, b := &actionFrame{}, &actionFrame{}, &actionFrame{}
	queue := func() []int64 {
		var values []int64
		for _, v := range frame.pending[target]["v"] {
			values = append(values, v.Const.Int)
		}
		return values
	}
	appended := []bool{
		frame.stage(target, "v", a, intConst(1)),
		frame.stage(target, "v", b, intConst(2)),
		frame.stage(target, "v", a, intConst(3)),
		frame.stage(target, "v", b, intConst(4)),
	}
	if want := []bool{true, true, false, false}; !slices.Equal(appended, want) {
		t.Fatalf("appended = %v, want %v", appended, want)
	}
	if got := queue(); len(got) != 2 || got[0] != 3 || got[1] != 4 {
		t.Fatalf("queue = %v, want [3 4]: each source's latest value in its own place", got)
	}
	take := func() {
		frame.shiftStaged(target, "v")
		frame.pending[target]["v"] = frame.pending[target]["v"][1:]
	}
	take()
	if !frame.stage(target, "v", a, intConst(5)) {
		t.Fatalf("a's write after its value was taken was not appended")
	}
	if got := queue(); !slices.Equal(got, []int64{4, 5}) {
		t.Fatalf("queue = %v, want [4 5]: a's value gone with the delivery, its next write queued last", got)
	}
	take()
	if !frame.stage(target, "v", b, intConst(6)) || frame.stage(target, "v", a, intConst(7)) {
		t.Fatalf("after the second delivery, want b's write appended and a's replacing its own")
	}
	if got := queue(); !slices.Equal(got, []int64{7, 6}) {
		t.Fatalf("queue = %v, want [7 6]", got)
	}
}

const stagedStreamSource = `
	package test {
		part def Streamer {
			attribute got : Integer = 0;
			perform action run {
				first start;
				action producer {
					out value : Integer;
					action once { assign value := 1; }
					action heard accept g : Integer;
					action again { assign value := g; }
					succession first start then once;
					succession first once then heard;
					succession first heard then again;
					succession first again then done;
				}
				action consumer {
					in value : Integer;
					action take { assign got := value; }
					succession first start then take;
					succession first take then done;
				}
				done;
				succession first start then producer;
				succession first producer then consumer;
				succession first consumer then done;
				flow producer.value to consumer.value;
			}
		}
	}
`

// A held image carries what a streaming source staged ahead of its target: once the
// copy's source resumes and writes again, that write replaces the value imaged
// waiting for the consumer rather than queuing behind it.
func TestHeldImageCarriesStagedStreams(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, stagedStreamSource)
	src := NewContext(typedModel(model, resolver), 10000)
	streamer, err := src.Instantiate(resolveSymbol(t, resolveSymbol(t, root, "test").Scope, "Streamer"))
	if err != nil {
		t.Fatalf("Instantiate Streamer: %v", err)
	}
	run := func(t *testing.T, obj *Instance) *ActionExecutor {
		t.Helper()
		behavior, ok := obj.Behavior("run")
		if !ok || behavior.Action == nil {
			t.Fatalf("object #%d performs no run action, behaviors: %v", obj.ID, obj.Behaviors())
		}
		return behavior.Action
	}
	if got := run(t, streamer).State(); got != StateWaiting {
		t.Fatalf("the source is %v, want parked at its accept after the first write", got)
	}

	dst := imageInto(t, src, streamer)
	copied, _ := dst.Instance(streamer.ID)
	nine := intConst(9)
	dst.PostMessage(Message{SignalType: "Integer", Object: copied.ID, Value: &nine})
	if err := run(t, copied).RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion(copy): %v", err)
	}
	if got := run(t, copied).State(); got != StateCompleted {
		t.Fatalf("the copy is %v, want completed", got)
	}
	if got := featureInt(t, dst, copied, "got"); got != 9 {
		t.Errorf("the copy's consumer read %d, want 9: the resumed producer's write replaced the staged 1", got)
	}
}

// The outputs of a performance write to every listening node's pins as they are
// written, in listening order, and no longer once the node stops listening.
func TestOutputListenersTakeEachWrite(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, stagedStreamSource)
	ctx := NewContext(typedModel(model, resolver), 10000)
	streamer, err := ctx.Instantiate(resolveSymbol(t, resolveSymbol(t, root, "test").Scope, "Streamer"))
	if err != nil {
		t.Fatalf("Instantiate Streamer: %v", err)
	}
	behavior, _ := streamer.Behavior("run")
	exec := behavior.Action
	var heard []string
	record := func(tag string) func(string, Value) error {
		return func(name string, value Value) error {
			heard = append(heard, tag+":"+name+"="+FormatValue(value))
			return nil
		}
	}
	first, second := &actionFrame{}, &actionFrame{}
	exec.listen(first, record("first"))
	exec.listen(second, record("second"))
	exec.listen(first, record("first'"))
	if err := exec.setFeature("got", intConst(4)); err != nil {
		t.Fatalf("setFeature: %v", err)
	}
	exec.unlisten(second)
	if err := exec.setFeature("got", intConst(5)); err != nil {
		t.Fatalf("setFeature: %v", err)
	}
	exec.unlisten(first)
	if err := exec.setFeature("got", intConst(6)); err != nil {
		t.Fatalf("setFeature: %v", err)
	}
	want := []string{"second:got=4", "first':got=4", "first':got=5"}
	if !slices.Equal(heard, want) {
		t.Fatalf("heard = %v, want %v", heard, want)
	}
}
