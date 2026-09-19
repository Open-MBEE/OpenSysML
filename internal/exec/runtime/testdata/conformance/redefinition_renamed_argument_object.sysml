// An object performs an action with an argument redefining a parameter under a
// name of its own, a short name alone included: the action's body reads the
// redefined parameter's name and finds the argument's value.
package test {
    private import ScalarValues::*;

    action def Tenfold {
        in x : Integer default 3;
        out y : Integer;
        first start;
        then assign y := x * 10;
        then done;
    }

    part machine {
        perform action ps : Tenfold { in <a> :>> x = 4; }
        perform action pn : Tenfold { in b :>> x = 7; }
    }

    calc def probe {
        return : Integer = machine.ps.y + machine.pn.y;
    }
}
