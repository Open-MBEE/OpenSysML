package test {
    private import ScalarValues::*;

    // A parameter redefined under a name of its own, a short name alone included
    // (`in <f> :>> factor`), binds the parameter it redefines: an inherited body
    // reading the redefined name reads the redefinition's value.
    calc def Scale {
        in x : Integer;
        in factor : Integer default 2;
        x * factor
    }
    calc def ShortFactor :> Scale { in <f> :>> factor default 3; }
    calc def NamedFactor :> Scale { in g :>> factor default 4; }

    action def Base {
        in x : Integer default 3;
        in z : Integer default x * 2;
        out y : Integer;
    }
    action def ShortRedefined :> Base {
        in <w> :>> x = 5;
        first start;
        then assign y := w + z;
        then done;
    }
    action def NamedRedefined :> Base {
        in v :>> x = 6;
        first start;
        then assign y := v + z;
        then done;
    }
    action def Tenfold :> Base {
        first start;
        then assign y := x * 10;
        then done;
    }

    action outer {
        out shortCalc : Integer;
        out namedCalc : Integer;
        out shortAction : Integer;
        out namedAction : Integer;
        out shortArgument : Integer;
        out namedArgument : Integer;
        first start;
        then action sr : ShortRedefined;
        then action nr : NamedRedefined;
        then perform action ps : Tenfold { in <a> :>> x = 4; }
        then perform action pn : Tenfold { in b :>> x = 7; }
        then action fin {
            assign shortCalc := ShortFactor(7);
            assign namedCalc := NamedFactor(7);
            assign shortAction := sr.y;
            assign namedAction := nr.y;
            assign shortArgument := ps.y;
            assign namedArgument := pn.y;
        }
        then done;
    }
}
