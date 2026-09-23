package Structure {
    doc /* The rolling stock. */
    part def Vehicle {
        part engine : Engine;
        part wheel : Wheel;
        attribute mass : ScalarValues::Real;
        connection 'engine to wheel' connect engine to wheel;
        view 'Vehicle Internals' {
            expose Vehicle;
            expose engine;
            expose wheel;
            expose 'engine to wheel';
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
        action def Crank {
            view Cranking : StandardViewDefinitions::ActionFlowView {
                expose Crank;
                render Views::asInterconnectionDiagram;
            }
            view Ignition : StandardViewDefinitions::ActionFlowView {
                expose Crank;
                render Views::asInterconnectionDiagram;
            }
            succession 'start to ignite' first start then ignite;
            action ignite {
                /* body not migrated (the name "ignited" resolves to nothing readable: nothing visible from Structure::Engine::Cranking::ignite is called ignited) {JavaScript}:
                 * ignited = true;
                 */
            }
            succession 'ignite to final' first ignite then final;
            action final terminate;
        }
        action crank : Crank;
        action def Halt {
            view Halting : StandardViewDefinitions::ActionFlowView {
                expose Halt;
                expose Engine;
                render Views::asInterconnectionDiagram;
            }
            /* body not migrated (the name "ignited" resolves to nothing readable: nothing visible from Structure::Engine::Halt is called ignited) {JavaScript}:
             * ignited = false;
             */
        }
        action halt : Halt;
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
        view 'Drive Flow' : StandardViewDefinitions::ActionFlowView {
            expose Drive;
            render Views::asInterconnectionDiagram;
        }
        view 'start engine 2' : StandardViewDefinitions::ActionFlowView {
            expose Drive;
            render Views::asInterconnectionDiagram;
        }
        succession 'start to start engine' first start then 'start engine';
        action 'start engine' {
            /* body not migrated (the name "running" resolves to nothing readable: nothing visible from Behavior::Drive::start engine is called running) {JavaScript}:
             * running = true;
             */
        }
        succession 'start engine to final' first 'start engine' then final;
        action final terminate;
    }
    state def Modes {
        view 'Idle 2' : StandardViewDefinitions::StateTransitionView {
            expose Modes;
            render Views::asInterconnectionDiagram;
        }
        entry; then Idle;
        state Idle;
        state Running;
        transition 'Idle then Running' first Idle then Running;
    }
}
package Usage {
    comment /* The driver drives. */
    part def Driver;
    use case def 'Drive Vehicle' {
        subject;
        actor driver : Driver;
    }
    view Driving {
        expose Driver;
        expose 'Drive Vehicle';
        expose 'Drive Vehicle'::driver;
        render Views::asTextualNotation;
    }
}
package Reports {
    view 'Vehicle Overview' {
        expose 'Vehicle Overview';
        view 'Vehicle Overview' {
            expose Structure::Vehicle;
            render Views::asTreeDiagram;
        }
    }
    view Fleet {
        view summary :> 'Vehicle Overview';
        expose 'Fleet 2';
        expose Structure::Vehicle::'Vehicle Internals';
    }
    view 'Fleet 2' {
        expose Structure;
        expose Reports::Fleet::summary;
        render Views::asTreeDiagram;
    }
}
view 'Block Table' {
    expose Structure::Vehicle;
    expose Structure::Engine;
    expose Structure::Engine::power;
    render Views::asElementTable;
}
