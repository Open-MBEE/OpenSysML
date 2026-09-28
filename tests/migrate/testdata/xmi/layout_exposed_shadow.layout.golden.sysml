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
            expose Structure::Scale::text;
            expose Structure::Scale::width;
            @DiagramLayout::Canvas { unit = "px"; :>> DiagramLayout::Canvas::width = 435; :>> DiagramLayout::Canvas::height = 100; }
            metadata DiagramLayout::Layout about Vehicle { x = 10; y = 20; :>> DiagramLayout::Layout::width = 200; :>> DiagramLayout::Layout::height = 80; }
            metadata DiagramLayout::Layout about Structure::Scale::height { x = 240; y = 20; :>> DiagramLayout::Layout::width = 100; :>> DiagramLayout::Layout::height = 40; }
            metadata DiagramLayout::Route about drive { :>> DiagramLayout::Route::points = (240, 40, 120, 40); }
            @DiagramLayout::Picture { location = "images/_symbol_picture.png"; x = 400; y = 20; :>> DiagramLayout::Picture::width = 35; :>> DiagramLayout::Picture::height = 35; }
            metadata DiagramLayout::Style about Vehicle { :>> DiagramLayout::Style::text = "#FF0000"; }
            render Views::asInterconnectionDiagram;
        }
    }
    part def Engine;
    part def Wheel;
    part def Scale {
        attribute height : ScalarValues::Real;
        attribute points : ScalarValues::Real;
        attribute text : ScalarValues::String;
        attribute width : ScalarValues::Real;
    }
}
