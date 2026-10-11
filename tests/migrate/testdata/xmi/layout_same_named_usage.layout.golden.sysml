package UI {
    part def GUI;
    part def Panel;
    part def Console {
        part GUI : UI::GUI;
        part panel : Panel;
    }
    view 'Console BDD' {
        expose Console;
        expose UI::GUI;
        expose Panel;
        expose UI::Console::GUI;
        expose UI::Console::panel;
        @DiagramLayout::Canvas { unit = "px"; width = 330; height = 237; }
        metadata DiagramLayout::Layout about Console { x = 100; y = 20; width = 200; height = 76; }
        metadata DiagramLayout::Layout about UI::GUI { x = 20; y = 200; width = 80; height = 37; }
        metadata DiagramLayout::Layout about Panel { x = 250; y = 200; width = 80; height = 37; }
        metadata DiagramLayout::Layout about UI::Console::GUI { x = 110; y = 90; width = 10; height = 10; }
        metadata DiagramLayout::Layout about UI::Console::panel { x = 280; y = 90; width = 10; height = 10; }
        render Views::asTreeDiagram;
    }
}
