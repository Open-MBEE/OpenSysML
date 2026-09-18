// An association end may declare its cross feature inline, ahead of its own
// declaration (KerML.xtext OwnedCrossFeatureMember).
package W12DEndCrossFeature {
	class C1 {
		feature a : C2;
	}

	class C2 {
		feature b : C1;
	}

	assoc A1 {
		end x1 [0..1] feature x : C1 crosses y.b {
			public import y::y1;
		}
		end feature y : C2 crosses x.y1 {
			member feature y1 [0..1] featured by C1;
		}
	}

	assoc A2 {
		end xa subsets C1::a feature x : C1;
		end feature y : C2;
	}

	// The prefix between `end` and the cross feature's declaration is the
	// cross feature's (KerML.xtext BasicFeaturePrefix), not the end's.
	assoc A3 {
		end var x1 : C1 [1] feature x : C1;
		end derived const x2 : C2 [1] feature y : C2;
	}
	assoc A4 {
		end in x1 [1] feature x : C1;
		end abstract composite x2 [1] feature y : C2;
	}
	// A reserved `var` with no name is still the cross feature's prefix.
	assoc A5 {
		end var [1] feature x : C1;
		end feature y : C2;
	}
	// An unnamed cross feature may specialize after its multiplicity
	// (KerML.xtext FeatureSpecializationPart).
	assoc A6 {
		end [1] :> C1::a feature x : C1;
		end [1] subsets C2::b : C1 feature y : C2;
	}
	assoc A7 {
		end x1 [1..*] ordered feature x : C1;
		end [*] nonunique ordered :> C2::b feature y : C2;
	}
	assoc A8 {
		end ordered nonunique feature x : C1;
		end [1] nonunique feature y : C2;
	}
}
