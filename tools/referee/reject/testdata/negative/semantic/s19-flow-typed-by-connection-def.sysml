// Invalid: A flow connection must be typed by flow connection definitions (SysML 7.13 Connections; pilot validateFlowUsageType).
package S19FlowTypedByConnectionDef {
    item def I;
    part def P {
        out item x : I;
        in item y : I;
    }
    connection def CD {
        end part a : P;
        end part b : P;
    }
    part s {
        part p : P;
        part q : P;
        flow f : CD from p.x to q.y;
    }
}
