package test {
    // Test transition effect actions
    state TestEffects {
        attribute counter : Integer = 0;
        
        entry; then init;
        state init;
        state active;
        
        succession first init then active;
        transition first active do {
            assign counter := counter + 1;
        } then done;
    }
}
