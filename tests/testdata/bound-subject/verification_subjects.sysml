package Structure {
    part def Boiler { attribute speed : Real default = 1.2; }
    part boilerAsMaintained : Boiler;
    part spareBoiler : Boiler;
}
package Verification {
    private import ScalarValues::*;
    verification def SizingCheck {
        subject s : Structure::Boiler;
        VerificationCases::PassIf(s.speed <= 1.5)
    }
    verification maintainedSizing : SizingCheck {
        subject :>> s = Structure::boilerAsMaintained;
    }
    verification spareSizing : SizingCheck {
        subject s = Structure::spareBoiler;
    }
    verification unboundSizing : SizingCheck;
}
