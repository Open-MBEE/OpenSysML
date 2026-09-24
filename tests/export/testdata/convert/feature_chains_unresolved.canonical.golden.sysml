package Chains {
    port def IP;
    part def IU {
        port pip : IP;
    }
    part ctx {
        part launchVehicle : Ext::LV;
        part supplier : IU;
        connect launchVehicle.instrumentUnit.pip to supplier.pip;
        ref r redefines launchVehicle.instrumentUnit;
    }
}
