// The typed usage's own `assign y := y + 1` is unordered with Scale's inherited
// `start → scaling → done`: it leaves 31 after scaling, or scaling overwrites it with 30 first.
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
