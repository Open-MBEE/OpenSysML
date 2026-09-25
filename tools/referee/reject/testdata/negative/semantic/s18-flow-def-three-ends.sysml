// Invalid: A flow connection definition can have at most two ends (SysML 7.13 Connections; pilot validateFlowDefinitionConnectionEnds).
package S18FlowDefThreeEnds {
    part def P;
    flow def FD {
        end part a : P;
        end part b : P;
        end part c : P;
    }
}
