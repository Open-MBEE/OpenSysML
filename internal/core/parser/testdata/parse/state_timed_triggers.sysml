state TimedTriggers {
    entry; then start;
    state start;
    state waiting;
    accept after 10 then working;
    state working;
    accept at 15 then done;
    accept when working then done;

    succession first start then waiting;
}
