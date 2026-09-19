// An invoked node's `out` values return to same-named enclosing features once
// the node completes, so what its own body makes of an output is what returns:
// `scaled` performs Scale (y = 30), its body adds one, and Adjust's `y` reads 31.
package test {
    private import ScalarValues::*;

    action def Scale {
        in x : Integer;
        in factor : Integer = 10;
        out y : Integer;

        first start;
        then action scaling { assign y := x * factor; }
        then done;
    }

    action def Adjust {
        out y : Integer = 0;
        out seen : Integer = 0;

        first start;
        then action scaled : Scale {
            in x = 3;
            assign y := y + 1;
        }
        then action check { assign seen := y; }
        then done;
    }
}
