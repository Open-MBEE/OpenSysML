// A part performing an action holds the action's pins as the action does: a bare pin
// bound from the part's attribute is a one-element sequence in the part's slots, as it is
// in the action's own results — not bare in one place and wrapped in the other.
package test {
    private import ScalarValues::*;

    action def Step {
        in value : Integer;
        out doubled : Integer;
        first start;
        then action d { assign doubled := value * 2; }
        then done;
    }

    part def Device {
        attribute level : Integer = 5;
        perform action step : Step { in value = level; }
    }

    part dev : Device;
}
