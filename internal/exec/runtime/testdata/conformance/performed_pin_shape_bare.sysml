// A pin writing no multiplicity is [0..*] (SysML v2 §7.6.3), so it holds its one value as
// a one-element sequence on every surface — the action's results, the trace, the slot the
// performing part holds — not bare on one and wrapped on another, and the body reads the
// same shape before and after another read.
package test {
    private import ScalarValues::*;

    action def Step {
        in value : Integer;
        out doubled : Integer;
        out again : Integer;
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
        out doubled : Integer;
        out again : Integer;
        out one : Integer[1];
        first start;
        then action s : Step { in value = 5; }
        then action read { assign doubled := s.doubled; assign again := s.again; assign one := s.again; }
        then done;
    }
}
