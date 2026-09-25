// A `bind` a base action states between two of its nodes holds at both of
// them or at neither: when the derived action declares its own node under the
// name of one end, the binding does not reach the other end through the
// replacement. `Base`'s `bind add.a = src.n` names `Base::src`; `Replaced`
// takes the name `src` for a node of its own, so the binding lowers to nothing
// and `add` keeps its declared `a = 1` rather than reading the replacement's
// `n = 1000`.
package test {
    private import ScalarValues::*;

    action def Adder {
        in a : Integer;
        in b : Integer;
        out sum : Integer;

        first step;
        action step { assign sum := a + b; }
    }

    action def Base {
        out result : Integer = 0;

        action src { out n : Integer; assign n := 5; }
        action add : Adder { in a = 1; in b = 2; }
        bind add.a = src.n;
        flow add.sum to fin.k;
        action fin { in k : Integer; assign result := k; }
    }

    action def Replaced :> Base {
        action src { out n : Integer; assign n := 1000; }
        first start then src;
        succession src then add;
        succession add then fin;
        then done;
    }
}
