// A block definition diagram migrated from Cameo: boxes 33px high, two
// subtypes of Detector with the inheritance lines routed as the source drew
// them, one subtype with two generals, and the same subtype drawn in a second
// view with a route of its own. The unpositioned view is laid out by the engine.
package Microscope {
    part def Detector;
    part def Sensor;
    part def Instrument;
    part def 'Display System';
    part def 'Scanning Electron Microscope';
    part def 'Secondary Electron Detector' :> Detector;
    part def 'Backscattered Electron Detector' :> Detector, Sensor;
}

package MicroscopeViews {
    private import Views::*;
    private import DiagramLayout::*;
    private import Microscope::*;

    view positioned {
        expose Microscope::*;
        @Canvas { unit = "px"; width = 620; height = 240; }
        metadata Layout about Instrument { x = 20; y = 20; width = 110; height = 33; }
        metadata Layout about 'Display System' { x = 20; y = 90; width = 110; height = 33; }
        metadata Layout about 'Scanning Electron Microscope' { x = 20; y = 160; width = 110; height = 33; }
        metadata Layout about Detector { x = 260; y = 20; width = 110; height = 33; }
        metadata Layout about Sensor { x = 480; y = 20; width = 110; height = 33; }
        metadata Layout about 'Secondary Electron Detector' { x = 160; y = 160; width = 160; height = 33; }
        metadata Layout about 'Backscattered Electron Detector' { x = 400; y = 160; width = 160; height = 33; }
        metadata Route about 'Secondary Electron Detector' { points = (240, 160, 240, 110, 315, 110, 315, 53); ref :>> general : Detector; }
        metadata Route about 'Backscattered Electron Detector' { points = (440, 160, 440, 110, 315, 110, 315, 53); ref :>> general : Detector; }
        metadata Route about 'Backscattered Electron Detector' { points = (520, 160, 520, 120, 535, 120, 535, 53); ref :>> general : Sensor; }
        render asTreeDiagram;
    }

    view second {
        expose Detector;
        expose 'Backscattered Electron Detector';
        @Canvas { unit = "px"; width = 300; height = 200; }
        metadata Layout about Detector { x = 20; y = 20; width = 110; height = 33; }
        metadata Layout about 'Backscattered Electron Detector' { x = 120; y = 140; width = 160; height = 33; }
        metadata Route about 'Backscattered Electron Detector' { points = (200, 140, 200, 90, 75, 90, 75, 53); ref :>> general : Detector; }
        render asTreeDiagram;
    }

    view unpositioned {
        expose Microscope::*;
        render asTreeDiagram;
    }
}
