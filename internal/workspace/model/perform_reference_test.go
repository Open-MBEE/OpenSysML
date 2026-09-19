package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// The two OMG training models that a performed action's members are reachable
// from: `18. Action Performance/Action Performance Example.sysml` and
// `38. Allocation/Allocation Usage Example.sysml`, reduced to the constructs
// under test so the regression is pinned without the corpus.
func TestPerformedActionMembersResolve(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{
			name: "named perform action with references",
			src: `package 'Action Performance' {
				action takePicture {
					action focus;
					action shoot;
				}

				part camera {
					perform action takePhoto[*] ordered references takePicture;

					part f {
						perform takePhoto.focus;
					}

					part i {
						perform takePhoto.shoot;
					}
				}
			}`,
		},
		{
			name: "perform shorthand naming a feature chain",
			src: `package 'Allocation Usage' {
				package LogicalModel {
					action def ProvidePower;
					action def GenerateTorque;

					action providePower : ProvidePower {
						action generateTorque : GenerateTorque;
					}

					part torqueGenerator {
						perform providePower.generateTorque;
					}
				}

				package PhysicalModel {
					private import LogicalModel::*;

					part powerTrain {
						part engine {
							perform providePower.generateTorque;
						}
					}

					allocate torqueGenerator to powerTrain {
						allocate torqueGenerator.generateTorque to powerTrain.engine.generateTorque;
					}
				}
			}`,
		},
		{
			name: "perform shadowing the action it performs",
			src: `package 'Requirement Satisfaction' {
				action 'provide power' {
					action 'generate torque' { }
				}

				part vehicle {
					perform 'provide power';

					part engine {
						perform 'provide power'.'generate torque';
					}
				}
			}`,
		},
		{
			name: "perform of an action inherited from the part's type",
			src: `package 'Inherited Performance' {
				part def Vehicle {
					action providePower {
						action generateTorque;
					}
				}

				part vehicle : Vehicle {
					perform providePower;

					part engine {
						perform providePower.generateTorque;
					}
				}
			}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := NewWorkspace()
			ws.Open("t.sysml", []byte(tc.src), 1)
			for _, d := range ws.Diagnostics("t.sysml") {
				if d.Severity == diag.SeverityError {
					t.Errorf("unexpected error: %s", d.Message)
				}
			}
		})
	}
}
