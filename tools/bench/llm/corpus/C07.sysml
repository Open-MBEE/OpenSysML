package Assembly {
    private import ISQ::*;
    private import SI::*;
    private import NumericalFunctions::sum;

    part def Component {
        attribute mass :> ISQ::mass;
    }

    constraint def TotalMassLimit {
        in parts : MassValue[0..*];
        in limit : MassValue;
        sum(parts) <= limit
    }

    part def Chassis :> Component {
        attribute redefines mass = 120 [kg];
    }

    part def Motor :> Component {
        attribute redefines mass = 45 [kg];
    }

    part def BatteryPack :> Component {
        attribute redefines mass = 300 [kg];
    }

    part def Vehicle {
        part chassis : Chassis;
        part motor : Motor;
        part battery : BatteryPack;
        attribute massLimit : MassValue = 500 [kg];
        assert constraint massBudget : TotalMassLimit {
            in parts = (chassis.mass, motor.mass, battery.mass);
            in limit = massLimit;
        }
    }
}
