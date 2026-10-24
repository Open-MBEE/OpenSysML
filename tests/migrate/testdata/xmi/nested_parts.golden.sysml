package Model {
    package Blocks {
        part def Vehicle {
            part engine : Engine;
            view 'Vehicle Internals' {
                expose engine;
                expose Blocks::Engine::piston;
                expose Blocks::Engine::sensor;
                expose Blocks::Engine::'piston to sensor';
                @DiagramLayout::Canvas { unit = "px"; width = 500; height = 300; }
                metadata DiagramLayout::Layout about engine { x = 20; y = 20; width = 440; height = 240; }
                metadata DiagramLayout::Layout about Blocks::Engine::piston { x = 60; y = 100; width = 120; height = 60; }
                metadata DiagramLayout::Layout about Blocks::Engine::sensor { x = 300; y = 100; width = 120; height = 60; }
                metadata DiagramLayout::Route about Blocks::Engine::'piston to sensor' { points = (180, 130, 300, 130); }
                render Views::asInterconnectionDiagram;
            }
        }
        part def Engine {
            part piston : Piston;
            part sensor : Sensor;
            connection 'piston to sensor' connect piston to sensor;
            metadata MigrationMetadata::SynthesizedName about 'piston to sensor';
        }
        part def Piston;
        part def Sensor;
    }
}
