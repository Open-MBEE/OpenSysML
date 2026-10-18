package Door {
    attribute def Unlock;
    attribute def Lock;
    attribute def Open;
    attribute def Close;

    state def DoorStates {
        entry; then locked;
        state locked;
        state closed;
        state opened;
        transition locked_to_closed first locked accept Unlock then closed;
        transition closed_to_locked first closed accept Lock then locked;
        transition closed_to_opened first closed accept Open then opened;
        transition opened_to_closed first opened accept Close then closed;
    }
}
