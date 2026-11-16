package Machines {
    state def Lamp {
        entry; then off;
        state off;
        state on parallel {
            state heat {
                entry; then warm;
                state warm;
                state hot;
                transition warm then hot;
            }
            state light {
                entry; then dim;
                state dim;
                state bright;
                transition dim then bright;
                transition bright then heat.hot;
            }
        }
        transition off then on;
        transition on then on.light.bright;
        transition on.heat.hot then off;
    }
}
