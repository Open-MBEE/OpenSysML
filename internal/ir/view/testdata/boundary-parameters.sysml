// Use cases whose actor parameters are pins of a positioned node, meeting at one route end, at
// a corner, crowded at the corners and over an unconnected pin; a part whose declared ports
// meet at one route end, sized, unsized and unpositioned, or are routed inside it like its nested
// parts; and
// a migration made up one connection's name.
package BoundaryParameters {
    private import ScalarValues::*;
    part Operator;
    part Administrator;
    part 'Service Technician';
    part Supervisor;
    part Rack;
    part Instrument {
        port sensor;
        port heater;
        part detector;
        part filament;
    }
    connection 'Rack to Sensor' connect Rack to Instrument.sensor;
    connection 'Rack to Heater' connect Rack to Instrument.heater;
    connection 'Rack to Detector' connect Rack to Instrument.detector;
    connection 'Rack to Filament' connect Rack to Instrument.filament;

    use case 'Load Sample' {
        subject;
        actor operator :> Operator;
        actor technician :> 'Service Technician';
        actor administrator :> Administrator;
        actor supervisor :> Supervisor;
    }
    connection 'Operator to Load Sample' connect Operator to 'Load Sample'.operator;
    connection 'Service Technician to Load Sample' connect 'Service Technician' to 'Load Sample'.technician;
    connection 'Administrator to Load Sample' connect Administrator to 'Load Sample'.administrator;
    connection 'Supervisor to Load Sample' connect Supervisor to 'Load Sample'.supervisor;

    use case 'Acquire Image' {
        subject;
        actor operator :> Operator;
    }
    use case 'Configure Instrument' {
        subject;
        actor operator :> Operator;
        actor administrator :> Administrator;
    }

    connection 'Operator to Acquire Image' connect Operator to 'Acquire Image'.operator;
    connection 'Operator to Configure Instrument' connect Operator to 'Configure Instrument'.operator;
    connection 'Administrator to Configure Instrument' connect Administrator to 'Configure Instrument'.administrator;

    // The interconnection rendering drawing the parameter symbols and edge labels
    // the default leaves out: the use-case views below render with it.
    rendering withParameters :> Views::asInterconnectionDiagram {
        attribute showParameterSymbols : Boolean default = true;
        attribute showEdgeLabels : Boolean default = true;
    }

    // The actors left of the use cases, their routes ending on the left border.
    view lab {
        expose Operator;
        expose Administrator;
        expose 'Acquire Image';
        expose 'Configure Instrument';
        expose 'Operator to Acquire Image';
        expose 'Operator to Configure Instrument';
        expose 'Administrator to Configure Instrument';
        render withParameters;

        metadata DiagramLayout::Layout about Operator { x = 40; y = 60; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Administrator { x = 40; y = 220; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Acquire Image' { x = 320; y = 20; width = 160; height = 50; }
        metadata DiagramLayout::Layout about 'Configure Instrument' { x = 320; y = 120; width = 160; height = 50; }
        metadata DiagramLayout::Route about 'Operator to Acquire Image' { points = (120, 82, 320, 45); }
        metadata DiagramLayout::Route about 'Operator to Configure Instrument' { points = (120, 82, 320, 145); }
        metadata DiagramLayout::Route about 'Administrator to Configure Instrument' { points = (120, 242, 320, 145); }
    }

    // The same, mirrored: the actors right of the use cases, the routes ending
    // on the right border, the administrator's coming from above.
    view mirroredLab {
        expose Operator;
        expose Administrator;
        expose 'Configure Instrument';
        expose 'Operator to Configure Instrument';
        expose 'Administrator to Configure Instrument';
        render withParameters;

        metadata DiagramLayout::Layout about Operator { x = 600; y = 220; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Administrator { x = 600; y = 60; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Configure Instrument' { x = 320; y = 120; width = 160; height = 50; }
        metadata DiagramLayout::Route about 'Operator to Configure Instrument' { points = (600, 242, 480, 145); }
        metadata DiagramLayout::Route about 'Administrator to Configure Instrument' { points = (600, 82, 480, 145); }
    }

    // Two routes end at the use case's top-left corner, one from the left and
    // one from above.
    view corner {
        expose Operator;
        expose 'Service Technician';
        expose 'Load Sample';
        expose 'Operator to Load Sample';
        expose 'Service Technician to Load Sample';
        render withParameters;

        metadata DiagramLayout::Layout about Operator { x = 40; y = 98; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Service Technician' { x = 280; y = 20; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Load Sample' { x = 320; y = 120; width = 160; height = 50; }
        metadata DiagramLayout::Route about 'Operator to Load Sample' { points = (120, 120, 320, 120); }
        metadata DiagramLayout::Route about 'Service Technician to Load Sample' { points = (320, 64, 320, 120); }
    }

    // Two routes from above and one from the left meet near the top-left corner; the two
    // symbols from above are together wider than the top side leaves beyond the left one.
    view crowdedCorner {
        expose Operator;
        expose Administrator;
        expose 'Service Technician';
        expose 'Load Sample';
        expose 'Operator to Load Sample';
        expose 'Service Technician to Load Sample';
        expose 'Administrator to Load Sample';
        render withParameters;

        metadata DiagramLayout::Layout about Operator { x = 40; y = 98; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Service Technician' { x = 280; y = 20; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Administrator { x = 350; y = 20; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Load Sample' { x = 320; y = 120; width = 30; height = 50; }
        metadata DiagramLayout::Route about 'Operator to Load Sample' { points = (120, 120, 320, 120); }
        metadata DiagramLayout::Route about 'Service Technician to Load Sample' { points = (320, 64, 320, 120); }
        metadata DiagramLayout::Route about 'Administrator to Load Sample' { points = (340, 64, 340, 120); }
    }

    // As crowdedCorner, with a fourth route from the right meeting the top-right
    // corner: the two symbols from above fit between neither corner's symbol.
    view crowdedCorners {
        expose Operator;
        expose Administrator;
        expose 'Service Technician';
        expose Supervisor;
        expose 'Load Sample';
        expose 'Operator to Load Sample';
        expose 'Service Technician to Load Sample';
        expose 'Administrator to Load Sample';
        expose 'Supervisor to Load Sample';
        render withParameters;

        metadata DiagramLayout::Layout about Operator { x = 40; y = 98; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Service Technician' { x = 280; y = 20; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Administrator { x = 350; y = 20; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Supervisor { x = 520; y = 98; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Load Sample' { x = 320; y = 120; width = 30; height = 50; }
        metadata DiagramLayout::Route about 'Operator to Load Sample' { points = (120, 120, 320, 120); }
        metadata DiagramLayout::Route about 'Service Technician to Load Sample' { points = (320, 64, 320, 120); }
        metadata DiagramLayout::Route about 'Administrator to Load Sample' { points = (340, 64, 340, 120); }
        metadata DiagramLayout::Route about 'Supervisor to Load Sample' { points = (520, 120, 350, 120); }
    }

    // The routes to the instrument's two declared ports end at one point of its left border.
    view ports {
        expose Rack;
        expose Instrument;
        expose 'Rack to Sensor';
        expose 'Rack to Heater';
        render Views::asInterconnectionDiagram;
        metadata DiagramLayout::Layout about Rack { x = 40; y = 60; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Instrument { x = 320; y = 120; width = 160; height = 100; }
        metadata DiagramLayout::Route about 'Rack to Sensor' { points = (120, 82, 320, 170); }
        metadata DiagramLayout::Route about 'Rack to Heater' { points = (120, 82, 320, 170); }
    }

    // The instrument has a position but no size, so its box is fitted round its ports.
    view unsized {
        expose Rack;
        expose Instrument;
        expose 'Rack to Sensor';
        expose 'Rack to Heater';
        render Views::asInterconnectionDiagram;
        metadata DiagramLayout::Layout about Rack { x = 40; y = 60; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Instrument { x = 320; y = 120; }
        metadata DiagramLayout::Route about 'Rack to Sensor' { points = (120, 82, 320, 170); }
        metadata DiagramLayout::Route about 'Rack to Heater' { points = (120, 82, 320, 170); }
    }

    // The instrument has no position at all: its box is fitted round its ports.
    view unpositioned {
        expose Rack;
        expose Instrument;
        expose 'Rack to Sensor';
        expose 'Rack to Heater';
        render Views::asInterconnectionDiagram;
        metadata DiagramLayout::Layout about Rack { x = 40; y = 60; width = 80; height = 44; }
        metadata DiagramLayout::Route about 'Rack to Sensor' { points = (120, 82, 320, 170); }
        metadata DiagramLayout::Route about 'Rack to Heater' { points = (120, 82, 320, 170); }
    }

    // The routes to the instrument's declared ports end at one point inside it.
    view interiorPorts {
        expose Rack;
        expose Instrument;
        expose 'Rack to Sensor';
        expose 'Rack to Heater';
        render Views::asInterconnectionDiagram;
        metadata DiagramLayout::Layout about Rack { x = 40; y = 60; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Instrument { x = 320; y = 120; width = 160; height = 100; }
        metadata DiagramLayout::Route about 'Rack to Sensor' { points = (120, 82, 370, 170); }
        metadata DiagramLayout::Route about 'Rack to Heater' { points = (120, 82, 370, 170); }
    }

    // The routes to the instrument's nested parts end at one point inside it.
    view interior {
        expose Rack;
        expose Instrument;
        expose 'Rack to Detector';
        expose 'Rack to Filament';
        render Views::asInterconnectionDiagram;
        metadata DiagramLayout::Layout about Rack { x = 40; y = 60; width = 80; height = 44; }
        metadata DiagramLayout::Layout about Instrument { x = 320; y = 120; width = 160; height = 100; }
        metadata DiagramLayout::Route about 'Rack to Detector' { points = (120, 82, 370, 170); }
        metadata DiagramLayout::Route about 'Rack to Filament' { points = (120, 82, 370, 170); }
    }

    // Only the administrator is connected; its route ends where the middle of the three
    // unconnected parameters' pins is placed on the top side.
    view freePin {
        expose Administrator;
        expose 'Load Sample';
        expose 'Administrator to Load Sample';
        render withParameters;
        metadata DiagramLayout::Layout about Administrator { x = 350; y = 20; width = 80; height = 44; }
        metadata DiagramLayout::Layout about 'Load Sample' { x = 320; y = 120; width = 160; height = 50; }
        metadata DiagramLayout::Route about 'Administrator to Load Sample' { points = (390, 64, 390, 120); }
    }

    metadata MigrationMetadata::SynthesizedName about 'Operator to Acquire Image';
}
