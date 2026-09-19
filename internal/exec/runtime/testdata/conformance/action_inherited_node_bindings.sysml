// A `bind` or `flow` a base action states at a pin of a node the derived action
// inherits applies to the inherited node's performance: `Base` binds `add.a` to
// its `x` (which `Derived` redefines) and flows `add.sum` on to `fin.n`; `Mid`
// binds `extra`'s pins to its own `y` and to the sibling's `add.sum`.
// `Derived` specializes `Mid` and `Base` both, and each binding applies once.
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

    action def Mid :> Base {
        attribute y : Integer = 100;
        action extra : Adder;
        bind extra.a = y;
        bind extra.b = add.sum;
    }

    action def Derived :> Mid, Base {
        attribute :>> x = 9;
        first start then add;
        succession add then fin;
        succession fin then extra;
        succession extra then done;
    }
}
