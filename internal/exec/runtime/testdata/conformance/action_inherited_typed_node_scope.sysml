// An inherited typed node performs the action its declaration names, resolved
// where the node was declared: `Derived` sequences `Base`'s `twice` and
// `twiceMore`, whose callee `Double` is a private member of `Base` alone.
package test {
    private import ScalarValues::*;

    action def Base {
        private action def Double {
            in x : Integer;
            out y : Integer;

            first step;
            action step { assign y := 2 * x; }
        }

        out result : Integer = 0;

        action twice : Double {
            in x = 21;
        }
        action twiceMore = Double(x = 4);
    }

    action def Derived :> Base {
        first start then twice;
        succession twice then twiceMore;
        succession twiceMore then fin;
        action fin { assign result := twice.y + twiceMore.y; }
    }
}
