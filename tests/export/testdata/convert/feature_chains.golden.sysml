package Chains {
    port def IP;
    part def IU {
        port pip : IP;
    }
    part def LV {
        part instrumentUnit : IU;
    }
    interface def II {
        end port a : IP;
        end port b : IP;
    }
    part ctx {
        part launchVehicle : LV;
        part supplier : IU;
        connect launchVehicle.instrumentUnit.pip to supplier.pip;
        interface ic : II connect launchVehicle.instrumentUnit.pip to supplier.pip;
        attribute x = launchVehicle.instrumentUnit.pip;
        ref r redefines launchVehicle.instrumentUnit;
    }
}
