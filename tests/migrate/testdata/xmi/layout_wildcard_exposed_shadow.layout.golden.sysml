package Structure {
    part def Vehicle {
        part engine : Engine;
    }
    part def Engine;
    part def Scale {
        attribute height : ScalarValues::Real;
    }
}
view Overview {
    expose Structure::**;
    view Layout {
        expose Structure::Vehicle;
        expose Structure::Vehicle::engine;
        @DiagramLayout::Canvas { unit = "px"; width = 340; :>> DiagramLayout::Canvas::height = 180; }
        metadata DiagramLayout::Layout about Structure::Vehicle { x = 10; y = 20; width = 200; :>> DiagramLayout::Layout::height = 80; }
        metadata DiagramLayout::Layout about Structure::Vehicle::engine { x = 240; y = 140; width = 100; :>> DiagramLayout::Layout::height = 40; }
        render Views::asInterconnectionDiagram;
    }
}
