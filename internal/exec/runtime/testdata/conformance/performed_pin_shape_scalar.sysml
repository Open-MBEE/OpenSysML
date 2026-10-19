// A pin written [1] holds its one value bare on every surface, however often the body
// reads it: `in value : Integer[1]` bound to 5 reads 5, is 5 in the action's results and
// in the slot the performing part holds, and the pin written twice stays bare.
package test {
    private import ScalarValues::*;

    action def Step {
        in value : Integer[1];
        out doubled : Integer[1];
        out again : Integer[1];
        first start;
        then action d { assign doubled := value * 2; }
        then action a { assign again := value + doubled; }
        then done;
    }

    part def Device {
        attribute level : Integer = 5;
        perform action step : Step { in value = level; }
    }

    part dev : Device;

    action probe {
        out doubled : Integer[1];
        out again : Integer[1];
        first start;
        then action s : Step { in value = 5; }
        then action read { assign doubled := s.doubled; assign again := s.again; }
        then done;
    }
}
