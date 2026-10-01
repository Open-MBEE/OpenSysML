package semantics

import "testing"

func TestFeatureReadOnly(t *testing.T) {
	m, root := buildModel(t, `package P {
	part def Base {
		constant attribute k;
		derived attribute d;
		attribute w;
	}
	part def Sub :> Base {
		attribute :>> k;
		attribute :>> d;
		attribute :>> w;
		attribute s :> k;
		attribute ds :> d;
	}
	action def Gen { in constant attribute limit; }
	action def Spec :> Gen { in attribute cap; }
	part def Cyc {
		attribute a :> b;
		attribute b :> a;
	}
}`)
	tests := []struct {
		path     string
		kind     ReadOnlyKind
		declared string
	}{
		{"P::Base::k", ReadOnlyConstant, "P::Base::k"},
		{"P::Base::d", ReadOnlyDerived, "P::Base::d"},
		{"P::Base::w", Writable, ""},
		{"P::Sub::k", ReadOnlyConstant, "P::Base::k"},
		{"P::Sub::d", ReadOnlyDerived, "P::Base::d"},
		{"P::Sub::w", Writable, ""},
		{"P::Sub::s", ReadOnlyConstant, "P::Base::k"},
		// Subsetting a derived feature makes the subsetting one's values a subset
		// of them, not values the model determines, so it stays writable.
		{"P::Sub::ds", Writable, ""},
		// A parameter implicitly redefines the general behavior's one at its position.
		{"P::Spec::cap", ReadOnlyConstant, "P::Gen::limit"},
		{"P::Cyc::a", Writable, ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := m.FeatureReadOnly(nestedSym(t, root, tt.path))
			if got.Kind != tt.kind {
				t.Fatalf("kind = %v, want %v", got.Kind, tt.kind)
			}
			if tt.declared == "" {
				if got.Declared != nil {
					t.Fatalf("declared = %v, want none", got.Declared.Name)
				}
				return
			}
			if wantSym := nestedSym(t, root, tt.declared); got.Declared != wantSym {
				t.Fatalf("declared by %v, want %s", got.Declared, tt.declared)
			}
		})
	}
}
