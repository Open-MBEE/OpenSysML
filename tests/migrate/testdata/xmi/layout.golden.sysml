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
