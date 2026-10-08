package ViewsModel {
    part def Pump {
        attribute rate : ScalarValues::Real;
        ref part outlet : Valve;
        view Bare {
            render Views::asTextualNotation;
        }
        view diagram {
            expose rate;
            render Views::asInterconnectionDiagram;
        }
    }
    part def Valve;
    connection def Supplies {
        end source : Pump;
        end sink : Valve;
        view Supplies {
            expose ViewsModel::Supplies;
            expose source;
            expose ViewsModel::Pump::outlet;
            render Views::asTreeDiagram;
        }
    }
    enum def Mode {
        on;
        off;
    }
    view Dangling {
        render Views::asElementTable;
    }
    view Empty {
        render Views::asTreeDiagram;
    }
    view Feeds {
        render Views::asTreeDiagram;
    }
    view Modes {
        expose ViewsModel::Mode::on;
        expose ViewsModel::Mode::off;
        render Views::asElementTable;
    }
    view Untyped {
        expose Valve;
        render Views::asTextualNotation;
    }
}
package Spares {
    occurrence def Seal;
}
metadata MigrationMetadata::LibraryNameAvoided about ViewsModel { sourceName = "Views"; }
view Ownerless {
    expose ViewsModel::Pump;
    expose ViewsModel::Pump::rate;
    render Views::asTreeDiagram;
}
view 'Lost Owner' {
    expose ViewsModel::Valve;
    render Views::asInterconnectionDiagram;
}
view Inventory {
    expose Spares;
    expose Spares::Seal;
    expose ViewsModel::Pump;
    render Views::asTreeDiagram;
}
