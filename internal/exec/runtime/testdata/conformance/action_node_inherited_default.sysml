package test {
    private import ScalarValues::*;

    // A default an action inherits and does not redeclare is in force when the
    // action is performed, however it is invoked; one that reads a parameter the
    // action redefines reads the redefinition.
    action def Base {
        in x : Integer = 3;
        in z : Integer = x * 2;
        out y : Integer;
    }

    action def Derived :> Base {
        first start;
        then assign y := x + z;
        then done;
    }

    action def Redefined :> Base {
        in :>> x = 5;
        first start;
        then assign y := x + z;
        then done;
    }

    action def BaseResult {
        in x : Integer = 3;
        out result : Integer;
    }

    action def DerivedResult :> BaseResult {
        first start;
        then assign result := x * 10;
        then done;
    }

    action outer {
        out typed : Integer;
        out redefined : Integer;
        out invoked : Integer;
        out performed : Integer;
        first start;
        then action d : Derived;
        then action rd : Redefined;
        then action c = DerivedResult();
        then perform action p : DerivedResult;
        then action fin {
            assign typed := d.y;
            assign redefined := rd.y;
            assign invoked := c;
            assign performed := p.result;
        }
        then done;
    }
}
