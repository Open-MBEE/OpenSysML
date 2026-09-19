// A `bind` or `flow` a base action states at a node applies to that node's
// declaration, or to one redefining it, never to an unrelated node the derived
// action declares under the same name. `Masked` declares its own `add` and
// `fin`, so `Base`'s `bind add.a = x` and `flow add.sum to fin.n` lower to
// nothing and `result` keeps its default; `Redefined` redefines `add`, so both
// hold at the redefining node.
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
        attribute x : Integer = 5;
        out result : Integer = 0;

        action add : Adder { in b = 2; }
        bind add.a = x;
        flow add.sum to fin.n;
        action fin { in n : Integer; assign result := n; }
    }

    action def Masked :> Base {
        action add { out total : Integer; assign total := 42; }
        action fin { in k : Integer = 0; out seen : Integer; assign seen := add.total; }
        first start then add;
        succession add then fin;
        then done;
    }

    action def Redefined :> Base {
        action add :>> add : Adder { in b = 100; }
        first start then add;
        succession add then fin;
        then done;
    }
}
