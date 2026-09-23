package Structure {
    part def Vehicle {
        part engine : Engine;
        part wheel : Wheel;
        connection drive connect engine to wheel;
        view 'Vehicle Internals' {
            expose Vehicle;
            expose engine;
            expose wheel;
            expose drive;
            expose Behavior::Drive::'start to step';
            @DiagramLayout::Canvas { unit = "px"; width = 300; height = 50; }
            metadata DiagramLayout::Layout about engine { x = 20; y = 10; width = 100; height = 40; }
            metadata DiagramLayout::Layout about wheel { x = 200; y = 10; width = 100; height = 40; }
            metadata DiagramLayout::Route about drive { points = (120, 30, 200, 30); }
            render Views::asInterconnectionDiagram;
        }
    }
    part def Engine;
    part def Wheel;
    view 'Vehicle Structure' {
        expose Vehicle;
        expose Engine;
        expose Wheel;
        expose Structure::Vehicle::engine;
        expose Structure::Vehicle::drive;
        expose Behavior::Drive::'step';
        @DiagramLayout::Canvas { unit = "px"; width = 340; height = 160; }
        metadata DiagramLayout::Layout about Vehicle { x = 10; y = 20; width = 200; height = 80; }
        metadata DiagramLayout::Layout about Engine { x = 240; y = 20; width = 100; height = 60; }
        metadata DiagramLayout::Layout about Behavior::Drive::'step' { x = 240; y = 120; width = 100; height = 40; }
        render Views::asTreeDiagram;
    }
}
package Behavior {
    action def Drive {
        succession 'start to step' first start then 'step';
        action 'step' {
            /* body not migrated (the name "going" resolves to nothing readable: nothing visible from Behavior::Drive::step is called going) {JavaScript}:
             * going = true;
             */
        }
    }
}
