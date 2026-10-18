package Pump {
    private import ScalarValues::Real;

    port def FluidPort {
        attribute flowRate : Real;
        attribute pressure : Real;
    }

    part def Pump {
        port inlet : FluidPort;
        port outlet : FluidPort;
    }
}
