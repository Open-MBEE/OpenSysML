// Invalid: a control node must be owned by an action definition or action usage (SysML v2 8.3.17.6; pilot validateControlNodeOwningType).
package CN05ControlNodeOutsideAction {
    constraint def C {
        fork f;
    }
    part def Q {
        constraint c {
            decide d;
        }
    }
}
