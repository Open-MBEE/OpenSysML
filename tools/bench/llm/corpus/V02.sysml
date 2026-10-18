package Clock {
    part def Face;
    part def Hand;
    part def Movement;

    part def WallClock {
        part face : Face;
        part hourHand : Hand;
        part minuteHand : Hand;
        part movement : Movement;
    }

    comment clockNote about WallClock /* A battery-driven quartz wall clock. */
    comment handsNote about WallClock::hourHand, WallClock::minuteHand /* Both hands are driven by the same movement. */
}
