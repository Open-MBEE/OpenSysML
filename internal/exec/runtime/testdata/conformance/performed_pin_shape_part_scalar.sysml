// A part performing an action holds the action's pins as the action does: a [1] pin
// bound from the part's attribute is bare in the part's slots, as it is
// in the action's own results, however often the body read it.
package test {
    private import ScalarValues::*;

    action def Step {
        in value : Integer[1];
        out doubled : Integer[1];
        first start;
        then action d { assign doubled := value * 2; assign doubled := value + doubled; }
        then done;
    }

    part def Device {
        attribute level : Integer = 5;
        perform action step : Step { in value = level; }
    }

    part dev : Device;
}
