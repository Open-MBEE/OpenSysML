package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessStreamingFlow exercises the failure modes of a plain `flow`,
// which streams each value its source writes to the performances of its target under way.
func TestRuntimeRobustnessStreamingFlow(t *testing.T) {
	t.Run("source_never_writes", testStreamingFlowSourceNeverWrites)
	t.Run("target_completed_before_source_writes", testStreamingFlowTargetCompletedFirst)
	t.Run("target_pin_not_declared", testStreamingFlowTargetPinNotDeclared)
	t.Run("later_performance_takes_one_of_two_late_values", testStreamingFlowOneLateValueLeft)
	t.Run("flows_form_a_cycle", testStreamingFlowCycle)
}

// streamingPair is an action performing producer and consumer side by side, the
// producer's `out value` flowing to the consumer's pin named by target.
func streamingPair(producerBody, target string) string {
	return `package test {
		action stream {
			attribute total : Integer = 0;
			fork split;
			action producer {
				out value : Integer;
				` + producerBody + `
				succession first start then step;
				succession first step then done;
			}
			action consumer {
				in value : Integer;
				action take { assign total := total + 1; }
				succession first start then take;
				succession first take then done;
			}
			join sync;
			succession first start then split;
			succession first split then producer;
			succession first split then consumer;
			succession first producer then sync;
			succession first consumer then sync;
			succession first sync then done;
			flow producer.value to ` + target + `;
		}
	}`
}

// testStreamingFlowSourceNeverWrites: a source that completes without ever writing
// the pin its flow streams from has carried nothing, which the run reports rather
// than leaving the target's pin silently empty.
func testStreamingFlowSourceNeverWrites(t *testing.T) {
	_, err := executeActionSource(t, "stream", streamingPair(
		"action step { assign total := total + 0; }", "consumer.value",
	))
	if !errors.Is(err, ErrFlowSource) || !strings.Contains(err.Error(), "node producer produced no value at value") {
		t.Fatalf("error = %v, want ErrFlowSource from a source that never wrote", err)
	}
}

// testStreamingFlowTargetCompletedFirst: a value streamed after the target's last
// performance ended reaches no performance of it, which the run reports once the
// action completes without a further performance having taken it.
func testStreamingFlowTargetCompletedFirst(t *testing.T) {
	_, err := executeActionSource(t, "stream", `package test {
		action stream {
			attribute total : Integer = 0;
			action consumer {
				in value : Integer;
				assign total := total + 1;
			}
			action producer {
				out value : Integer;
				assign value := 7;
			}
			succession first start then consumer;
			succession first consumer then producer;
			succession first producer then done;
			flow producer.value to consumer.value;
		}
	}`)
	if !errors.Is(err, ErrStreamUnreceived) || !strings.Contains(err.Error(), "node consumer completed before node producer wrote value") {
		t.Fatalf("error = %v, want ErrStreamUnreceived from a target over before its source wrote", err)
	}
}

// testStreamingFlowTargetPinNotDeclared: a stream to a pin the target does not
// declare is refused at the first write, while the target is being performed.
func testStreamingFlowTargetPinNotDeclared(t *testing.T) {
	_, err := executeActionSource(t, "stream", streamingPair(
		"action step { assign value := 1; }", "consumer.missing",
	))
	if !errors.Is(err, ErrNodePin) {
		t.Fatalf("error = %v, want ErrNodePin from a stream to an undeclared pin", err)
	}
}

// testStreamingFlowOneLateValueLeft: two values streamed after the target's performance
// ended; one further performance of the target takes the first, and the second, which no
// performance took, is reported when the action completes.
func testStreamingFlowOneLateValueLeft(t *testing.T) {
	_, err := executeActionSource(t, "stream", `package test {
		action stream {
			attribute total : Integer = 0;
			merge again;
			action consumer {
				in value : Integer;
				assign total := total + 1;
			}
			action producer {
				out value : Integer;
				assign value := 1;
			}
			action second {
				out value : Integer;
				assign value := 2;
			}
			decide choose;
			succession first start then again;
			succession first again then consumer;
			succession first consumer then choose;
			if total == 1 then producer;
			else done;
			succession first producer then second;
			succession first second then again;
			flow producer.value to consumer.value;
			flow second.value to consumer.value;
		}
	}`)
	if !errors.Is(err, ErrStreamUnreceived) || !strings.Contains(err.Error(), "node consumer completed before node second wrote value") {
		t.Fatalf("error = %v, want ErrStreamUnreceived for the late value no performance took", err)
	}
}

// testStreamingFlowCycle: two ongoing nodes stream into each other, so a write to one
// pin is carried back to it; that is refused rather than carried on without end.
func testStreamingFlowCycle(t *testing.T) {
	_, err := executeActionSource(t, "stream", `package test {
		action stream {
			attribute n : Integer = 0;
			fork split;
			action a {
				inout v : Integer;
				action wait { assign n := n + 1; }
				action write { assign v := 1; }
				succession first start then wait;
				succession first wait then write;
				succession first write then done;
			}
			action b {
				inout v : Integer;
				action idle { assign n := n + 1; }
				action linger { assign n := n + 1; }
				succession first start then idle;
				succession first idle then linger;
				succession first linger then done;
			}
			join sync;
			succession first start then split;
			succession first split then a;
			succession first split then b;
			succession first a then sync;
			succession first b then sync;
			succession first sync then done;
			flow a.v to b.v;
			flow b.v to a.v;
		}
	}`)
	if !errors.Is(err, ErrStreamCycle) {
		t.Fatalf("error = %v, want ErrStreamCycle", err)
	}
}
