package Ordered {
    attribute def Real;
    calc def Mixed {
        in x : Real;
        { in a : Real; private attribute k : Real = 1; in b : Real; a + b + k * x }
    }
    calc def Declares {
        { private attribute k : Real = 1; }
    }
}
