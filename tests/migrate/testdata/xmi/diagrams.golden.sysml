package Structure {
    doc /* The rolling stock. */
    part def Vehicle {
        part engine : Engine;
        part wheel : Wheel;
        attribute mass : ScalarValues::Real;
        connect engine to wheel;
        view 'Vehicle Internals' {
            expose Vehicle;
            expose engine;
            expose wheel;
            expose Structure::Engine::power;
            render Views::asInterconnectionDiagram;
        }
        view 'Engine Matrix' {
            expose Engine;
            render Views::asElementTable;
        }
    }
    part def Engine {
        attribute power : ScalarValues::Real;
        part wheel : Wheel;
    }
    part def Wheel;
    view 'Vehicle Structure' {
        expose Vehicle;
        expose Engine;
        expose Wheel;
        expose Structure::Vehicle::engine;
        expose Structure::Vehicle::mass;
        expose Structure::Engine::wheel;
        expose Behavior::Drive;
        expose ScalarValues::Real;
        render Views::asTreeDiagram;
    }
    view 'Vehicle Structure 2' {
        expose Structure;
        expose Behavior;
        render Views::asTreeDiagram;
    }
    view 'Engine 2' {
        expose Engine;
        render Views::asTextualNotation;
    }
}
package Behavior {
    action def Drive {
        view 'Drive Flow' {
            render Views::asTextualNotation;
        }
        view 'start engine 2' {
            render Views::asTextualNotation;
        }
        first start then 'start engine';
        action 'start engine' {
            /* body not migrated (the name "running" resolves to nothing readable: nothing visible from Behavior::Drive::start engine is called running) {JavaScript}:
             * running = true;
             */
        }
        first 'start engine' then final;
        action final terminate;
    }
    state def Modes {
        view 'Idle 2' {
            render Views::asTextualNotation;
        }
        entry; then Idle;
        state Idle;
        state Running;
        transition first Idle then Running;
    }
}
view 'Block Table' {
    expose Structure::Vehicle;
    expose Structure::Engine;
    expose Structure::Engine::power;
    render Views::asElementTable;
}
