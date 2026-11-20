package OccurrenceParameters {
    occurrence def Flight;
    part def Vehicle;
    individual part def Vehicle1 :> Vehicle;
    action def Observe {
        in snapshot ;
        in snapshot atStart : Flight;
        out timeslice ;
        in individual : Vehicle1;
        in individual v : Vehicle1;
        in individual snapshot : Vehicle1;
        in individual snapshot vs : Vehicle1;
        in ref timeslice ;
        in x;
    }
    occurrence def Q {
        timeslice ;
        individual : Vehicle1;
        individual snapshot ms : Vehicle1;
        individual snapshot : Vehicle1;
        ref snapshot rs : Flight;
        ref individual timeslice : Vehicle1;
    }
}
