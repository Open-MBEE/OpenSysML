package Circuit {
    private import ISQ::*;
    private import SI::*;

    calc def OhmsLaw {
        in voltage : ElectricPotentialValue;
        in resistance : ResistanceValue;
        return current : ElectricCurrentValue = voltage / resistance;
    }

    calc def Power {
        in voltage : ElectricPotentialValue;
        in current : ElectricCurrentValue;
        return power : PowerValue = voltage * current;
    }

    part def Heater {
        attribute supplyVoltage : ElectricPotentialValue = 230 [V];
        attribute resistance : ResistanceValue = 26.45 [ohm];
        attribute current : ElectricCurrentValue = OhmsLaw(supplyVoltage, resistance);
        attribute power : PowerValue = Power(supplyVoltage, current);
    }
}
