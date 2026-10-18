package Battery {
    private import ISQ::*;
    private import SI::*;
    private import NumericalFunctions::sum;

    calc def StoredEnergy {
        in voltage : ElectricPotentialValue;
        in charge : ElectricChargeValue;
        return energy : EnergyValue = voltage * charge;
    }

    calc def Runtime {
        in energy : EnergyValue;
        in power : PowerValue;
        return duration : DurationValue = energy / power;
    }

    constraint def RuntimeAtLeast {
        in runtime : DurationValue;
        in minimum : DurationValue;
        runtime >= minimum
    }

    part def Cell {
        attribute voltage : ElectricPotentialValue = 3.7 [V];
        attribute charge : ElectricChargeValue = 10800 [C];
        attribute energy : EnergyValue = StoredEnergy(voltage, charge);
    }

    part def Device {
        part cells : Cell[4];
        attribute drawPower : PowerValue = 5 [W];
        attribute packEnergy : EnergyValue = sum(cells.energy);
        attribute runtime : DurationValue = Runtime(packEnergy, drawPower);
        attribute requiredRuntime : DurationValue = 28800 [s];
        assert constraint lastsLongEnough : RuntimeAtLeast {
            in runtime = runtime;
            in minimum = requiredRuntime;
        }
    }
}
