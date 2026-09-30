// Invalid: A usage must be typed by definitions: a transition's bare accept names the payload type, and `start` here is the inherited Action::start usage (SysML 7.6 Definition and Usage; pilot validateUsageType).
state def S {
    state A;
    state B;
    transition first A accept start then B;
}
