package HeatingSystem {
    item def HotWater;

    port def WaterOutPort {
        out item water : HotWater;
    }

    part def Boiler {
        port outlet : WaterOutPort;
    }

    part def Radiator {
        port inlet : ~WaterOutPort;
    }

    part def House {
        part boiler : Boiler;
        part livingRoomRadiator : Radiator;
        part bedroomRadiator : Radiator;
        flow of HotWater from boiler.outlet.water to livingRoomRadiator.inlet.water;
        flow of HotWater from boiler.outlet.water to bedroomRadiator.inlet.water;
    }
}
