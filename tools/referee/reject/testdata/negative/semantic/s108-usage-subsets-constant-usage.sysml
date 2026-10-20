// Invalid: a usage that may vary in time cannot subset or redefine a constant one unless it is constant (KerML 8.3.3.3.9 validateSubsettingConstantConformance; pilot validateSubsettingConstantConformance).
package S108UsageSubsetsConstantUsage {
    private import ScalarValues::*;
    part def P {
        constant attribute c : Real;
        attribute d :> c;
    }
    part def Q :> P {
        attribute e :>> c;
    }
}
