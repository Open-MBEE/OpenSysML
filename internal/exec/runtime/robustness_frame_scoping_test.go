package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessFrameScoping exercises names that reach past the feature
// a body may read: a block-local attribute is gone once its block completes, a
// nested node's attribute is that node's and not the enclosing body's, and a
// body attribute that shadows an accept payload is read as itself, so one the
// body never values is uninitialized rather than the payload.
func TestRuntimeRobustnessFrameScoping(t *testing.T) {
	t.Run("block_local_attribute_is_unresolved_after_its_block", testFrameScopingBlockLocalUnresolved)
	t.Run("nested_node_attribute_is_unresolved_in_the_enclosing_body", testFrameScopingNestedAttributeUnresolved)
	t.Run("body_attribute_shadowing_a_payload_reads_as_itself", testFrameScopingShadowedPayloadNotRead)
}

func testFrameScopingBlockLocalUnresolved(t *testing.T) {
	_, err := executeActionSource(t, "run", `package test {
		private import ScalarValues::*;
		action run {
			attribute later : Integer = 0;
			first start;
			then action body {
				if true { attribute x : Integer = 5; }
				then assign later := x;
			}
			then done;
		}
	}`)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference for a block-local read after its block", err)
	}
}

func testFrameScopingNestedAttributeUnresolved(t *testing.T) {
	_, err := executeActionSource(t, "run", `package test {
		private import ScalarValues::*;
		action run {
			attribute later : Integer = 0;
			first start;
			then action inner { attribute x : Integer = 5; }
			then action reader { assign later := x; }
			then done;
		}
	}`)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("error = %v, want ErrUnresolvedReference for a nested node's attribute read by simple name", err)
	}
}

func testFrameScopingShadowedPayloadNotRead(t *testing.T) {
	_, err := executeActionSource(t, "communicator", `package test {
		private import ScalarValues::*;
		action communicator {
			attribute msg : Integer;
			attribute seen : Integer = 0;
			first start;
			action sender { send 9 to receiver; }
			action receiver accept msg : Integer;
			action reader { assign seen := msg; }
			done;
			succession first start then sender;
			succession first sender then receiver;
			succession first receiver then reader;
			succession first reader then done;
		}
	}`)
	var noValue *NoValueError
	if !errors.As(err, &noValue) || noValue.Feature != "msg" {
		t.Fatalf("error = %v, want NoValueError for the body's own unvalued msg, not the payload", err)
	}
}
