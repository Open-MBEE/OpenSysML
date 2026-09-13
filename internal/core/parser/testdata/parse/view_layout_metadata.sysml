package VehicleLayout {
    private import DiagramLayout::*;
    private import Views::*;

    part def Engine {
        // The position every view that does not place the engine falls back to.
        @Layout { x = 0; y = 0; }
    }
    part def Wheel;
    part def Vehicle {
        part engine : Engine;
        part wheel : Wheel[4];
        connection drive connect engine to wheel;
    }

    view vehicleView {
        expose Vehicle::**;
        render asInterconnectionDiagram;

        @Canvas { unit = "px"; width = 1200; height = 800; }
        metadata Layout about Vehicle::engine { x = 120; y = 80; width = 200; height = 90; }
        metadata Layout about Vehicle::wheel { x = 480; y = 80; collapsed = true; }
        metadata Route about Vehicle::drive { points = (320, 125, 400, 125, 480, 125); }
    }
}
