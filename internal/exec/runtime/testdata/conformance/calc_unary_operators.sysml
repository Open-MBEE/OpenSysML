package test {
    // Test unary operators: negation and logical not
    calc testNegation {
        in x : Integer;
        -x
    }
    
    calc testLogicalNot {
        in flag : Boolean;
        not flag
    }
}
