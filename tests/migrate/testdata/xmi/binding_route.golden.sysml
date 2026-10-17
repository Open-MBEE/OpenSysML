package Model {
    package Blocks {
        part def Vehicle {
            attribute mass : ScalarValues::Real;
            attribute limit : ScalarValues::Real;
            binding 'mass = limit' bind mass = limit;
            view 'Vehicle Internals' {
                expose mass;
                expose limit;
                expose 'mass = limit';
                @DiagramLayout::Canvas { unit = "px"; width = 400; height = 200; }
                metadata DiagramLayout::Layout about mass { x = 20; y = 90; width = 100; height = 40; }
                metadata DiagramLayout::Layout about limit { x = 200; y = 90; width = 100; height = 40; }
                metadata DiagramLayout::Route about 'mass = limit' { points = (120, 110, 200, 110); }
                render Views::asInterconnectionDiagram;
            }
            metadata MigrationMetadata::SynthesizedName about 'mass = limit';
        }
    }
}
