package ThenAfterMembers {
    item def Signal;
    action def Step {
        in item x : Signal;
        out item y : Signal;
    }
    action def AfterFirstStart {
        first start;
        then action a : Step;
        then done;
    }
}
