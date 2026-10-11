package Blocks {
    part def Controller {
        part sensors : Sensor;
    }
    part def Sensor {
        ref part controller : Controller;
    }
    view 'Pair BDD' {
        expose Controller;
        expose Sensor;
        expose Blocks::Controller::sensors;
        expose Blocks::Sensor::controller;
        @DiagramLayout::Canvas { unit = "px"; width = 400; height = 300; }
        metadata DiagramLayout::Layout about Controller { x = 40; y = 20; width = 120; height = 50; }
        metadata DiagramLayout::Layout about Sensor { x = 40; y = 200; width = 120; height = 50; }
        metadata DiagramLayout::Route about Blocks::Controller::sensors { points = (100, 70, 100, 130, 100, 200); }
        metadata DiagramLayout::Route about Blocks::Sensor::controller { points = (100, 200, 100, 130, 100, 70); }
        render Views::asTreeDiagram;
    }
    view 'Sensor BDD' {
        expose Controller;
        expose Sensor;
        expose Blocks::Sensor::controller;
        @DiagramLayout::Canvas { unit = "px"; width = 400; height = 300; }
        metadata DiagramLayout::Layout about Controller { x = 240; y = 20; width = 120; height = 50; }
        metadata DiagramLayout::Layout about Sensor { x = 40; y = 200; width = 120; height = 50; }
        metadata DiagramLayout::Route about Blocks::Sensor::controller { points = (160, 225, 300, 225, 300, 70); }
        render Views::asTreeDiagram;
    }
}
