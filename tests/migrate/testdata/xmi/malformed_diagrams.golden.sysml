package Model {
    package Views {
        part def Pump {
            attribute rate : ScalarValues::Real;
            ref part outlet : Valve;
            view Bare {
                render $::Views::asTextualNotation;
            }
            view diagram {
                expose rate;
                render $::Views::asInterconnectionDiagram;
            }
        }
        part def Valve;
        connection def ValveToPump {
            end outlet : Valve crosses pump.outlet;
            end pump : Pump;
            view Feeds {
                expose ValveToPump;
                render $::Views::asTreeDiagram;
            }
            metadata MigrationMetadata::SynthesizedName about pump;
        }
        connection def Supplies {
            end source : Pump;
            end sink : Valve;
            view Supplies {
                expose Views::Supplies;
                expose Views::Pump::outlet;
                render $::Views::asTreeDiagram;
            }
        }
        enum def Mode {
            on;
            off;
        }
        part valve : Valve;
        part pump : Pump;
        connection 'valve to pump' : ValveToPump connect valve to pump;
        connection supplies : Supplies connect pump to valve;
        view Dangling {
            render $::Views::asElementTable;
        }
        view Empty {
            render $::Views::asTreeDiagram;
        }
        view Modes {
            expose Views::Mode::on;
            expose Views::Mode::off;
            render $::Views::asElementTable;
        }
        view Untyped {
            expose Valve;
            render $::Views::asTextualNotation;
        }
        metadata MigrationMetadata::SynthesizedName about ValveToPump, 'valve to pump', supplies;
    }
    view Ownerless {
        expose Views::Pump;
        expose Views::Pump::rate;
        render $::Views::asTreeDiagram;
    }
    view 'Lost Owner' {
        expose Views::Valve;
        render $::Views::asInterconnectionDiagram;
    }
}
package Spares {
    occurrence def Seal;
}
view Inventory {
    expose Spares;
    expose Spares::Seal;
    expose Model::Views::Pump;
    render Views::asTreeDiagram;
}
