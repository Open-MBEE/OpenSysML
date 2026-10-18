package Beam {
    private import ISQ::*;
    private import SI::*;
    private import ScalarValues::Real;

    calc def Stress {
        in force : ForceValue;
        in area : AreaValue;
        return stress : PressureValue = force / area;
    }

    calc def SafetyFactor {
        in yieldStrength : PressureValue;
        in stress : PressureValue;
        return factor : Real = yieldStrength / stress;
    }

    constraint def MinimumSafetyFactor {
        in factor : Real;
        in required : Real;
        factor >= required
    }

    part def SteelBeam {
        attribute load : ForceValue = 50000 [N];
        attribute crossSection : AreaValue = 0.002 [m ** 2];
        attribute yieldStrength : PressureValue = 250000000 [Pa];
        attribute stress : PressureValue = Stress(load, crossSection);
        attribute safetyFactor : Real = SafetyFactor(yieldStrength, stress);
        attribute requiredFactor : Real = 2.0;
        assert constraint safe : MinimumSafetyFactor {
            in factor = safetyFactor;
            in required = requiredFactor;
        }
    }
}
