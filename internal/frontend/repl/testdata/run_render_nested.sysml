package NestedRun {
    private import SI::*;

    part def Controller {
        exhibit state modes {
            entry; then waiting;
            state waiting;
            transition first waiting accept after 1 [s] then done;
        }
    }

    part def Mission {
        part controller : Controller;
    }

    part mission : Mission;
}
