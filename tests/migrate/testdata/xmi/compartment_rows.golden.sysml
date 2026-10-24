package Model {
    package Blocks {
        part def Vehicle {
            attribute speed : ScalarValues::Real;
            attribute mass : Mass;
            constraint limit : MaxSpeed;
            port fuel;
            part engine : Engine;
            attribute mode : Mode;
            private attribute trim : Mode;
        }
        part def Engine;
        constraint def MaxSpeed;
        attribute def Mass;
        enum def Mode {
            eco;
            sport;
        }
        connection def EngineToVehicle {
            end engine : Engine crosses vehicle.engine;
            end vehicle : Vehicle;
            metadata MigrationMetadata::SynthesizedName about vehicle;
        }
        part engine : Engine;
        part vehicle : Vehicle;
        connection 'engine to vehicle' : EngineToVehicle connect engine to vehicle;
        view 'Vehicle BDD' {
            expose Vehicle;
            expose Blocks::Vehicle::fuel;
            expose Engine;
            expose Mode;
            expose EngineToVehicle;
            @DiagramLayout::Canvas { unit = "px"; width = 480; height = 320; }
            metadata DiagramLayout::Layout about Vehicle { x = 40; y = 20; width = 160; height = 120; }
            metadata DiagramLayout::Layout about Blocks::Vehicle::fuel { x = 195; y = 60; width = 10; height = 10; }
            metadata DiagramLayout::Layout about Engine { x = 40; y = 220; width = 120; height = 50; }
            metadata DiagramLayout::Layout about Mode { x = 300; y = 220; width = 120; height = 60; }
            metadata DiagramLayout::Route about EngineToVehicle { points = (100, 220, 100, 140); }
            metadata DiagramLayout::Route about Blocks::Vehicle::mode { points = (200, 100, 360, 220); }
            render Views::asTreeDiagram;
        }
        metadata MigrationMetadata::SynthesizedName about EngineToVehicle, 'engine to vehicle';
    }
}
