// A block definition diagram migrated from a tool that gave a Layout only to
// the symbols it drew: the definitions' boxes and one nested part drawn as a
// box of its own. The members with no Layout list in compartments of their
// owner's box, grouped by kind.
package Amplifier {
    private import ScalarValues::Real;
    private import ISQ::*;

    attribute def dB :> Real;
    port def SignalPort;
    part def Housing;
    part def Heatsink;
    constraint def GainBound;

    enum def Band { low; high; }

    // An unnamed member, a name written over two lines and a value with a
    // string argument.
    part def Label {
        attribute : Real;
        attribute 'upper\rlower' : Real;
        attribute length : ScalarValues::Natural = StringFunctions::Length("a  b");
    }

    part def Stage {
        attribute gain : dB = 15.0;
        attribute noiseFigure : Real = RealFunctions::sqrt(4.0);
        ref part heatsink : Heatsink { attribute fins : Real = 2 * 6; }
        part housing : Housing;
        part shield : Housing;
        port input : SignalPort;
        port output : SignalPort;
        constraint bound : GainBound;
        action calibrate;
        item def Slot;
    }
}

package AmplifierViews {
    private import ScalarValues::Boolean;
    private import Views::*;
    private import DiagramLayout::*;

    // Positioned: the Stage box lists its layout-less members; `shield` has a
    // box inside the Stage box, `housing` one outside, and the band lists its
    // literals.
    view positioned {
        expose Amplifier::Stage;
        expose Amplifier::Housing;
        expose Amplifier::Band;
        metadata Layout about Amplifier::Stage { x = 10; y = 10; width = 240; height = 260; }
        metadata Layout about Amplifier::Stage::shield { x = 40; y = 220; width = 100; height = 30; }
        metadata Layout about Amplifier::Stage::housing { x = 300; y = 400; width = 120; height = 40; }
        metadata Layout about Amplifier::Housing { x = 300; y = 10; width = 120; height = 40; }
        metadata Layout about Amplifier::Band { x = 500; y = 10; width = 100; height = 100; }
        render asTreeDiagram;
    }

    // A Layout with a corner and no size: the box is sized to its rows.
    view cornered {
        expose Amplifier::Stage;
        metadata Layout about Amplifier::Stage { x = 10; y = 10; }
        render asTreeDiagram;
    }

    // A corner-only Layout round a two-line row and a string argument.
    view wrapped {
        expose Amplifier::Label;
        metadata Layout about Amplifier::Label { x = 10; y = 10; }
        render asTreeDiagram;
    }

    // The same members under a box too small for them all.
    view cramped {
        expose Amplifier::Stage;
        metadata Layout about Amplifier::Stage { x = 10; y = 10; width = 160; height = 90; }
        render asTreeDiagram;
    }

    // Unpositioned: every member is a node of its own.
    view plain {
        expose Amplifier::Stage;
        expose Amplifier::Housing;
        expose Amplifier::Band;
        render asTreeDiagram;
    }

    // The tree rendering without compartments.
    rendering bareTree :> Views::asTreeDiagram {
        attribute showCompartments : Boolean default = false;
    }

    // Compartments turned off: the layout-less members are left undrawn.
    view bare {
        expose Amplifier::Stage;
        metadata Layout about Amplifier::Stage { x = 10; y = 10; width = 240; height = 260; }
        render bareTree;
    }
}
