package CallTriggerRegionTest {
    // One invocation is broadcast to every orthogonal region: the regions that
    // have a matching call trigger each fire one transition, run-to-completion.
    state def Dashboard parallel {
        state display {
            entry; then start;
            state start;
            state Dimmed;
            state Lit;

            succession first start then Dimmed;
            transition first Dimmed accept wake() then Lit;
        }

        state audio {
            entry; then start;
            state start;
            state Muted;
            state Chiming;

            succession first start then Muted;
            transition first Muted accept wake() then Chiming;
            transition first Chiming accept mute() then Muted;
        }
    }
}
