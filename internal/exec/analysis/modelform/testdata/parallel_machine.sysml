package Orthogonal {
    attribute def Go;
    state def Machine parallel {
        state motion {
            entry; then idle;
            state idle;
            state moving;
            transition first idle accept Go then moving;
        }
        state power;
    }
    part def Tug { exhibit state modes : Machine; }
}
