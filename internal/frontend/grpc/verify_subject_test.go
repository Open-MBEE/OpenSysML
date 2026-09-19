package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// nestedSubjectModelSource redefines a nested feature on the object, so the
// object a check is about is the nested one rather than the one named.
const nestedSubjectModelSource = `package Demo {
	part def Wheel {
		attribute pressure default = 30.0;
		constraint inflated {
			pressure > 20.0
		}
	}
	part def Car {
		part wheel : Wheel;
	}
	part flat : Car {
		part :>> wheel {
			attribute :>> pressure = 5.0;
		}
	}
}
`

// A verdict about a condition reached through a nested redefinition names the
// nested object it was evaluated against, so a client reading the reported feature values
// sees the values behind the verdict.
func TestVerifyConstraintNamesTheNestedSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, nestedSubjectModelSource, "verify-nested-subject")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Wheel::inflated",
		SubjectSymbolId: "Demo::flat",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("VerifyConstraint reported %q", resp.Error)
	}
	if resp.Verdict.Holds {
		t.Error("inflated on flat: holds = true, want the nested redefinition to violate it")
	}
	if resp.Verdict.InstanceTypeId == "Demo::Car" {
		t.Errorf("instance_type_id = %q, want the nested wheel object rather than the car named", resp.Verdict.InstanceTypeId)
	}
	var named *pb.Instance
	for _, inst := range resp.Instances {
		if inst.Id == resp.Verdict.InstanceId {
			named = inst
		}
	}
	if named == nil {
		t.Fatalf("verdict names instance %d, which the response does not carry", resp.Verdict.InstanceId)
	}
	pressure, ok := named.FeatureValues["pressure"]
	if !ok {
		t.Fatalf("the object the verdict names has no pressure feature value: %v", named.FeatureValues)
	}
	if pressure.Value.GetRealValue() != 5.0 {
		t.Errorf("pressure = %v, want the redefined 5", pressure)
	}
}

// ambiguousSubjectModelSource redefines the same nested feature on two objects of
// the same car, so no one of them is the object a check is about.
const ambiguousSubjectModelSource = `package Demo {
	part def Bolt {
		attribute torque default = 1.0;
		constraint tight {
			torque > 10.0
		}
	}
	part def Axle {
		part bolt : Bolt;
	}
	part def Car {
		part front : Axle {
			part :>> bolt {
				attribute :>> torque = 20.0;
			}
		}
		part rear : Axle {
			part :>> bolt {
				attribute :>> torque = 30.0;
			}
		}
	}
	part car : Car;
}
`

// An ambiguous subject is a check that was never made, so it arrives as a typed
// reason rather than as a verdict of false a client would read as a violation.
func TestVerifyConstraintReportsAnAmbiguousSubject(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, ambiguousSubjectModelSource, "verify-ambiguous-subject")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Bolt::tight",
		SubjectSymbolId: "Demo::car",
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if got := resp.Verdict.FailureReason; got != pb.FailureReason_FAILURE_REASON_AMBIGUOUS_SUBJECT {
		t.Errorf("failure_reason = %v, want FAILURE_REASON_AMBIGUOUS_SUBJECT (error: %q)", got, resp.Verdict.Error)
	}
	// The message still names the carriers apart, which is what a caller acts on.
	for _, want := range []string{"front::bolt", "rear::bolt"} {
		if !strings.Contains(resp.Verdict.Error, want) {
			t.Errorf("error %q does not name a carrier %q", resp.Verdict.Error, want)
		}
	}
}
