package edit

import (
	"strings"
	"testing"
)

const forwardBatchModel = `package P {
    metadata def Tag;
    action def A {
        first start;
        action a;
    }
    part def Vehicle;
}
`

// A batch is resolved as a whole: a reference one operation writes may name a
// declaration a later operation of the same batch adds, whatever the order the
// operations were given in.
func TestBatchResolvesReferencesAgainstTheWholeBatch(t *testing.T) {
	tests := []struct {
		name string
		ops  []Operation
		want []string
	}{
		{
			name: "typed action then its action def",
			ops: []Operation{
				AddThenMember("P::A", "action", "g", "GenerateHeat"),
				AddMember("P", "action def", "GenerateHeat"),
			},
			want: []string{"then action g : GenerateHeat;", "action def GenerateHeat;"},
		},
		{
			name: "typed part then its part def",
			ops: []Operation{
				{Kind: OpAddMember, Owner: "P", MemberKind: "part", MemberName: "engine", Type: "Engine"},
				AddMember("P", "part def", "Engine"),
			},
			want: []string{"part engine : Engine;", "part def Engine;"},
		},
		{
			name: "typed attribute then its attribute def",
			ops: []Operation{
				{Kind: OpAddMember, Owner: "P::Vehicle", MemberKind: "attribute", MemberName: "mass", Type: "Mass"},
				AddMember("P", "attribute def", "Mass"),
			},
			want: []string{"attribute mass : Mass;", "attribute def Mass;"},
		},
		{
			name: "succession then the member it reaches",
			ops: []Operation{
				AddThen("P::A", "g"),
				AddMember("P::A", "action", "g"),
			},
			want: []string{"then g;", "action g;"},
		},
		{
			name: "initial node then the member it starts at",
			ops: []Operation{
				AddFirst("P::A", "g"),
				AddMember("P::A", "action", "g"),
			},
			want: []string{"first g;", "action g;"},
		},
		{
			name: "guarded succession then the member it reaches",
			ops: []Operation{
				AddGuardedThen("P::A", "true", "g"),
				AddMember("P::A", "action", "g"),
			},
			want: []string{"if true then g;", "action g;"},
		},
		{
			name: "metadata prefix then its metadata def",
			ops: []Operation{
				AddMetadataPrefix("P::Vehicle", "Marker"),
				AddMember("P", "metadata def", "Marker"),
			},
			want: []string{"#Marker part def Vehicle;", "metadata def Marker;"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ops := range [][]Operation{tt.ops, {tt.ops[1], tt.ops[0]}} {
				got := applyOps(t, forwardBatchModel, ops...)
				for _, want := range tt.want {
					if !strings.Contains(got, want) {
						t.Fatalf("result lacks %q:\n%s", want, got)
					}
				}
			}
		})
	}
}

// A reference nothing in the batch declares is refused as it is when it stands
// alone: by the same check, at the same operation, with the same message.
func TestBatchRefusesReferencesUnresolvedAfterTheWholeBatch(t *testing.T) {
	tests := []struct {
		name    string
		ops     []Operation
		want    Failure
		index   int
		message string
	}{
		{
			name: "typed action",
			ops: []Operation{
				AddThenMember("P::A", "action", "g", "Nowhere"),
				AddMember("P", "action def", "GenerateHeat"),
			},
			want: FailureResultInvalid, index: -1,
			message: "unresolved reference: Nowhere",
		},
		{
			name: "succession",
			ops: []Operation{
				AddThen("P::A", "missing"),
				AddMember("P::A", "action", "g"),
			},
			want: FailureUnknownTarget, index: 0,
			message: `sequence node "missing" resolves to nothing visible from "P::A"`,
		},
		{
			name: "initial node whose own label is all the name reaches",
			ops: []Operation{
				AddFirst("P::A", "missing"),
				AddMember("P::A", "action", "g"),
			},
			want: FailureUnknownTarget, index: 0,
			message: `sequence node "missing" resolves to nothing visible from "P::A"`,
		},
		{
			name: "succession to a definition the batch adds",
			ops: []Operation{
				AddThen("P::A", "g"),
				AddMember("P", "action def", "g"),
			},
			want: FailureResultInvalid, index: -1,
			message: "succession endpoint g is not an action node",
		},
		{
			name: "metadata prefix",
			ops: []Operation{
				AddMetadataPrefix("P::Vehicle", "Missing"),
				AddMember("P", "metadata def", "Marker"),
			},
			want: FailureInvalidValue, index: 0,
			message: `metadata type "Missing" does not resolve from P::Vehicle`,
		},
		{
			name: "metadata prefix twice in one batch",
			ops: []Operation{
				AddMetadataPrefix("P::Vehicle", "Tag"),
				AddMetadataPrefix("P::Vehicle", "Tag"),
			},
			want: FailureInvalidValue, index: 0,
			message: `"P::Vehicle" already carries #Tag`,
		},
		{
			name: "metadata prefix by a later part def",
			ops: []Operation{
				AddMetadataPrefix("P::Vehicle", "Marker"),
				AddMember("P", "part def", "Marker"),
			},
			want: FailureInvalidValue, index: 0,
			message: `"Marker" is a partDef, not a metadata definition`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Apply(loadContent(t, "batch.sysml", forwardBatchModel), tt.ops)
			if res != nil {
				t.Fatalf("refused batch returned content:\n%s", res.Content)
			}
			e := editError(t, err)
			if e.Failure != tt.want || e.OperationIndex != tt.index {
				t.Fatalf("refusal = %s at %d (%s), want %s at %d", e.Failure, e.OperationIndex, e.Message, tt.want, tt.index)
			}
			if !strings.Contains(e.Message, tt.message) {
				t.Fatalf("message = %q, want it to contain %q", e.Message, tt.message)
			}
		})
	}
}

// Names are still taken in operation order: an earlier operation's declaration
// takes the name a later one asks for, and a later declaration that would hide
// the one an earlier reference reaches is judged on the model it leaves.
func TestBatchNamesShadowInOperationOrder(t *testing.T) {
	src := `package P {
    action def GenerateHeat;
    package Q {
        action def A {
            first start;
        }
    }
}
`
	res, err := Apply(loadContent(t, "shadow.sysml", src), []Operation{
		AddMember("P::Q", "action def", "GenerateHeat"),
		AddMember("P::Q", "action def", "GenerateHeat"),
	})
	if res != nil {
		t.Fatalf("duplicate names returned content:\n%s", res.Content)
	}
	e := editError(t, err)
	if e.Failure != FailureMemberNameTaken || e.OperationIndex != 1 {
		t.Fatalf("refusal = %s at %d (%s), want %s at 1", e.Failure, e.OperationIndex, e.Message, FailureMemberNameTaken)
	}

	// The typed action reaches P::Q::GenerateHeat, which the later operation
	// adds in front of P::GenerateHeat; both are action defs, so the batch holds.
	got := applyOps(t, src, AddThenMember("P::Q::A", "action", "g", "GenerateHeat"), AddMember("P::Q", "action def", "GenerateHeat"))
	for _, want := range []string{"then action g : GenerateHeat;", "        action def GenerateHeat;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("result lacks %q:\n%s", want, got)
		}
	}

	// A later part def of the same name hides the action def the typed action
	// meant, and the batch is refused for the model it would leave.
	res, err = Apply(loadContent(t, "shadow.sysml", src), []Operation{
		AddThenMember("P::Q::A", "action", "g", "GenerateHeat"),
		AddMember("P::Q", "part def", "GenerateHeat"),
	})
	if res != nil {
		t.Fatalf("shadowed typing returned content:\n%s", res.Content)
	}
	e = editError(t, err)
	if e.Failure != FailureResultInvalid {
		t.Fatalf("refusal = %s (%s), want %s", e.Failure, e.Message, FailureResultInvalid)
	}
}
