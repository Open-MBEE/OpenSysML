package Charger {
    item def Electricity;

    port def PowerOutPort {
        out item power : Electricity;
    }

    part def PowerSupply {
        port supply : PowerOutPort;
    }

    part def Battery {
        port charge : ~PowerOutPort;
    }

    part def ChargingStation {
        part psu : PowerSupply;
        part battery : Battery;
        connect psu.supply to battery.charge;
    }
}
