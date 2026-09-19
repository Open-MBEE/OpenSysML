// Completion is reached through a pseudostate as through any other path.
package test {
    attribute def Stop;

    state sm {
        entry; then start;
        state start;
        state working;
        junction meet;
        succession first start then working;
        transition first working accept Stop then meet;
        transition first meet then done;
    }
}
