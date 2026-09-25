// A redefining declaration whose body values features the value it inherits
// would supply governs over that value: the more specific declaration of the
// feature holds, while a body that only re-declares features still reads the
// inherited value.
package test {
    private import ScalarValues::Real;

    attribute def Cost { attribute v : Real = 1.0; attribute w : Real = 2.0; }
    attribute def Inner { attribute u : Real = 1.0; }
    attribute def Nested { attribute v : Inner; }

    part def Ring {
        attribute template : Cost { attribute :>> v = 9.0; attribute :>> w = 8.0; }
        attribute cost : Cost = template;
        attribute kept : Cost = template;
        attribute plain : Cost = template;

        attribute deepTemplate : Nested { attribute :>> v { attribute :>> u = 9.0; } }
        attribute deepKept : Nested = deepTemplate;
        attribute deepGoverned : Nested = deepTemplate;
    }

    part r : Ring {
        attribute :>> cost { attribute :>> v = 11.0; }
        // A body that only re-declares a feature states no value, so the
        // inherited value still binds this one — at any depth of nesting.
        attribute :>> kept { attribute :>> v; }
        attribute :>> deepKept { attribute :>> v { attribute :>> u; } }
        // A value stated at depth is a value all the same, so this body governs.
        attribute :>> deepGoverned { attribute :>> v { attribute :>> u = 11.0; } }

        assert constraint bodyGovernsInheritedValue { cost.v == 11.0 }
        assert constraint featureNotValuedTakesItsTypesDefault { cost.w == 2.0 }
        assert constraint reDeclarationKeepsInheritedValue { kept.v == 9.0 }
        assert constraint valueBindsWithoutABody { plain.v == 9.0 }
        assert constraint deepReDeclarationKeepsInheritedValue { deepKept.v.u == 9.0 }
        assert constraint valueStatedAtDepthGoverns { deepGoverned.v.u == 11.0 }
    }
}
