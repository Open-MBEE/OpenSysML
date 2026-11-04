// The compartments whose heading the graphical notation (SysML v2 §8.2.3)
// does not spell as the plain plural of the members' keyword: enums, calcs,
// rendering, subject, objective, satisfies, verifies, and ends for a
// connection's end features of any kind; flows and metadata take the plural.
package Headings {
    private import ScalarValues::Real;
    private import Views::*;

    enum def Mode { idle; active; }
    port def Plug;
    item def Signal;
    requirement def Safe;
    rendering def Gauge;
    metadata def Rationale { attribute text : ScalarValues::String; }
    part def Sensor { port a : Plug; port b : Plug; }

    part def Controller {
        attribute gain : Real;
        rendering gauge : Gauge;
        calc total { gain }
        part s1 : Sensor;
        part s2 : Sensor;
        flow of Signal from s1.a to s2.b;
        satisfy requirement safe : Safe by s1;
        @Rationale { text = "keeps the sensors apart"; }
    }

    connection def Link {
        end part source : Sensor;
        end part target : Sensor;
    }

    verification case def Check {
        subject controller : Controller;
        objective { verify requirement safe : Safe; }
        actor tester;
    }

    // A view's own render members describe its picture and are not shown.
    view def Dashboard {
        render asTreeDiagram;
    }
}

package HeadingViews {
    private import Views::*;
    private import DiagramLayout::*;

    // Positioned: every exposed definition has a box of its own and lists
    // its layout-less members in compartments.
    view headings {
        expose Headings::Mode;
        expose Headings::Controller;
        expose Headings::Link;
        expose Headings::Check;
        expose Headings::Dashboard;
        metadata Layout about Headings::Mode { x = 10; y = 10; }
        metadata Layout about Headings::Controller { x = 200; y = 10; }
        metadata Layout about Headings::Link { x = 500; y = 10; }
        metadata Layout about Headings::Check { x = 10; y = 300; }
        metadata Layout about Headings::Dashboard { x = 300; y = 300; }
        render asTreeDiagram;
    }
}
