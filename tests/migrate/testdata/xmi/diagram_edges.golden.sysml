package Structure {
    part def Component;
    part def Vehicle {
        part engine : Engine;
        part wheel : Wheel;
        attribute mass : ScalarValues::Real;
        attribute limit : ScalarValues::Real;
        attribute speed : ScalarValues::Real;
        connection 'engine to wheel' connect engine to wheel;
        connection 'engine to wheel 2' connect engine to wheel;
        connect engine to wheel;
        connection drive connect engine to wheel;
        binding 'mass = limit' bind mass = limit;
        action Drive {
            view Driving : StandardViewDefinitions::ActionFlowView {
                expose Drive;
                expose 'set speed.value = target';
                render Views::asInterconnectionDiagram;
            }
            view 'Driving Sketch' {
                render Views::asTextualNotation;
            }
            in target : ScalarValues::Real[1];
            succession 'start to gain' first start then gain;
            action gain {
                out result[1] = 2.5;
            }
            first gain then 'set speed';
            action 'set speed' {
                in value[1];
                assign speed := value;
            }
            succession finish first 'set speed' then final;
            action final terminate;
            flow 'gain.result to set speed.value' from gain.result to 'set speed'.value;
            binding 'set speed.value = target' bind 'set speed'.value = target;
            metadata MigrationMetadata::SynthesizedName about 'start to gain', final, 'gain.result to set speed.value', 'set speed.value = target';
        }
        satisfy requirement 'satisfy Mass Limit' :> RequirementsModel::'Mass Limit';
        satisfy RequirementsModel::'Speed Limit';
        satisfy requirement 'satisfy Speed Limit' :> RequirementsModel::'Speed Limit';
        view 'Vehicle Internals' {
            expose engine;
            expose wheel;
            expose mass;
            expose limit;
            expose 'engine to wheel';
            expose 'engine to wheel 2';
            expose drive;
            expose 'mass = limit';
            render Views::asInterconnectionDiagram;
        }
        metadata MigrationMetadata::SynthesizedName about 'engine to wheel', 'engine to wheel 2', 'mass = limit', 'satisfy Mass Limit', 'satisfy Speed Limit';
    }
    part def Engine :> Component {
        satisfy requirement 'satisfy Speed Limit' :> RequirementsModel::'Speed Limit';
        metadata MigrationMetadata::SynthesizedName about 'satisfy Speed Limit';
    }
    part def Wheel;
    connection def WheelToVehicle {
        end wheel : Wheel crosses vehicle.wheel;
        end vehicle : Vehicle;
    }
    dependency 'Vehicle to Engine' from Vehicle to Engine;
    dependency Vehicle to Wheel;
    dependency 'Engine to Component' from Engine to Component;
    dependency 'Wheel to Component' from Wheel to Component;
    part wheel : Wheel;
    part vehicle : Vehicle;
    connection 'wheel to vehicle' : WheelToVehicle connect wheel to vehicle;
    view Structure {
        expose Component;
        expose Vehicle;
        expose Engine;
        expose Wheel;
        expose 'Vehicle to Engine';
        expose 'Engine to Component';
        expose 'Wheel to Component';
        expose WheelToVehicle;
        render Views::asTreeDiagram;
    }
    metadata MigrationMetadata::SynthesizedName about WheelToVehicle, 'Vehicle to Engine', 'Engine to Component', 'Wheel to Component';
}
package Behavior {
    item def Go;
    item def Stop;
    item def Resume;
    state def Modes {
        view Modes : StandardViewDefinitions::StateTransitionView {
            expose Behavior::Modes;
            render Views::asInterconnectionDiagram;
        }
        entry; then Idle;
        state Idle;
        state Running {
            do action spin {
                /* body not migrated (the statement "keep turning" is not an assignment of a v2 expression (language English)) {English}:
                 * keep turning
                 */
            }
        }
        state Halted;
        transition 'Idle accept Go then Running' first Idle accept Go then Running;
        transition halt first Running accept Stop then Halted;
        transition first Halted then Idle;
        transition 'Halted accept Go then Running' first Halted accept Go then Running;
        transition 'Halted accept Resume then Running' first Halted accept Resume then Running;
        metadata MigrationMetadata::SynthesizedName about 'Idle accept Go then Running', 'Halted accept Go then Running', 'Halted accept Resume then Running';
    }
}
package RequirementsModel {
    requirement <R1> 'Mass Limit' {
        doc /* The vehicle shall weigh less than 1500 kg. */
    }
    requirement <R2> 'Speed Limit' {
        doc /* The vehicle shall not exceed 120 km/h. */
    }
    verification def 'Mass Test' {
        objective 'objective' {
            verify requirement 'verify Mass Limit' :> 'Mass Limit';
            metadata MigrationMetadata::SynthesizedName about 'verify Mass Limit';
        }
        metadata MigrationMetadata::SynthesizedName about 'objective';
    }
    view Traceability {
        expose 'Mass Limit';
        expose Structure::Vehicle;
        expose 'Mass Test';
        expose Structure::Vehicle::'satisfy Mass Limit';
        expose RequirementsModel::'Mass Test'::'objective'::'verify Mass Limit';
        expose Structure::Vehicle::'satisfy Speed Limit';
        expose Structure::Engine::'satisfy Speed Limit';
        render Views::asTreeDiagram;
    }
}
package Usage {
    use case 'Drive Vehicle' {
        include 'Start Engine';
        include 'Stop Engine';
    }
    use case 'Start Engine';
    use case 'Stop Engine';
    connection 'Drive Vehicle includes Start Engine' connect 'Drive Vehicle' to 'Start Engine';
    connection 'Drive Vehicle includes Stop Engine' connect 'Drive Vehicle' to 'Stop Engine';
    view 'Driving Cases' {
        expose 'Drive Vehicle';
        expose 'Start Engine';
        expose 'Drive Vehicle includes Start Engine';
        render Views::asInterconnectionDiagram;
    }
}
metadata MigrationMetadata::LibraryNameAvoided about RequirementsModel { sourceName = "Requirements"; }
