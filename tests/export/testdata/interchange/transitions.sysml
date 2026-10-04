package TransitionCases {
    item def Go;
    action def Reset;
    attribute ready : ScalarValues::Boolean;

    state def Modes {
        state x;
        state y;

        first start then x;
        transition triggered first x accept Go then y;
        transition guarded first y accept Go if ready then x;
        transition effectful first x do action reset : Reset then y;
    }
}
