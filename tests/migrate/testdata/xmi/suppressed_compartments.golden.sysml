package Model {
    package Optics {
        part def Camera {
            attribute resolution : ScalarValues::Integer;
            part lens : Lens;
            ref part mount : Mount;
            port data;
            abstract action capture;
        }
        part def Lens {
            attribute focalLength : ScalarValues::Real;
        }
        part def Mount {
            attribute thread : ScalarValues::String;
        }
        view 'Camera BDD' {
            expose Camera;
            expose Optics::Camera::data;
            expose Lens;
            expose Mount;
            @DiagramLayout::Canvas { unit = "px"; width = 480; height = 320; }
            metadata DiagramLayout::Layout about Camera { x = 40; y = 20; width = 200; height = 100; showAttributes = false; showParts = false; showPorts = false; showReferences = false; }
            metadata DiagramLayout::Layout about Optics::Camera::data { x = 235; y = 60; width = 10; height = 10; }
            metadata DiagramLayout::Layout about Lens { x = 40; y = 220; width = 120; height = 33; showActions = false; showAttributes = false; showPorts = false; showReferences = false; }
            metadata DiagramLayout::Layout about Mount { x = 300; y = 220; width = 120; height = 60; }
            render Views::asTreeDiagram;
        }
    }
}
