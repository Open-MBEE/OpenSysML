package test {
    // Test state do behavior
    state TestDo {
        attribute result : Integer = 0;
        
        entry; then init;
        state init;
        state processing {
            do {
                assign result := result + 10;
            }
        }
        
        succession first init then processing;
        succession first processing then done;
    }
}
