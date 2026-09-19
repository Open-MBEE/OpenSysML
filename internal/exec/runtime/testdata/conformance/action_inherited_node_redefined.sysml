// The redefining `add` of `Redefined` takes `Base`'s `bind add.a = x` and
// `flow add.sum to fin.n`; see action_inherited_node_masked.sysml.
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

    action def Redefined :> Base {
        action add :>> add : Adder { in b = 100; }
        first start then add;
        succession add then fin;
        then done;
    }
}
