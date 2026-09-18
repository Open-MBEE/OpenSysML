// A struct-typed attribute takes its value from a nested body restating the
// features of its type, on a declaration and on a redefinition of one.
package test {
    private import ScalarValues::Real;

    attribute def Cost { attribute v : Real = 1.0; }
    attribute def Assembly { attribute piece : Cost; }

    part def Ring { attribute ringCost : Cost; attribute assembly : Assembly; }
    part def Band :> Ring { attribute bandCost :>> ringCost; }

    part b : Band {
        attribute :>> bandCost { attribute :>> v = 400.0; }
        attribute :>> assembly { attribute :>> piece { attribute :>> v = 5.0; } }
        attribute declared : Cost { attribute :>> v = 7.0; }
    }
}
