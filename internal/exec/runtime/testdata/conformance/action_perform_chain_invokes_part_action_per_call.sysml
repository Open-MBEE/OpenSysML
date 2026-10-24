package test {
    private import ScalarValues::*;

    part def Counter {
        attribute count : Integer = 0;

        action inc {
            in delta : Integer;
            assign count := count + delta;
        }
    }

    action def Main {
        part c : Counter;
        out result : Integer;

        perform action a ::> c.inc {
            in delta[1] = 2;
        }

        perform action b ::> c.inc {
            in delta[1] = 3;
        }

        action read {
            assign result := c.count;
        }

        first start then a;
        first a then b;
        first b then read;
        first read then done;
    }
}
