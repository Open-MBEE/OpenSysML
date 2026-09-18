package IfBranches {
    action def Drive {
        in attribute speed : Real;
        if speed > 0 {
            action accelerate;
            assign speed := speed + 1;
        } else {
            action brake;
        }
    }
}
