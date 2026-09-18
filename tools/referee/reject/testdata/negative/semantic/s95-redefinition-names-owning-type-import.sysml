// Invalid: a redefinition owned by a feature of a type does not see what the owning type itself imports (KerML 8.2.3.5.2 redefinition name resolution; pilot `Couldn't resolve reference to Feature 'x'`).
package S95RedefinitionNamesOwningTypeImport {
    private import ScalarValues::Real;
    package Lib {
        attribute x : Real;
    }
    part def B {
        private import Lib::*;
        attribute z :>> x;
    }
}
