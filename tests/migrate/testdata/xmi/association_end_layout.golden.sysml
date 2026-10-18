package Model {
    package Blocks {
        part def Vehicle {
            part engine : Engine;
        }
        part def Engine {
            part piston : Piston;
        }
        part def Piston;
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
            expose Engine;
            expose Blocks::Engine::piston;
            expose EngineToVehicle;
            @DiagramLayout::Canvas { unit = "px"; width = 400; height = 300; }
            metadata DiagramLayout::Layout about Vehicle { x = 40; y = 20; width = 120; height = 50; }
            metadata DiagramLayout::Layout about Engine { x = 40; y = 160; width = 120; height = 90; }
            metadata DiagramLayout::Layout about Blocks::Engine::piston { x = 55; y = 200; width = 90; height = 30; }
            metadata DiagramLayout::Route about EngineToVehicle { points = (100, 160, 100, 70); }
            render Views::asTreeDiagram;
        }
        metadata MigrationMetadata::SynthesizedName about EngineToVehicle, 'engine to vehicle';
    }
}
