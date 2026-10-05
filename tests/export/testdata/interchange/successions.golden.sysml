package Successions {
    individual part def Timeline;
    individual part timeline : Timeline {
        timeslice occurrence firstEvent;
        succession first firstEvent then secondEvent;
        timeslice occurrence secondEvent;
    }
}
