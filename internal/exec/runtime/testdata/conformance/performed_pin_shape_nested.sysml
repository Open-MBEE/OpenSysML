// A performed action nested in a performed action takes the rule at every depth: the
// outer bare pin is a one-element sequence, the inner [1] pin bare, and the inner bare
// pin fed from the outer one still holds one element, not a sequence of a sequence.
package test {
    private import ScalarValues::*;

    action def Inner {
        in v : Integer;
        in w : Integer[1];
        out sum : Integer;
        out sumOne : Integer[1];
        first start;
        then action s { assign sum := v + w; assign sumOne := v + w; }
        then done;
    }

    action def Outer {
        in value : Integer;
        out inner : Integer;
        out innerOne : Integer[1];
        first start;
        then action i : Inner { in v = value; in w = value; }
        then action r { assign inner := i.sum; assign innerOne := i.sumOne; }
        then done;
    }

    action probe {
        out value : Integer;
        out inner : Integer;
        out innerOne : Integer[1];
        out innerV : Integer;
        out innerW : Integer[1];
        first start;
        then action o : Outer { in value = 4; }
        then action read {
            assign value := o.value;
            assign inner := o.inner;
            assign innerOne := o.innerOne;
            assign innerV := o.i.v;
            assign innerW := o.i.w;
        }
        then done;
    }
}
