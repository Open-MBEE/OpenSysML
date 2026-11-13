package passes

import "testing"

// Concern, framed concern, objective and satisfy are RequirementUsages (SysML v2 §8.3.21.9)
// and a render a RenderingUsage (§8.3.26.6): a second effective type gets the pilot's message.
func TestOneTypeCoversTheRequirementAndRenderingUsageFamilies(t *testing.T) {
	const reqMsg = "A requirement must be typed by one requirement definition."
	const renMsg = "A rendering must be typed by one rendering definition."
	cases := []struct{ name, src, want string }{
		{"concern", `package T {
			concern def C1; concern def C2;
			concern c : C1, C2;
		}`, reqMsg},
		{"frame concern", `package T {
			concern def C1; concern def C2;
			requirement r { frame concern c : C1, C2; }
		}`, reqMsg},
		{"viewpoint frame concern", `package T {
			concern def C1; concern def C2;
			viewpoint v { frame concern c : C1, C2; }
		}`, reqMsg},
		{"satisfy requirement", `package T {
			requirement def R1; requirement def R2;
			part p { satisfy requirement s : R1, R2; }
		}`, reqMsg},
		{"objective", `package T {
			requirement def R1; requirement def R2;
			use case u { objective o : R1, R2; }
		}`, reqMsg},
		{"render", `package T {
			rendering def N1; rendering def N2;
			view v { render rendering n : N1, N2; }
		}`, renMsg},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := only(typeDiags(t, c.src), "one-type")
			if len(diags) != 1 || diags[0].Message != c.want {
				t.Fatalf("want one %q, got %v", c.want, diags)
			}
		})
	}
}

// One declared type, or two where one is the other's supertype (KerML §8.3.3.3.4),
// is silent for the same kinds.
func TestOneTypeRequirementAndRenderingFamiliesAcceptOneEffectiveType(t *testing.T) {
	const src = `package T {
		concern def C1; concern def C2 :> C1;
		requirement def R1; requirement def R2 :> R1;
		rendering def N1; rendering def N2 :> N1;
		concern c1 : C1;
		concern c2 : C1, C2;
		requirement r {
			frame concern fc1 : C1;
			frame concern fc2 : C1, C2;
		}
		part p {
			satisfy requirement s1 : R1;
			satisfy requirement s2 : R1, R2;
		}
		use case u {
			objective o1 : R1;
			objective o2 : R1, R2;
		}
		view v {
			render rendering n1 : N1;
			render rendering n2 : N1, N2;
		}
	}`
	if diags := only(typeDiags(t, src), "one-type"); len(diags) != 0 {
		t.Fatalf("one effective type is silent, got %v", diags)
	}
}
