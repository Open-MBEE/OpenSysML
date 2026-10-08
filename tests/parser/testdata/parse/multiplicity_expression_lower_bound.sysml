package M {
    private import ScalarValues::*;

    part def C {
        attribute m : Real = 1.0;
    }

    part def Holder {
        attribute k : Integer = 3;
        attribute scaledBound : Integer = k * 1000;
        attribute arithmeticBound : Integer = 3 + 2;
        part large : C[5000];
        part scaled : C[scaledBound];
        part named : C[k..*];
        part calculated : C[arithmeticBound];
        part unbounded : C[*..*];
    }
}
