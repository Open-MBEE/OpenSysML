// A redefinition in a specialization keeps what the base declared and overrides
// only what it restates, over a chain where the innermost restatement wins.
package test {
    private import ScalarValues::Real;

    part def Inner {
        attribute a : Real;
        attribute b : Real;
        attribute c : Real;
        attribute sum : Real = a + b + c;
    }

    part def Outer {
        part inner : Inner;
        attribute t : Real = inner.b;
    }

    part base : Outer {
        part :>> inner {
            attribute :>> b = 7.0;
            attribute :>> c = 1.0;
        }
    }

    part middle :> base {
        part :>> inner {
            attribute :>> a = 1.0;
            attribute :>> c = 2.0;
        }
    }

    part leaf :> middle {
        part :>> inner {
            attribute :>> c = 3.0;
        }
        assert constraint keepsBaseValue {
            inner.b == 7.0
        }
        assert constraint innermostWins {
            inner.c == 3.0
        }
        assert constraint keepsMiddleValue {
            inner.a == 1.0
        }
        assert constraint sumsInheritedValues {
            inner.sum == 11.0
        }
    }
}
