package test {
    package inner {
        attribute value : Integer = 42;
    }
    
    // Test qualified name resolution
    calc testQualified {
        test::inner::value
    }
}
