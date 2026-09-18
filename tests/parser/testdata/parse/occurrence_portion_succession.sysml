package Portions {
    individual part def Craft;

    individual part craft : Craft {
        snapshot postSeparation;
        snapshot postFlyby;
        first postSeparation then postFlyby;

        event occurrence separation;
        event occurrence flyby;
        first separation then flyby;

        timeslice cruise;
        occurrence burns [2];
    }

    action def Sortie {
        first prep;
        action prep;
    }
}
