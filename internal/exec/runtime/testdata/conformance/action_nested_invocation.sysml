package test {
    // Test nested action invocation
    action outer {
        attribute result : Integer = 0;
        
        first start;
        
        action inner {
            assign result := result + 5;
        }
        
        done;
        
        succession first start then inner;
        succession first inner then done;
    }
}
