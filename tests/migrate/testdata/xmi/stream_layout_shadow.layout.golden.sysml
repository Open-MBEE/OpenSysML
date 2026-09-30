package Structure {
    part def Vehicle {
        attribute height : ScalarValues::Real;
        view 'Vehicle Picture' {
            @DiagramLayout::Canvas { unit = "px"; width = 40; :>> DiagramLayout::Canvas::height = 60; }
            @DiagramLayout::Picture { location = "images/_symbol_picture.png"; x = 10; y = 20; width = 30; :>> DiagramLayout::Picture::height = 40; }
            render Views::asInterconnectionDiagram;
        }
    }
}
