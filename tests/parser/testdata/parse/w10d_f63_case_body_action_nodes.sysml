// A case body carries an action body's items (SysML.xtext:2191 CaseBodyItem →
// CalculationBodyItem → ActionBodyItem): control nodes, loops and assignments.
package CaseBodyActionNodes {
    attribute def Power;
    analysis def Dynamics {
        in attribute powerProfile : Power[*];
        private attribute position;
        private attribute speed;
        for i in 1..2 {
            action step;
            then assign position := speed;
        }
    }
    use case def Drive {
        action start;
        action drive;
        join join1;
        fork fork1;
            then drive;
        first start then join1;
    }
    calc def Sum {
        attribute a;
        fork fork2;
            then a;
        return attribute r = a;
    }
    // A typed `for` variable (SysML.xtext ForLoopNode) in an action body.
    action def Loop {
        for n : ScalarValues::Integer in (1, 2, 3) {
            action body;
        }
    }
}
