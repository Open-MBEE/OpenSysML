package Bodies {
    attribute def Real;
    calc def Scaled {
        in x : Real;
        { in y : Real; private attribute k : Real = 2; y * k + x }
    }
}
