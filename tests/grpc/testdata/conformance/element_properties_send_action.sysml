// The SendActionUsage metaclass family read by specification name: a send
// action's payload and receiver arguments inside an action definition.
package SendFamily {
    private import ScalarValues::*;
    attribute def Go;
    part def Robot {
        action controller : Control;
    }
    action def Control {
        action ping send new Go() to robot;
        ref part robot : Robot;
    }
}
