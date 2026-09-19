// An orthogonal machine completes only once every top-level region reaches
// `done`; here only one does, so the machine stays active.
package test {
    attribute def First;
    attribute def Second;

    state sm parallel {
        state left {
            entry; then lstart;
            state lstart;
            transition first lstart accept First then done;
        }
        state right {
            entry; then rstart;
            state rstart;
            state rbusy;
            transition first rstart accept Second then rbusy;
        }
    }
}
