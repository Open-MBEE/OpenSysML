package test {
    private import ScalarValues::*;

    calc def C {
        attribute a : Integer := 1;
        calc inner {
            attribute a : Integer := 9;
            C::a
        }
    }

    action def Act {
        attribute a : Integer := 1;
        out attribute r : Integer;
        out attribute w : Integer;
        out attribute calcResult : Integer;
        if true {
            attribute a : Integer := 9;
            assign r := Act::a;
            assign Act::a := 4;
        }
        assign w := a;
        assign calcResult := C::inner();
    }
}
