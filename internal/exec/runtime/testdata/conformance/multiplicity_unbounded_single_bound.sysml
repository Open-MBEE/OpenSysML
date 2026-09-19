// A single unbounded bound `[*]` is `0..*`: no object is required, so the
// collection materializes empty, exactly as the explicit `[0..*]` does.
package test {
    private import ScalarValues::*;

    part def C {
        attribute m : Real = 1.0;
    }

    part def Bounds {
        part starred : C[*];
        part explicit : C[0..*];
        part required : C[2..*];
        part single : C[1];
    }
}
