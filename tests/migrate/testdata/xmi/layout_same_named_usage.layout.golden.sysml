package Model {
    package UI {
        part def GUI;
        part def Panel;
        part def Console {
            part GUI : UI::GUI;
            part panel : Panel;
        }
        connection def GUIToConsole {
            end GUI : UI::GUI crosses console.GUI;
            end console : Console;
            metadata MigrationMetadata::SynthesizedName about console;
        }
        connection def PanelToConsole {
            end panel : Panel crosses console.panel;
            end console : Console;
            metadata MigrationMetadata::SynthesizedName about console;
        }
        part gUI : GUI;
        part console : Console;
        connection 'gUI to console' : GUIToConsole connect gUI to console;
        part panel : Panel;
        connection 'panel to console' : PanelToConsole connect panel to console;
        view 'Console BDD' {
            expose Console;
            expose UI::GUI;
            expose Panel;
            expose UI::Console::GUI;
            expose UI::Console::panel;
            expose GUIToConsole;
            expose PanelToConsole;
            @DiagramLayout::Canvas { unit = "px"; width = 330; height = 237; }
            metadata DiagramLayout::Layout about Console { x = 100; y = 20; width = 200; height = 76; }
            metadata DiagramLayout::Layout about UI::GUI { x = 20; y = 200; width = 80; height = 37; }
            metadata DiagramLayout::Layout about Panel { x = 250; y = 200; width = 80; height = 37; }
            metadata DiagramLayout::Layout about UI::Console::GUI { x = 110; y = 90; width = 10; height = 10; }
            metadata DiagramLayout::Layout about UI::Console::panel { x = 280; y = 90; width = 10; height = 10; }
            metadata DiagramLayout::Route about GUIToConsole { points = (60, 200, 115, 100); }
            metadata DiagramLayout::Route about PanelToConsole { points = (290, 200, 285, 100); }
            render Views::asTreeDiagram;
        }
        metadata MigrationMetadata::SynthesizedName about GUIToConsole, PanelToConsole, 'gUI to console', 'panel to console';
    }
}
