// A block definition diagram migrated from a tool that let the modeler
// collapse compartments symbol by symbol: each Layout keeps the box the tool
// sized for the compartments left visible and turns the others off with its
// show… switches. A hidden compartment is neither listed nor counted as
// clipped; one left on keeps the box's fit and clipping rules.
package Camera {
    private import ScalarValues::Real;

    attribute def Pixels :> Real;
    port def DataPort;
    part def Lens;
    part def Sensor;
    part def Mount;
    constraint def Focus;
    item def Frame;

    part def Imaging {
        attribute resolution : Pixels = 4096;
        attribute exposure : Real;
        part lens : Lens;
        part sensor : Sensor;
        ref part mount : Mount;
        port data : DataPort;
        constraint focus : Focus;
        action capture;
        item frame : Frame;
    }
}

package CameraViews {
    private import Views::*;
    private import DiagramLayout::*;

    // The box lists attributes, ports, constraints and the ref part; the
    // parts, items and actions the modeler collapsed are not listed, and the
    // box, sized for the rest, clips nothing.
    view selective {
        expose Camera::Imaging;
        metadata Layout about Camera::Imaging {
            x = 10; y = 10; width = 240; height = 220;
            showParts = false; showItems = false; showActions = false;
        }
        render asTreeDiagram;
    }

    // Every compartment collapsed: the 200×33 bar lists nothing and reports
    // nothing unwritten.
    view collapsed {
        expose Camera::Imaging;
        metadata Layout about Camera::Imaging {
            x = 10; y = 10; width = 200; height = 33;
            showAttributes = false; showParts = false; showReferences = false; showPorts = false;
            showConstraints = false; showItems = false; showActions = false;
        }
        render asTreeDiagram;
    }

    // Parts collapsed in a bar sized for nothing more: the compartments left
    // on do not fit and are reported, the parts are not.
    view cramped {
        expose Camera::Imaging;
        metadata Layout about Camera::Imaging {
            x = 10; y = 10; width = 200; height = 33;
            showParts = false;
        }
        render asTreeDiagram;
    }

    // Parts collapsed in a box with room for every row: the parts are left
    // out of a box that would have listed them, nothing is clipped.
    view roomy {
        expose Camera::Imaging;
        metadata Layout about Camera::Imaging { x = 10; y = 10; width = 300; height = 420; showParts = false; }
        render asTreeDiagram;
    }

    // The tree rendering without compartments.
    rendering bareTree :> Views::asTreeDiagram {
        attribute showCompartments : ScalarValues::Boolean default = false;
    }

    // The rendering's switch wins: nothing is listed though only parts are off.
    view bare {
        expose Camera::Imaging;
        metadata Layout about Camera::Imaging { x = 10; y = 10; width = 240; height = 260; showParts = false; }
        render bareTree;
    }
}
