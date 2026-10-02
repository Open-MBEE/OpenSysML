// The typed usage merges Scale's ordered flow with its own unordered assignment,
// so either write may stand last in `scaled`'s output.
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
