package OccurrenceParameters {
    occurrence def Flight;
    part def Vehicle;
    individual part def Vehicle1 specializes Vehicle;
    action def Observe {
        in snapshot;
        in snapshot atStart : Flight;
        out timeslice;
        in individual occurrence : Vehicle1;
        in individual occurrence v : Vehicle1;
        in individual snapshot occurrence : Vehicle1;
        in individual snapshot occurrence vs : Vehicle1;
        in ref timeslice;
        in x;
    }
    occurrence def Q {
        timeslice;
        individual : Vehicle1;
        individual snapshot ms : Vehicle1;
        individual snapshot : Vehicle1;
        ref snapshot rs : Flight;
        ref individual timeslice : Vehicle1;
    }
}
