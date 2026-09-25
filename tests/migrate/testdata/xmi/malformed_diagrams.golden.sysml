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
    connection def Supplies {
        end source : Pump;
        end sink : Valve;
        view Supplies {
            expose Views::Supplies;
            expose source;
            expose Views::Pump::outlet;
            render $::Views::asTreeDiagram;
        }
    }
    enum def Mode {
        on;
        off;
    }
    view Dangling {
        render $::Views::asElementTable;
    }
    view Empty {
        render $::Views::asTreeDiagram;
    }
    view Feeds {
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
}
package Spares {
    part def Seal;
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
view Inventory {
    expose Spares;
    expose Spares::Seal;
    expose Views::Pump;
    render $::Views::asTreeDiagram;
}
