package M {
    private import ScalarValues::*;

    part def C {
        attribute m : Real = 1.0;
    }

    part def Holder {
        attribute k : Integer = 3;
        part large : C[5000];
        part scaled : C[k * 1000];
        part named : C[k..*];
        part arithmetic : C[3 + 2];
        part unbounded : C[*..*];
    }
}
