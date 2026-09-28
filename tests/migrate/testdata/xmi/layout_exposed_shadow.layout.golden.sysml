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
            expose Scale;
            expose Structure::Scale::height;
            expose Structure::Scale::points;
            @DiagramLayout::Canvas { unit = "px"; width = 340; :>> DiagramLayout::Canvas::height = 100; }
            metadata DiagramLayout::Layout about Vehicle { x = 10; y = 20; width = 200; :>> DiagramLayout::Layout::height = 80; }
            metadata DiagramLayout::Layout about Structure::Scale::height { x = 240; y = 20; width = 100; :>> DiagramLayout::Layout::height = 40; }
            metadata DiagramLayout::Route about drive { :>> DiagramLayout::Route::points = (240, 40, 120, 40); }
            render Views::asInterconnectionDiagram;
        }
    }
    part def Engine;
    part def Wheel;
    part def Scale {
        attribute height : ScalarValues::Real;
        attribute points : ScalarValues::Real;
    }
}
