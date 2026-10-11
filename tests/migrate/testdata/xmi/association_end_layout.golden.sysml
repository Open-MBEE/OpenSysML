package Blocks {
    part def Vehicle {
        part engine : Engine;
    }
    part def Engine {
        part piston : Piston;
    }
    part def Piston;
    view 'Vehicle BDD' {
        expose Vehicle;
        expose Engine;
        expose Blocks::Vehicle::engine;
        expose Blocks::Engine::piston;
        @DiagramLayout::Canvas { unit = "px"; width = 400; height = 300; }
        metadata DiagramLayout::Layout about Vehicle { x = 40; y = 20; width = 120; height = 50; }
        metadata DiagramLayout::Layout about Engine { x = 40; y = 160; width = 120; height = 90; }
        metadata DiagramLayout::Layout about Blocks::Engine::piston { x = 55; y = 200; width = 90; height = 30; }
        metadata DiagramLayout::Route about Blocks::Vehicle::engine { points = (100, 70, 100, 160); }
        render Views::asTreeDiagram;
    }
}
